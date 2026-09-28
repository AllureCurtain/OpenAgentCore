"""Host-runnable initialization and finite-receipt checks; no packages downloaded."""
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location('agents_api_runtime_initialize', Path(__file__).with_name('initialize.py'))
initialize = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(initialize)
CANARY = 'CANARY-initializer-receipt-5d1c'


def invoke(failure):
    request = json.dumps({'version': 1, 'action': 'setup', 'network': 'enabled', 'command': 'echo ' + CANARY})
    stdin = mock.Mock()
    stdin.buffer = io.BytesIO(request.encode())
    stdout, stderr = io.StringIO(), io.StringIO()
    with mock.patch.object(initialize.sys, 'stdin', stdin), mock.patch.object(initialize.sys, 'argv', ['initialize']), \
            mock.patch.object(initialize, 'roots', lambda: None), mock.patch.object(initialize, 'run', side_effect=failure), \
            contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        code = initialize.main()
    return code, stdout.getvalue(), stderr.getvalue()


class InitializationTest(unittest.TestCase):
    def test_configure_preserves_runtime_capabilities_and_freezes_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory)
            capabilities = config / 'capabilities'
            capabilities.mkdir()
            manifest = capabilities / 'installed.json'
            manifest.write_bytes(b'fixed-runtime-fixture')
            with mock.patch.object(initialize, 'CONFIG', config):
                initialize.configure({'VALUE': "configured'\n$(false)"})
                self.assertEqual(manifest.read_bytes(), b'fixed-runtime-fixture')
                self.assertFalse((capabilities / 'skills').exists())
                self.assertEqual(json.loads((config / 'tool-env.json').read_text())['VALUE'], "configured'\n$(false)")
                with self.assertRaises(FileExistsError):
                    initialize.configure({'VALUE': 'changed'})

    def test_skill_is_not_an_initializer_action(self):
        with mock.patch.object(initialize.subprocess, 'run') as run:
            with self.assertRaisesRegex(ValueError, 'invalid action'):
                initialize.run({'action': 'skill', 'network': 'enabled'})
            run.assert_not_called()

    def test_setup_executes_directly_with_resolved_cwd_and_configured_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            workspace, config, packages, capabilities = [base / name for name in ('work', 'config', 'packages', 'caps')]
            for path in (workspace / 'sub', config, packages, capabilities):
                path.mkdir(parents=True)
            skill = capabilities / 'proof.sh'
            skill.write_text('#!/bin/sh\nprintf skill-proof')
            secret = base / 'same-user-file'
            secret.write_text('ordinary-user-access')
            import shlex
            command = ('/bin/sh ' + shlex.quote(str(skill)) + ' > skill-result; '
                       'cat ' + shlex.quote(str(secret)) + ' > readable; '
                       'printf "%s" "$VALUE" > value; pwd > cwd; '
                       'test -z "${DAEMON_PRIVATE_CANARY+x}"')
            with mock.patch.object(initialize, 'CONFIG', config), mock.patch.object(initialize, 'PACKAGES', packages), \
                 mock.patch.dict(os.environ, {'DAEMON_PRIVATE_CANARY': CANARY}):
                initialize.configure({'VALUE': "'\n$(false)"})
                initialize.run({'action': 'setup', 'network': 'disabled', 'cwd': '/workspace/sub', 'command': command},
                               workspace=str(workspace), capability_root=str(capabilities))
            self.assertEqual((workspace / 'sub/skill-result').read_text(), 'skill-proof')
            self.assertEqual((workspace / 'sub/readable').read_text(), 'ordinary-user-access')
            self.assertEqual((workspace / 'sub/value').read_text(), "'\n$(false)")
            self.assertEqual((workspace / 'sub/cwd').read_text(), str(workspace / 'sub') + '\n')

    def test_package_managers_use_direct_argv_and_frozen_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory)
            (config / 'tool-env.json').write_text(json.dumps({'VALUE': CANARY}))
            for action, executable in (('npm', 'npm'), ('python', '/usr/bin/python3')):
                with self.subTest(action=action), mock.patch.object(initialize, 'CONFIG', config), \
                     mock.patch.object(initialize.subprocess, 'run') as run:
                    initialize.run({'action': action, 'network': 'enabled', 'packages': ['valid-package']}, workspace=directory)
                    self.assertEqual(run.call_args.args[0][0], executable)
                    self.assertEqual(run.call_args.args[0][-2:], ['--', 'valid-package'])
                    self.assertEqual(run.call_args.kwargs['cwd'], directory)
                    self.assertEqual(run.call_args.kwargs['env']['VALUE'], CANARY)
                    for channel in ('stdin', 'stdout', 'stderr'):
                        self.assertEqual(run.call_args.kwargs[channel], subprocess.DEVNULL)
                    with self.assertRaises(ValueError):
                        initialize.run({'action': action, 'network': 'enabled', 'packages': ['--bad']}, workspace=directory)

    def test_system_still_delegates_to_existing_installer(self):
        install = mock.Mock()
        with mock.patch.object(initialize.runpy, 'run_path', return_value={'install': install}) as load:
            initialize.run({'action': 'system', 'packages': ['jq']}, workspace='/actual/work')
        load.assert_called_once_with('/usr/local/bin/oac-tool-root')
        install.assert_called_once_with(['jq'], workspace='/actual/work')

    def test_cwd_uses_only_logical_workspace_paths(self):
        self.assertEqual(initialize.execution_directory('/workspace/sub', '/actual'), '/actual/sub')
        for invalid in ('/workspace/../outside', '/workspace/', '/elsewhere', 'relative', '/workspace/x\n'):
            with self.subTest(cwd=invalid), self.assertRaises(ValueError):
                initialize.execution_directory(invalid, '/actual')

    def test_private_initialization_uses_bound_workspace_and_empty_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            workspace = str(Path(directory))
            stdin = mock.Mock()
            stdin.buffer = io.BytesIO(b'{"version":1,"action":"configure"}')
            with mock.patch.object(initialize.sys, 'argv', ['initialize', 'initialize', workspace, workspace + '/caps']), \
                 mock.patch.object(initialize.sys, 'stdin', stdin), \
                 mock.patch.object(initialize, 'roots') as roots, mock.patch.object(initialize, 'run') as run, \
                 contextlib.redirect_stdout(io.StringIO()) as output:
                self.assertEqual(initialize.main(), 0)
                roots.assert_called_once_with(workspace)
                run.assert_called_once_with({'version': 1, 'action': 'configure'}, workspace=workspace, capability_root=workspace + '/caps')
                self.assertEqual(json.loads(output.getvalue()), {'version': 1, 'outcome': 'completed'})
            with mock.patch.object(initialize, 'configure') as configure:
                initialize.run({'action': 'configure'}, workspace=workspace)
                configure.assert_called_once_with({})

    def test_invalid_invocation_and_oversized_input_remain_finite(self):
        for argv, data in ((['initialize', 'stdio', 'unused'], b''), (['initialize'], b'x' * 33)):
            stdin = mock.Mock()
            stdin.buffer = io.BytesIO(data)
            with mock.patch.object(initialize.sys, 'argv', argv), mock.patch.object(initialize.sys, 'stdin', stdin), \
                 mock.patch.object(initialize, 'MAX_INPUT', 32), contextlib.redirect_stdout(io.StringIO()) as output:
                self.assertEqual(initialize.main(), 1)
                self.assertEqual(json.loads(output.getvalue()), {'version': 1, 'outcome': 'failed'})

    def test_step_reports_only_its_exit_status(self):
        for status in (1, 3, 100, 255):
            code, stdout, stderr = invoke(initialize.StepFailure(status))
            self.assertEqual(code, 1)
            self.assertEqual(stderr, '')
            self.assertEqual(json.loads(stdout), {'version': 1, 'outcome': 'failed', 'exit_code': status})
            self.assertNotIn(CANARY, stdout)
        self.assertEqual(invoke(initialize.StepFailure(3))[1], '{"version":1,"outcome":"failed","exit_code":3}\n')

    def test_runtime_helpers_signals_and_other_errors_stay_generic(self):
        for failure in (initialize.StepFailure(-9), initialize.StepFailure(256),
                        subprocess.CalledProcessError(2, ['/usr/bin/tar', CANARY]),
                        subprocess.TimeoutExpired([CANARY], 120), ValueError(CANARY), OSError(CANARY)):
            code, stdout, stderr = invoke(failure)
            self.assertEqual((code, stderr), (1, ''))
            self.assertEqual(json.loads(stdout), {'version': 1, 'outcome': 'failed'})
            self.assertNotIn(CANARY, stdout)

    def test_real_child_failure_has_finite_receipt(self):
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory)
            (config / 'tool-env.json').write_text('{}')
            with mock.patch.object(initialize, 'CONFIG', config):
                with self.assertRaises(initialize.StepFailure) as failure:
                    initialize.run({'action': 'setup', 'network': 'enabled',
                                    'command': 'echo ' + CANARY + '; echo ' + CANARY + ' >&2; exit 7'}, workspace=directory)
            self.assertEqual(initialize.failed_receipt(failure.exception), '{"version":1,"outcome":"failed","exit_code":7}')


if __name__ == '__main__':
    unittest.main()
