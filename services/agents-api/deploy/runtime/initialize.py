"""Trusted hosted initialization. Invoke with /usr/bin/python3 -I -S.

Core owns sequencing and the completion ledger. This helper executes one bounded
operation; it never retries, schedules, selects a harness or interprets templates.
"""
import ctypes
import json
import os
from pathlib import Path
import re
import runpy
import select
import signal
import shlex
import subprocess
import sys

ROOT = Path('/environment')
CONFIG = ROOT / 'initialization'
PACKAGES = ROOT / 'packages'
ENV_FILE = CONFIG / 'tool-env.sh'
MAX_INPUT = 32 * 1024 * 1024
BASE_ENV = {'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': '/tmp', 'LANG': 'C.UTF-8'}
DIRECTORY = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC


def open_directory(parent, name):
    """Do not follow user-created directory aliases outside the Runtime roots."""
    return os.open(name, DIRECTORY, dir_fd=parent)


def roots(workspace=None):
    root = os.open('/', DIRECTORY)
    try:
        environment = open_directory(root, 'environment')
    finally:
        os.close(root)
    try:
        for name in (('workspace', 'packages', 'initialization') if workspace is None else ('packages', 'initialization')):
            child = open_directory(environment, name)
            os.close(child)
    finally:
        os.close(environment)
    if workspace is not None:
        os.close(os.open(workspace, DIRECTORY))


