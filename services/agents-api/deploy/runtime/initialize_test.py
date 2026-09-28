"""Real Linux initialization checks; run inside a disposable packaged Runtime.

The fixture must expose writable workspace/packages/initialization roots and
execute with the launching user's permissions. No model is mocked;
these checks exercise initialization only, not public native-model acceptance.
"""
import json
import os
from pathlib import Path
import subprocess
import sys

HELPER = '/usr/local/bin/oac-runtime-initialize'
CANARY = 'private-initialization-canary-47a8'


def invoke(action, *, succeeds=True, exit_code=None, **fields):
    payload = json.dumps({'version': 1, 'action': action, 'network': 'enabled', **fields})
    result = subprocess.run(['/usr/bin/python3', '-I', '-S', HELPER], input=payload,
                            text=True, capture_output=True, timeout=120)
    expected = {'version': 1, 'outcome': 'completed' if succeeds else 'failed'}
    if exit_code is not None:
        # A failed initialization step reports only its exit status, never its output.
        expected['exit_code'] = exit_code
    assert result.returncode == (0 if succeeds else 1), (action, result.returncode)
    assert result.stderr == '', (action, 'unexpected stderr')
    assert json.loads(result.stdout) == expected, (action, result.stdout)
    assert CANARY not in result.stdout + result.stderr, 'confidential output exposed'


def main():
    for name in ('workspace', 'packages', 'initialization', 'private', 'staging'):
        Path('/environment', name).mkdir(exist_ok=True)
    Path('/environment/private/credential').write_text(CANARY)
    Path('/environment/staging/request').write_text(CANARY)
    os.environ['DAEMON_PRIVATE_CANARY'] = CANARY
    env = {'INITIALIZATION_VALUE': CANARY, 'WITH_QUOTES': "'\n$(false)"}
    if os.environ.get('OAC_TEST_PACKAGE_PROXY'):
        env.update(http_proxy=os.environ['OAC_TEST_PACKAGE_PROXY'],
                   https_proxy=os.environ['OAC_TEST_PACKAGE_PROXY'])
    invoke('configure', env=env)
    # A fixed Runtime-installed fixture tests setup access, not archive parsing.
    skill = Path('/environment/initialization/capabilities/skills/proof')
    (skill / 'scripts').mkdir(parents=True)
    (skill / 'SKILL.md').write_text('---\nname: proof\ndescription: A proof.\n---\nRead check.sh.')
    (skill / 'SKILL.md').chmod(0o400)
    (skill / 'scripts/check.sh').write_text('#!/bin/sh\nprintf skill-proof')
    (skill / 'scripts/check.sh').chmod(0o500)
    invoke('setup', command='/environment/initialization/capabilities/skills/proof/scripts/check.sh > skill-result')
    assert Path('/environment/workspace/skill-result').read_text() == 'skill-proof'
    if '--system' in sys.argv:
        invoke('system', packages=['jq', 'build-essential', 'libpq-dev'])
        invoke('system', succeeds=False, packages=['jq'])
        # The system installer currently retains its own package-root contract.
        # Its native execution checks are migrated with that installer.


    # Re-entry must not replace confidential configuration after any effects.
    invoke('configure', succeeds=False, env={'INITIALIZATION_VALUE': 'changed'})
    invoke('setup', command='printf "%s" "$INITIALIZATION_VALUE" > first; printf secret; printf secret >&2')
    assert Path('/environment/workspace/first').read_text() == CANARY
    check = '''import os, pathlib
for path in ('/environment/private/credential', '/environment/staging/request'):
    assert pathlib.Path(path).read_text() == 'private-initialization-canary-47a8'
assert 'DAEMON_PRIVATE_CANARY' not in os.environ
assert os.environ['INITIALIZATION_VALUE'] == 'private-initialization-canary-47a8'
assert os.environ['WITH_QUOTES'] == "'\\n$(false)"
pathlib.Path('/environment/packages/visible').write_text('ok')
'''
    Path('/environment/workspace/check.py').write_text(check)
    invoke('setup', network='disabled', command='/usr/bin/python3 check.py')
    # Shell cwd is explicit and ordered effects survive between invocations.
    Path('/environment/workspace/sub').mkdir()
    invoke('setup', cwd='/workspace/sub', command='test -f ../first && pwd > second')
    assert Path('/environment/workspace/sub/second').read_text() == '/environment/workspace/sub\n'
    invoke('setup', succeeds=False, exit_code=7, command='echo secret; echo secret >&2; exit 7')
    invoke('setup', succeeds=False, exit_code=3, command='echo "$INITIALIZATION_VALUE"; echo "$INITIALIZATION_VALUE" >&2; exit 3')
    # Failure to enter the requested cwd is a generic launcher failure.
    invoke('setup', succeeds=False, cwd='/workspace/missing', command='touch should-not-exist')
    assert not Path('/environment/workspace/should-not-exist').exists()
    if '--packages' in sys.argv:
        # Actual public registries, not synthetic package fixtures.
        invoke('npm', packages=['is-number@7.0.0'])
        invoke('python', packages=['packaging==26.0'])
        # pip's diagnostics name the package and can echo configuration; only its status is reported.
        invoke('python', succeeds=False, exit_code=1, packages=['oac-initializer-nonexistent-4f7e-zz'])
        invoke('setup', cwd='/workspace/sub', command="node -e \"if (!require('/environment/packages/npm/lib/node_modules/is-number')(42)) process.exit(1)\" && python3 -c 'import packaging; assert packaging.__version__ == \"26.0\"'")
    print(json.dumps({'initialization': 'passed', 'real_packages': '--packages' in sys.argv,
                      'system_packages': '--system' in sys.argv}))


if __name__ == '__main__':
    main()
