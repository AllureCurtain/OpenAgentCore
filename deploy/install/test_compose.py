"""Qualify the rendered Compose files without building images."""

import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("render_compose", ROOT / "scripts/render-compose.py")
render_compose = importlib.util.module_from_spec(spec)
spec.loader.exec_module(render_compose)


def rendered_compose(directory):
    text = render_compose.render({
        'REVISION': 'd' * 40,
        'RELEASE_BASE': 'https://example.com/releases/v1/',
        'ARCHIVE_CHECKSUM': 'e' * 64,
    })
    path = Path(directory) / 'compose.yaml'
    path.write_text(text)
    return path


class ComposeTests(unittest.TestCase):
    @classmethod
    def render(cls, public_url=None):
        env = dict(os.environ)
        env.pop('OAC_PUBLIC_URL', None)
        for name in ('OAC_IMAGE_CORE', 'OAC_IMAGE_WEB', 'OAC_IMAGE_INGRESS'):
            env.pop(name, None)
        env['OAC_DATA_DIR'] = '/tmp/oac-compose-fixture'
        if public_url is not None:
            env['OAC_PUBLIC_URL'] = public_url
        return json.loads(subprocess.check_output(
            ['docker', 'compose', '--env-file', os.devnull, '-f', str(cls.compose_file),
             '--profile', 'tools', 'config', '--format', 'json'], env=env))

    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.temporary.cleanup)
        cls.compose_file = rendered_compose(cls.temporary.name)
        cls.compose = cls.render()

    def test_compose_uses_private_services_and_ordered_initialization(self):
        services = self.compose['services']
        self.assertEqual(services['database']['depends_on']['init']['condition'], 'service_completed_successfully')
        self.assertEqual(services['migrate']['depends_on']['database']['condition'], 'service_healthy')
        self.assertIn('pg_isready -h 127.0.0.1', services['database']['healthcheck']['test'][1])
        self.assertEqual(services['core']['depends_on']['migrate']['condition'], 'service_completed_successfully')
        for service in services.values():
            self.assertNotIn('build', service)
            self.assertNotIn('ports', service)
            self.assertTrue(service['image'].endswith(':latest') or service['image'] == 'postgres:16-alpine')
            for volume in service.get('volumes', []):
                self.assertNotIn('docker.sock', json.dumps(volume))
                self.assertEqual(volume['type'], 'bind')
        self.assertEqual({v['target'] for v in services['web']['volumes']}, {'/run/oac', '/node-payload', '/domain'})
        self.assertEqual(services['credentials']['logging']['driver'], 'none')
        self.assertEqual(services['init']['command'], ['/usr/local/bin/oac', 'init'])
        self.assertEqual(services['gateway']['healthcheck']['test'], ['CMD', '/usr/local/bin/oac', 'healthcheck'])
        self.assertNotIn('python3', json.dumps(self.compose))
        self.assertEqual(services['init']['environment']['OAC_REVISION'], 'd' * 40)
        self.assertEqual(services['core']['environment']['OAC_HARNESSES'].split(','), ['claude_sdk', 'codex', 'mcode'])
        self.assertEqual(services['core']['environment']['OAC_DEFAULT_HARNESS'], 'codex')

    def test_public_url_can_be_configured_after_initial_startup(self):
        for value in (None, '', 'https://oac.example.test', 'http://localhost:9080'):
            with self.subTest(public_url=value):
                configured = self.render(value)
                expected = value or 'http://localhost:8080'
                for name, setting in (('core', 'OAC_PUBLIC_URL'), ('migrate', 'OAC_PUBLIC_URL'),
                                      ('web', 'OAC_WEB_ORIGIN')):
                    self.assertEqual(configured['services'][name]['environment'][setting], expected)
                self.assertEqual(
                    {service: [item.get('target') for item in spec.get('volumes', [])]
                     for service, spec in configured['services'].items()},
                    {service: [item.get('target') for item in spec.get('volumes', [])]
                     for service, spec in self.compose['services'].items()})

    def test_managed_https_runs_domain_setup_inside_the_gateway(self):
        env = dict(os.environ, OAC_DATA_DIR='/tmp/oac-compose-fixture', OAC_INSTALL_DIR='/tmp/oac-install-fixture')
        managed = json.loads(subprocess.check_output(
            ['docker', 'compose', '--env-file', os.devnull, '-f', str(self.compose_file),
             '-f', str(ROOT / 'deploy/compose/https.yaml'), 'config', '--format', 'json'], env=env))
        services = managed['services']
        self.assertEqual(sorted(services), ['core', 'database', 'gateway', 'init', 'migrate', 'web'])
        self.assertEqual(services['gateway']['command'], ['/usr/local/bin/oac', 'gateway'])
        self.assertEqual({port['published'] for port in services['gateway']['ports']}, {'80', '443'})
        for name, service in services.items():
            sockets = [v for v in service.get('volumes', []) if v.get('source') == '/var/run/docker.sock']
            self.assertEqual(len(sockets), 1 if name == 'gateway' else 0, name)

    def test_platform_network_injection_keeps_the_credentials_profile_valid(self):
        # Dokploy isolated deployments attach a project network to every service.
        transformed = copy.deepcopy(self.compose)
        transformed['networks']['platform'] = {}
        for service in transformed['services'].values():
            service.setdefault('networks', {})['platform'] = None
        subprocess.run(
            ['docker', 'compose', '-f', '-', '--profile', 'tools', 'config', '--quiet'],
            input=json.dumps(transformed), text=True, check=True)


if __name__ == '__main__':
    unittest.main()
