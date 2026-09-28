"""Host-runnable checks of the initializer's failure receipt.

The receipt may carry only the integer exit status of a sandboxed step, never its
command, input or output. Real isolation is covered by initialize_test.py inside a
packaged Runtime; these checks need no Runtime and run with plain python3.
"""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location('agents_api_runtime_initialize', Path(__file__).with_name('initialize.py'))
initialize = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(initialize)
CANARY = 'CANARY-initializer-receipt-5d1c'
SANDBOXED = ['/usr/bin/bwrap', '--unshare-user', '--', '/bin/bash', '-c', 'echo ' + CANARY + '; exit 3']


def invoke(failure):
    """Run main() with a request whose step raises failure; return (code, stdout, stderr)."""
    request = json.dumps({'version': 1, 'action': 'setup', 'network': 'enabled', 'command': 'echo ' + CANARY})
    stdin = mock.Mock()
    stdin.buffer = io.BytesIO(request.encode())
    stdout, stderr = io.StringIO(), io.StringIO()
    with mock.patch.object(initialize.sys, 'stdin', stdin), mock.patch.object(initialize.sys, 'argv', ['initialize']), \
            mock.patch.object(initialize, 'roots', lambda: None), mock.patch.object(initialize, 'run', side_effect=failure), \
            contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        code = initialize.main()
    return code, stdout.getvalue(), stderr.getvalue()