def configure(env):
    if not isinstance(env, dict) or any(
        not isinstance(name, str) or not re.fullmatch('[A-Za-z_][A-Za-z0-9_]*', name)
        or not isinstance(value, str) or '\x00' in value
        for name, value in env.items()
    ):
        raise ValueError('invalid environment')
    # These Runtime-owned defaults are applied inside the sandbox, never to its
    # launcher. Public reserved-name validation belongs to Core.
    values = {
        'PATH': '/environment/packages/npm/bin:/environment/packages/python/bin:' + BASE_ENV['PATH'],
        **env,
        'PYTHONPATH': '/environment/packages/python' + (':' + env['PYTHONPATH'] if env.get('PYTHONPATH') else ''),
    }
    materialized = json.dumps(values, ensure_ascii=True)
    script = ''.join('export ' + name + '=' + shlex.quote(value) + '\n' for name, value in sorted(values.items()))
    directory = os.open(CONFIG, DIRECTORY)
    try:
        fd = os.open('tool-env.sh', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o400, dir_fd=directory)
        with os.fdopen(fd, 'w') as stream:
            stream.write(script)
            stream.flush()
            os.fsync(stream.fileno())
        fd = os.open('tool-env.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o400, dir_fd=directory)
        with os.fdopen(fd, 'w') as stream:
            stream.write(materialized)
            stream.flush()
            os.fsync(stream.fileno())
        os.fsync(directory)
    finally:
        os.close(directory)


def sandbox(network, cwd, workspace=None):
    if network not in ('enabled', 'disabled') or not isinstance(cwd, str) or not cwd.startswith('/') or '\x00' in cwd:
        raise ValueError('invalid execution configuration')
    if (CONFIG / 'system-root.json').exists():
        tools = runpy.run_path('/usr/local/bin/oac-tool-root')
        if not tools['installed']():
            raise ValueError('system tools unavailable')
        return tools['initialization_sandbox'](cwd, network, workspace=workspace or str(ROOT / 'workspace'))
    # One packaging contract for every Provider/harness. No native state, daemon
    # credential, staging payload or parent process is visible in this mount map.
    args = ['/usr/bin/bwrap', '--unshare-user', '--unshare-pid', '--unshare-ipc', '--unshare-uts',
            '--die-with-parent', '--new-session', '--cap-drop', 'ALL', '--clearenv',
            '--ro-bind', '/usr', '/usr', '--symlink', 'usr/bin', '/bin',
            '--symlink', 'usr/sbin', '/sbin', '--symlink', 'usr/lib', '/lib',
            '--symlink', 'usr/lib64', '/lib64', '--dir', '/etc',
            '--ro-bind', '/etc/resolv.conf', '/etc/resolv.conf',
            '--ro-bind', '/etc/hosts', '/etc/hosts', '--ro-bind', '/etc/ssl', '/etc/ssl',
            '--proc', '/proc', '--dev', '/dev', '--tmpfs', '/tmp', '--dir', '/home',
            '--dir', '/environment', '--bind', workspace or str(ROOT / 'workspace'), '/workspace',
            '--bind', str(PACKAGES), str(PACKAGES), '--ro-bind', str(CONFIG), str(CONFIG)]
    if workspace is not None and workspace != '/workspace':
        args += ['--bind', workspace, workspace]
    if network == 'disabled':
        args += ['--unshare-net']
    for key, value in BASE_ENV.items():
        args += ['--setenv', key, value]
    return args + ['--chdir', cwd, '--']


def run(request, workspace=None):
    action = request['action']
    if action == 'configure':
        configure(request.get('env', {}))
        return
    if action == 'system':
        runpy.run_path('/usr/local/bin/oac-tool-root')['install'](request['packages'], workspace=workspace or str(ROOT / 'workspace'))
        return
    args = sandbox(request['network'], request.get('cwd', '/workspace'), workspace=workspace)
    if action == 'setup':
        command = request['command']
        if not isinstance(command, str) or not command or '\x00' in command:
            raise ValueError('invalid setup command')
        # Source confidential values only inside isolation. eval preserves the
        # shell's cwd, unlike replacing the native shell with a child wrapper.
        args += ['/bin/bash', '--noprofile', '--norc', '-c',
                 '. /environment/initialization/tool-env.sh && eval -- "$1"', '--', command]
    elif action in ('npm', 'python'):
        packages = request['packages']
        if not isinstance(packages, list) or not packages or any(
            not isinstance(package, str) or not package or '\x00' in package or package.startswith('-')
            for package in packages
        ):
            raise ValueError('invalid packages')
        command = ['npm', 'install', '--global', '--prefix', str(PACKAGES / 'npm'), '--', *packages]
        if action == 'python':
            command = ['/usr/bin/python3', '-m', 'pip', 'install', '--disable-pip-version-check',
                       '--no-input', '--target', str(PACKAGES / 'python'), '--', *packages]
        args += ['/bin/bash', '--noprofile', '--norc', '-c',
                 '. /environment/initialization/tool-env.sh && exec "$@"', '--', *command]
    else:
        raise ValueError('invalid action')
    # Setup/package output can contain arbitrary confidential values. The public
    # completion receipt deliberately contains no child stdout/stderr or command.
    subprocess.run(args, env=BASE_ENV, stdin=subprocess.DEVNULL,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)


def failed_receipt(error):
    """Report only the integer exit status of a step run inside the isolation sandbox.

    Setup commands and package managers (including apt from the system tool root)
    run through bwrap. Runtime-internal helpers such as seed extraction and the
    Skill writer do not, so their failures stay generic. The exception itself is
    never serialized: it can contain the command, input or captured output.
    """
    receipt = {'version': 1, 'outcome': 'failed'}
    code = getattr(error, 'returncode', None)
    if (isinstance(error, subprocess.CalledProcessError) and isinstance(error.cmd, list)
            and error.cmd[:1] == ['/usr/bin/bwrap'] and type(code) is int and 0 < code < 256):
        receipt['exit_code'] = code
    return json.dumps(receipt, separators=(',', ':'))


def stdio_lifetime(args):
    """Bind sandbox lifetime to the native process, not its transient spawn thread."""
    parent = os.getppid()
    if parent <= 1:
        raise ValueError('native parent unavailable')
    parent_fd = os.pidfd_open(parent)
    child_fd = None
    child = None
    try:
        watched = select.poll()
        watched.register(parent_fd, select.POLLIN)
        if os.getppid() != parent or watched.poll(0):
            raise ValueError('native parent exited')
        # Keep exited children waitable until their pidfd has been acquired.
        signal.signal(signal.SIGCHLD, signal.SIG_DFL)
        # Bind before fork. This entry is single-threaded; the child only sets
        # its death signal and checks the captured parent before exec. bwrap sets
        # its own signal later, leaving a demonstrated startup gap without this.
        owner = os.getpid()
        prctl = ctypes.CDLL(None).prctl
        prctl.argtypes = [ctypes.c_int] + [ctypes.c_ulong] * 4
        prctl.restype = ctypes.c_int

        def bind_parent():
            if prctl(1, signal.SIGKILL, 0, 0, 0) != 0 or os.getppid() != owner:
                os._exit(1)

        # No protocol forwarding. bwrap's parent-death signal now targets this
        # stable launcher; its PID namespace owns descendants.
        child = subprocess.Popen(args, env=BASE_ENV, close_fds=True, preexec_fn=bind_parent)
        child_fd = os.pidfd_open(child.pid)
        watched.register(child_fd, select.POLLIN)
        events = dict(watched.poll())
        if parent_fd in events:
            child.kill()
            child.wait()
            return 1
        result = child.wait()
        return result if result >= 0 else 128 - result
    finally:
        if child is not None:
            if child.poll() is None:
                child.kill()
            child.wait()
        if child_fd is not None:
            os.close(child_fd)
        os.close(parent_fd)


def stdio(installation_root, workspace, package, server):
    """Preserve the native MCP descriptors while entering the existing sandbox."""
    for value in (installation_root, workspace):
        if (not isinstance(value, str) or not value.startswith('/') or value == '/'
                or os.path.normpath(value) != value or any(ord(c) < 32 or ord(c) == 127 or c == '\\' for c in value)):
            raise ValueError('invalid Runtime directory')
        path = Path(value)
        if path.resolve(strict=True) != path or not path.is_dir():
            raise ValueError('invalid Runtime directory')
    roots(workspace)
    args = sandbox('enabled', '/workspace', workspace=workspace)
    helper = '/tmp/oac-mcp-exec'
    # System-package roots predate daemon installation. Mount only the fixed
    # static helper, never native configuration, credentials or Runtime state.
    args[-1:-1] = ['--ro-bind', '/usr/local/bin/oac-daemon', helper, '--ro-bind', installation_root, installation_root]
    args += [helper, 'runtime-mcp-exec', installation_root, package, server]
    return stdio_lifetime(args)


def main():
    workspace = None
    if len(sys.argv) == 3 and sys.argv[1] == 'initialize':
        workspace = sys.argv[2]
    elif len(sys.argv) != 1:
        try:
            if len(sys.argv) != 6 or sys.argv[1] != 'stdio':
                raise ValueError('invalid stdio invocation')
            return stdio(sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5])
        except Exception:
            print('Environment MCP unavailable', file=sys.stderr)
            return 1
    try:
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
                    or path.resolve(strict=True) != path or not path.is_dir()):
                raise ValueError('invalid Runtime workspace')
        if workspace is None:
            roots()
            run(request)
        else:
            roots(workspace)
            run(request, workspace=workspace)
    except Exception as error:
        print(failed_receipt(error))
        return 1
    print('{"version":1,"outcome":"completed"}')
    return 0


if __name__ == '__main__':
    sys.exit(main())
