"""Trusted hosted initialization. Invoke with /usr/bin/python3 -I -S.

Core owns sequencing and the completion ledger. This helper executes one bounded
operation; it never retries, schedules, selects a harness or interprets templates.
"""
import json
import os
from pathlib import Path
import re
import runpy
import shlex
import subprocess
import sys

ROOT = Path('/environment')
CONFIG = ROOT / 'initialization'
PACKAGES = ROOT / 'packages'
ENV_FILE = CONFIG / 'tool-env.sh'
MAX_INPUT = 32 * 1024 * 1024
BASE_ENV = {'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': '/tmp', 'LANG': 'C.UTF-8'}
DIRECTORY = os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC


def roots(workspace=None):
    for directory in (Path(workspace) if workspace else ROOT / 'workspace', PACKAGES, CONFIG):
        if not directory.is_dir():
            raise ValueError('invalid Runtime directory')


def configure(env):
    if not isinstance(env, dict) or any(
        not isinstance(name, str) or not re.fullmatch('[A-Za-z_][A-Za-z0-9_]*', name)
        or not isinstance(value, str) or '\x00' in value
        for name, value in env.items()
    ):
        raise ValueError('invalid environment')
    # Public reserved-name validation belongs to Core. Only the configured
    # environment and these defaults are passed to initialization commands.
    values = {
        'PATH': str(PACKAGES / 'npm/bin') + ':' + str(PACKAGES / 'python/bin') + ':' + BASE_ENV['PATH'],
        **env,
        'PYTHONPATH': str(PACKAGES / 'python') + (':' + env['PYTHONPATH'] if env.get('PYTHONPATH') else ''),
    }
    materialized = json.dumps(values, ensure_ascii=True)
    script = ''.join('export ' + name + '=' + shlex.quote(value) + '\n' for name, value in sorted(values.items()))
    directory = os.open(CONFIG, DIRECTORY)
    try:
        fd = os.open('tool-env.sh', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o400, dir_fd=directory)
        with os.fdopen(fd, 'w') as stream:
            stream.write(script)
            stream.flush()
            os.fsync(stream.fileno())
        fd = os.open('tool-env.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o400, dir_fd=directory)
        with os.fdopen(fd, 'w') as stream:
            stream.write(materialized)
            stream.flush()
            os.fsync(stream.fileno())
        os.fsync(directory)
    finally:
        os.close(directory)


class StepFailure(Exception):
    """A launched initialization command failed; never retain its text or output."""

    def __init__(self, exit_code):
        self.exit_code = exit_code
        super().__init__('initialization command failed')


def execution_directory(cwd, workspace):
    if (not isinstance(cwd, str) or os.path.normpath(cwd) != cwd
            or any(ord(c) < 32 or ord(c) == 127 or c == '\\' for c in cwd)
            or not (cwd == '/workspace' or cwd.startswith('/workspace/'))):
        raise ValueError('invalid execution directory')
    return str(Path(workspace or ROOT / 'workspace') / cwd.removeprefix('/workspace').lstrip('/'))


def run(request, workspace=None, capability_root=None):
    action = request['action']
    if action == 'configure':
        configure(request.get('env', {}))
        return
    if action == 'system':
        runpy.run_path('/usr/local/bin/oac-tool-root')['install'](request['packages'], workspace=workspace or str(ROOT / 'workspace'))
        return
    if action not in ('setup', 'npm', 'python'):
        raise ValueError('invalid action')
    if request['network'] not in ('enabled', 'disabled'):
        raise ValueError('invalid execution configuration')
    cwd = execution_directory(request.get('cwd', '/workspace'), workspace)
    if action == 'setup':
        command = request['command']
        if not isinstance(command, str) or not command or '\x00' in command:
            raise ValueError('invalid setup command')
        args = ['/bin/bash', '--noprofile', '--norc', '-c', command]
    else:
        packages = request['packages']
        if not isinstance(packages, list) or not packages or any(
            not isinstance(package, str) or not package or '\x00' in package or package.startswith('-')
            for package in packages
        ):
            raise ValueError('invalid packages')
        args = ['npm', 'install', '--global', '--prefix', str(PACKAGES / 'npm'), '--', *packages]
        if action == 'python':
            args = ['/usr/bin/python3', '-m', 'pip', 'install', '--disable-pip-version-check',
                    '--no-input', '--target', str(PACKAGES / 'python'), '--', *packages]
    env = {**BASE_ENV, **json.loads((CONFIG / 'tool-env.json').read_text())}
    # Execution uses the launching user's permissions. The outer Environment owns
    # isolation; command output is still excluded from the completion receipt.
    try:
        subprocess.run(args, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
    except subprocess.CalledProcessError as error:
        raise StepFailure(error.returncode) from None


def failed_receipt(error):
    """Expose a command's exit status, never its command, input or output."""
    receipt = {'version': 1, 'outcome': 'failed'}
    code = getattr(error, 'exit_code', None)
    # The system installer retains its existing implementation until its package
    # ownership is migrated; other internal helper failures remain generic.
    if (isinstance(error, subprocess.CalledProcessError) and isinstance(error.cmd, list)
            and error.cmd[:1] == ['/usr/bin/bwrap']):
        code = error.returncode
    if type(code) is int and 0 < code < 256:
        receipt['exit_code'] = code
    return json.dumps(receipt, separators=(',', ':'))


def main():
    workspace, capability_root = None, None
    if len(sys.argv) == 4 and sys.argv[1] == 'initialize':
        workspace, capability_root = sys.argv[2:]
    try:
        if len(sys.argv) != 1 and not (len(sys.argv) == 4 and sys.argv[1] == 'initialize'):
            raise ValueError('invalid initialization invocation')
        raw = sys.stdin.buffer.read(MAX_INPUT + 1)
        if len(raw) > MAX_INPUT:
            raise ValueError('input too large')
        request = json.loads(raw)
        if not isinstance(request, dict) or request.get('version') != 1:
            raise ValueError('invalid version')
        if workspace is not None:
            path = Path(workspace)
            if (not workspace.startswith('/') or workspace == '/' or os.path.normpath(workspace) != workspace
                    or any(ord(c) < 32 or ord(c) == 127 or c == '\\' for c in workspace)
                    or not path.is_dir()):
                raise ValueError('invalid Runtime workspace')
        if workspace is None:
            roots()
            run(request)
        else:
            roots(workspace)
            run(request, workspace=workspace, capability_root=capability_root)
    except Exception as error:
        print(failed_receipt(error))
        return 1
    print('{"version":1,"outcome":"completed"}')
    return 0


if __name__ == '__main__':
    sys.exit(main())