class FailureReceiptTest(unittest.TestCase):
    def assert_receipt(self, failure, expected):
        code, stdout, stderr = invoke(failure)
        self.assertEqual(code, 1)
        self.assertEqual(stderr, '')
        self.assertEqual(json.loads(stdout), expected)
        self.assertNotIn(CANARY, stdout)
        self.assertNotIn('echo', stdout)

    def test_configure_does_not_create_or_change_runtime_capabilities(self):
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory)
            capabilities = config / 'capabilities'
            capabilities.mkdir()
            manifest = capabilities / 'installed.json'
            manifest.write_bytes(b'fixed-runtime-fixture')
            with mock.patch.object(initialize, 'CONFIG', config):
                initialize.configure({'VALUE': 'configured'})
                self.assertEqual(manifest.read_bytes(), b'fixed-runtime-fixture')
                self.assertFalse((capabilities / 'skills').exists())
                self.assertIn('configured', (config / 'tool-env.sh').read_text())
                with self.assertRaises(FileExistsError):
                    initialize.configure({'VALUE': 'changed'})

    def test_skill_is_not_a_provider_initializer_action(self):
        with mock.patch.object(initialize, 'sandbox', return_value=[]), \
             mock.patch.object(initialize.subprocess, 'run') as run:
            with self.assertRaisesRegex(ValueError, 'invalid action'):
                initialize.run({'action': 'skill', 'network': 'enabled'})
            run.assert_not_called()

    def test_stdio_forwards_and_mounts_resolved_runtime_directories(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory).resolve()
            root, workspace = base / 'capabilities', base / 'actual-work'
            root.mkdir()
            workspace.mkdir()
            root, workspace = str(root), str(workspace)
            with mock.patch.object(initialize, 'roots') as roots, \
                 mock.patch.object(initialize, 'sandbox', return_value=['bwrap', '--']) as sandbox, \
                 mock.patch.object(initialize, 'stdio_lifetime', return_value=0) as launch:
                with mock.patch.object(initialize.sys, 'argv', ['initialize', 'stdio', root, workspace, 'plugins/0', 'server']):
                    self.assertEqual(initialize.main(), 0)
                roots.assert_called_once_with(workspace)
                sandbox.assert_called_once_with('enabled', '/workspace', workspace=workspace)
                argv = launch.call_args.args[0]
                self.assertEqual(argv[-5:], ['/tmp/oac-mcp-exec', 'runtime-mcp-exec', root, 'plugins/0', 'server'])
                self.assertIn(['--ro-bind', root, root], [argv[i:i+3] for i in range(len(argv)-2)])
            alias = base / 'alias'
            alias.symlink_to(workspace, target_is_directory=True)
            for invalid in ('/', 'relative', '/tmp/../secret', workspace + '/', workspace + '\n', str(alias)):
                for values in ((invalid, workspace), (root, invalid)):
                    with self.assertRaises(ValueError):
                        initialize.stdio(*values, 'plugins/0', 'server')

    def test_stdio_mounts_workspace_at_native_alias_and_physical_path(self):
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory)
            workspace = str(config / 'actual-work')
            with mock.patch.object(initialize, 'CONFIG', config):
                args = initialize.sandbox('enabled', '/workspace', workspace=workspace)
                mounts = [args[i:i+3] for i in range(len(args)-2)]
                self.assertIn(['--bind', workspace, '/workspace'], mounts)
                self.assertIn(['--bind', workspace, workspace], mounts)
                self.assertNotIn(['--bind', '/environment/workspace', '/workspace'], mounts)
                self.assertIn(['--ro-bind', str(config), str(config)], mounts)
                default = initialize.sandbox('enabled', '/workspace')
                self.assertIn(['--bind', '/environment/workspace', '/workspace'],
                              [default[i:i+3] for i in range(len(default)-2)])
                tools = initialize.runpy.run_path(str(Path(__file__).with_name('tool-root.py')))
                (config / 'system-root.json').write_text('{"version":1}')
                tools['installed'] = lambda: True
                with mock.patch.object(initialize.runpy, 'run_path', return_value=tools):
                    args = initialize.sandbox('enabled', '/workspace', workspace=workspace)
                mounts = [args[i:i+3] for i in range(len(args)-2)]
                self.assertIn(['--bind', workspace, '/workspace'], mounts)
                self.assertIn(['--bind', workspace, workspace], mounts)
                self.assertIn(['--ro-bind', '/environment/initialization', '/environment/initialization'], mounts)
                self.assertIn(['--chdir', '/workspace'], [args[i:i+2] for i in range(len(args)-1)])

    def test_private_initialization_uses_bound_workspace_and_empty_environment(self):
        with tempfile.TemporaryDirectory() as directory:
            workspace = str(Path(directory).resolve())
            stdin = mock.Mock()
            stdin.buffer = io.BytesIO(b'{"version":1,"action":"configure"}')
            with mock.patch.object(initialize.sys, 'argv', ['initialize', 'initialize', workspace]), \
                 mock.patch.object(initialize.sys, 'stdin', stdin), \
                 mock.patch.object(initialize, 'roots') as roots, \
                 mock.patch.object(initialize, 'run') as run, \
                 contextlib.redirect_stdout(io.StringIO()) as output:
                self.assertEqual(initialize.main(), 0)
                roots.assert_called_once_with(workspace)
                run.assert_called_once_with({'version': 1, 'action': 'configure'}, workspace=workspace)
                self.assertEqual(json.loads(output.getvalue()), {'version': 1, 'outcome': 'completed'})
            with mock.patch.object(initialize, 'configure') as configure:
                initialize.run({'action': 'configure'}, workspace=workspace)
                configure.assert_called_once_with({})

    def test_sandboxed_step_reports_only_its_exit_status(self):
        for status in (1, 3, 100, 255):
            failure = subprocess.CalledProcessError(status, SANDBOXED, output=CANARY.encode(), stderr=CANARY.encode())
            self.assert_receipt(failure, {'version': 1, 'outcome': 'failed', 'exit_code': status})
        # The receipt keeps its compact, stable encoding.
        _, stdout, _ = invoke(subprocess.CalledProcessError(3, SANDBOXED))
        self.assertEqual(stdout, '{"version":1,"outcome":"failed","exit_code":3}\n')

    def test_runtime_helpers_signals_and_other_errors_stay_generic(self):
        generic = {'version': 1, 'outcome': 'failed'}
        for failure in (
            subprocess.CalledProcessError(2, ['/usr/bin/tar', '-xzf', CANARY]),
            subprocess.CalledProcessError(1, ['/usr/local/bin/oac-codex-write', CANARY]),
            subprocess.CalledProcessError(-9, SANDBOXED),
            subprocess.CalledProcessError(256, SANDBOXED),
            subprocess.CalledProcessError(3, ' '.join(SANDBOXED)),
            subprocess.TimeoutExpired(SANDBOXED, 120, output=CANARY.encode()),
            ValueError(CANARY),
            OSError(CANARY),
        ):
            with self.subTest(failure=type(failure).__name__):
                self.assert_receipt(failure, generic)
        _, stdout, _ = invoke(ValueError(CANARY))
        self.assertEqual(stdout, '{"version":1,"outcome":"failed"}\n')

    def test_real_child_output_never_reaches_the_receipt(self):
        # A real process failure, launched like run() with discarded output.
        try:
            subprocess.run(['/bin/sh', '-c', 'echo ' + CANARY + '; echo ' + CANARY + ' >&2; exit 7'],
                           stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
        except subprocess.CalledProcessError as error:
            failure = subprocess.CalledProcessError(error.returncode, ['/usr/bin/bwrap', '--', *error.cmd])
        self.assert_receipt(failure, {'version': 1, 'outcome': 'failed', 'exit_code': 7})


if __name__ == '__main__':
    unittest.main()
