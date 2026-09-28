#!/usr/bin/env python3
"""Execute a maintainer-pinned private qualification package over authenticated SSH.

The package is independent of candidate assets. Its manifest fixes all helper
bytes, structured Python commands, timeouts and private path/resource configuration.
Only fresh child exit status and identity-bound stdout establish stage completion.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path, PurePosixPath
import re
import signal
import subprocess
import sys
import uuid

if 'qualification_control' in sys.modules:
    control = sys.modules['qualification_control']
else:
    _control_spec = importlib.util.spec_from_file_location(
        'qualification_control', Path(__file__).with_name('qualification_control.py'))
    control = importlib.util.module_from_spec(_control_spec)
    _control_spec.loader.exec_module(control)

CHECKS = ('fresh-install', 'current-lifecycle', 'managed-native',
          'current-generations', 'node-runtime', 'diagnostics-observations')
IDENTITY = ('source', 'tree', 'run_id', 'inventory_sha256', 'adapter_sha256')


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def file_identity(path):
    if path.is_symlink() or not path.is_file():
        raise ValueError('Expected a regular package file')
    checksum = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            checksum.update(chunk)
    return {'sha256': checksum.hexdigest(), 'size': path.stat().st_size}


def relative_path(name):
    if (not isinstance(name, str) or not re.fullmatch(r'[A-Za-z0-9_./-]+', name)
            or PurePosixPath(name).is_absolute() or any(p in ('', '.', '..') for p in name.split('/'))):
        raise ValueError('Invalid package path')
    return name


def verify_package(root, expected_hash):
    if root.is_symlink() or not root.is_dir() or not re.fullmatch('[0-9a-f]{64}', expected_hash):
        raise ValueError('Explicit qualification package and manifest SHA256 required')
    manifest_path = root / 'manifest.json'
    if file_identity(manifest_path)['sha256'] != expected_hash:
        raise ValueError('Qualification manifest hash mismatch')
    manifest = json.loads(manifest_path.read_bytes())
    if (set(manifest) != {'version', 'files', 'stages', 'configuration'} or manifest['version'] != 1
            or not isinstance(manifest['configuration'], dict) or not isinstance(manifest['files'], dict)):
        raise ValueError('Invalid qualification package manifest')
    expected = manifest['files']
    for name, identity in expected.items():
        relative_path(name)
        if name == 'manifest.json' or not isinstance(identity, dict) or set(identity) != {'sha256', 'size'}:
            raise ValueError('Invalid package file identity')
        if (not re.fullmatch('[0-9a-f]{64}', identity.get('sha256', ''))
                or type(identity.get('size')) is not int or identity['size'] < 0):
            raise ValueError('Invalid package file digest or size')
    actual = {}
    for path in root.rglob('*'):
        if path.is_symlink():
            raise ValueError('Package symlinks are forbidden')
        if path.is_dir():
            continue
        name = path.relative_to(root).as_posix()
        if name != 'manifest.json':
            actual[name] = file_identity(path)
    if actual != expected:
        raise ValueError('Qualification package bytes/set mismatch')
    stages = manifest['stages']
    if not isinstance(stages, list) or [stage.get('name') for stage in stages] != list(CHECKS):
        raise ValueError('Exact ordered qualification stages required')
    for stage in stages:
        if set(stage) != {'name', 'python', 'script', 'args', 'timeout_seconds'}:
            raise ValueError('Invalid stage command fields')
        if (not isinstance(stage['python'], str) or not re.fullmatch(r'/[A-Za-z0-9_./-]+', stage['python'])
                or '..' in PurePosixPath(stage['python']).parts
                or relative_path(stage['script']) not in expected or not stage['script'].endswith('.py')
                or not isinstance(stage['args'], list)
                or any(not isinstance(arg, str) or '\x00' in arg for arg in stage['args'])
                or type(stage['timeout_seconds']) is not int or not 1 <= stage['timeout_seconds'] <= 14400):
            raise ValueError('Invalid bounded structured Python command')
    return manifest


def verify_assets(request):
    if (any(not re.fullmatch('[0-9a-f]{40}', request.get(key, '')) for key in ('source', 'tree'))
            or str(uuid.UUID(request.get('run_id', ''))) != request.get('run_id')
            or request.get('required_checks') != list(CHECKS)):
        raise ValueError('Invalid qualification identity')
    for key in ('inventory_sha256', 'adapter_sha256'):
        if not re.fullmatch('[0-9a-f]{64}', request.get(key, '')):
            raise ValueError('Missing qualification identity digest')
    if hashlib.sha256(canonical(request['inventory'])).hexdigest() != request['inventory_sha256']:
        raise ValueError('Inventory digest mismatch')
    root = Path(request['directory'])
    if not root.is_absolute() or root.resolve() != root:
        raise ValueError('Invalid candidate directory')
    for name, expected in request['inventory'].items():
        if relative_path(name) != Path(name).name or file_identity(root / name) != expected:
            raise ValueError('Candidate bytes changed')


def save_new(path, value):
    with path.open('x', encoding='utf8') as stream:
        json.dump(value, stream, sort_keys=True)
        stream.flush()
        os.fsync(stream.fileno())


def execute_stage(stage, package, request, evidence, script_hash):
    # Execute the verified script bytes; helper imports are checked as a full set
    # before and after each stage. The authorized host remains the trust boundary.
    script = package / stage['script']
    code = ("import hashlib,pathlib,sys; p=pathlib.Path(sys.argv[1]); b=p.read_bytes(); "
            "hashlib.sha256(b).hexdigest()==sys.argv[2] or sys.exit('Stage bytes changed'); "
            "sys.path.insert(0,str(p.parent)); sys.argv=[str(p)]+sys.argv[3:]; "
            "exec(compile(b,str(p),'exec'),{'__name__':'__main__','__file__':str(p)})")
    argv = [stage['python'], '-B', '-c', code, str(script), script_hash, *stage['args']]
    env = dict(os.environ, PYTHONDONTWRITEBYTECODE='1', PYTHONPATH=str(package))
    output = evidence / (stage['name'] + '.stdout.private.json')
    with output.open('xb') as stdout, (evidence / (stage['name'] + '.stderr.private.log')).open('xb') as stderr:
        control.run_child(argv, canonical(request), stdout, stderr, stage['timeout_seconds'],
                          cwd=package, env=env)
    if output.stat().st_size > 8 * 1024 * 1024:
        raise ValueError('Oversized stage result')
    result = json.loads(output.read_bytes())
    if (not isinstance(result, dict) or any(result.get(key) != request[key] for key in IDENTITY)
            or result.get('status') != 'passed' or result.get('checks') != {stage['name']: 'passed'}
            or not isinstance(result.get('owned_resources'), dict)):
        raise ValueError('Stage result identity or required check mismatch')
    return result


def qualify(request):
    os.umask(0o077)
    verify_assets(request)
    package = Path(request['qualification_package'])
    if not package.is_absolute() or package.resolve() != package:
        raise ValueError('Invalid qualification package directory')
    manifest = verify_package(package, request['qualification_manifest_sha256'])
    evidence = Path(request['directory']) / 'qualification'
    evidence.mkdir(mode=0o700)  # Never read or resume an old passed result.
    save_new(evidence / 'supervision.intent.json', {key: request[key] for key in IDENTITY})
    owned, previous, completed = {}, None, {}
    for stage in manifest['stages']:
        verify_assets(request)
        if verify_package(package, request['qualification_manifest_sha256']) != manifest:
            raise ValueError('Qualification package changed')
        child = dict(request, owned_resources=owned, previous_stage_result=previous,
                     package_configuration=manifest['configuration'])
        result = execute_stage(stage, package, child, evidence, manifest['files'][stage['script']]['sha256'])
        verify_package(package, request['qualification_manifest_sha256'])
        verify_assets(request)
        owned, previous = result['owned_resources'], result
        completed.update(result['checks'])
    result = {key: request[key] for key in IDENTITY}
    result.update(status='passed', checks=completed, owned_resources=owned,
                  qualification_manifest_sha256=request['qualification_manifest_sha256'])
    save_new(evidence / 'supervision.result.json', result)
    return result


def main():
    if sys.argv[1:] == ['--describe']:
        print(json.dumps({'ready': True, 'required_checks': list(CHECKS)}))
        return 0
    try:
        print(json.dumps(control.serve(qualify, sys.stdin), sort_keys=True))
        return 0
    except Exception:
        # Native output, credentials and private paths stay in private receipts.
        print('Qualification stopped; retain owned resources and private receipts.', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
