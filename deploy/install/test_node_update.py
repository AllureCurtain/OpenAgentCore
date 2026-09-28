"""Program updates preserve enrolled state and resume their pinned payload."""
import contextlib
import hashlib
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock
import node_generations
import node_install as installer
import node_spec
import node_update


class UpdateTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.home = Path(temporary.name)
        self.args = SimpleNamespace(installation_id="94be54a1-138c-4f30-bc87-b13686272dbe", core_url="https://core.example", source_url="https://console.example", bundle=None, provider=None)
        self.root = self.home / ".oac/nodes" / self.args.installation_id
        (self.root / "state/node").mkdir(parents=True, mode=0o700)
        (self.root / installer.COMMON[0]).parent.mkdir(parents=True, mode=0o700)
        self.old = b'#!/bin/sh\necho "usage: oac-node register|run --config PATH --state-dir PATH" >&2\nexit 1\n'
        self.new = b'#!/bin/sh\necho 2\n'
        binary = self.root / installer.COMMON[0]
        binary.write_bytes(self.old)
        binary.chmod(0o700)
        runtime = {"source_commit": "b" * 40, "image_id": "sha256:" + "a" * 64, "image_manifest_digest": "sha256:" + "c" * 64, "microsandbox_ref": "oac-runtime@sha256:" + "d" * 64, "runtime_sha256": "e" * 64, "firmware_sha256": "f" * 64}
        spec = {"runtime": runtime, "resources": {"cpus": 1, "memory_mib": 1024}}
        self.provider = {"generation": 1, "provider": "docker", "installation_id": self.args.installation_id, "specification": spec, "docker": {"image": runtime["image_id"], "seccomp_file": str(self.root / "runtime/seccomp.json")}}
        self.identity = {"core_url": self.args.core_url, "credential": "a" * 64, "identity": {"installation_id": self.args.installation_id, "provider": "docker", "node_id": "7073346d-3c83-4361-9354-2709a9cf17ed", "deployment_generation": 1, "specification_digest": node_spec.digest("docker", spec)}}
        for name, value in (("provider.json", self.provider), ("state/node/identity.json", self.identity), ("registered.json", {})):
            node_generations.atomic_json(self.root / name, value)
        unit = self.root / installer.unit_name(self.args.installation_id)
        unit.write_text(installer.service_unit(self.root))
        unit.chmod(0o600)
        self.preparer = b"verified fixture preparer"
        checksum = hashlib.sha256(self.new).hexdigest()
        self.manifest = {"source_commit": "c" * 40, "artifacts": {installer.COMMON[0]: {"filename": "node-" + "c" * 40, "size": len(self.new), "sha256": checksum}}}
        self.sums = {"node-install.pyz": hashlib.sha256(self.preparer).hexdigest()}
        self.original = {name: (self.root / name).read_bytes() for name in ("provider.json", "registered.json", "state/node/identity.json")}
        def obtain(_manifest, _name, target, _bundle):
            target.write_bytes(self.new)
        def helper(stage, args, _installer):
            (stage / "generation-preparer.pyz").write_bytes(self.preparer)
            (stage / "generation-preparer.pyz").chmod(0o600)
            node_generations.atomic_json(stage / "preparation.json", {"source_url": args.source_url})
        for patch in (mock.patch.object(Path, "home", return_value=self.home), mock.patch.object(installer.node_spec, "fetch", return_value={"generation": 9}), mock.patch.object(installer, "metadata", return_value=(self.manifest, self.sums)), mock.patch.object(installer.distribution, "obtain_artifact", side_effect=obtain), mock.patch.object(node_generations, "install_helper", side_effect=helper), mock.patch.object(installer, "wait_ready"), mock.patch.object(installer, "checked", return_value="")):
            patch.start()
            self.addCleanup(patch.stop)

    def assert_original_state(self):
        for name, raw in self.original.items():
            self.assertEqual((self.root / name).read_bytes(), raw, name)

    def test_interruption_after_stop_keeps_pinned_payload_and_legacy_generation(self):
        marker = self.root / "state/node/generations/1.legacy-unfenced"
        def interrupted(arguments, *_args, **_kwargs):
            if "stop" in arguments:
                self.assertEqual(json.loads(marker.read_text())["specification_digest"], self.identity["identity"]["specification_digest"])
                self.assertEqual(node_update.journal(self.root, installer)["phase"], "prepared")
                raise installer.InstallError("interrupted after stop")
        installer.checked.side_effect = interrupted
        with self.assertRaisesRegex(installer.InstallError, "interrupted"):
            node_update.update(self.args, installer)
        self.assert_original_state()
        self.assertEqual((self.root / installer.COMMON[0]).read_bytes(), self.old)
        installer.metadata.side_effect = AssertionError("resume re-fetched current release")
        installer.checked.side_effect = None
        node_update.update(self.args, installer)
        self.assertEqual((self.root / installer.COMMON[0]).read_bytes(), self.new)
        self.assertEqual(node_update.journal(self.root, installer)["phase"], "complete")
        self.assert_original_state()
        old_sha = hashlib.sha256(self.old).hexdigest()
        self.assertEqual((self.root / "node-programs" / old_sha / "previous-node").read_bytes(), self.old)
        installer.node_spec.fetch.assert_called_with(self.args, "", self.identity, installer.open_request, allow_selection_change=True)
        with mock.patch.object(node_generations, "owned_root", return_value=(self.root, self.identity)):
            installer.checked.reset_mock()
            grant = SimpleNamespace(installation_id=self.args.installation_id, generation=1, specification_digest=self.identity["identity"]["specification_digest"])
            with self.assertRaisesRegex(installer.InstallError, "unfenced legacy helpers"):
                node_generations.collect(grant, installer)
            installer.checked.assert_not_called()
            later = json.loads(json.dumps(self.provider))
            later["generation"] = 2
            later["docker"]["image"] = "sha256:" + "9" * 64
            node_generations.atomic_json(self.root / "state/node/generations/2.json", later)
            grant.generation = 2
            with node_generations.collection_lease(self.root, 2, installer, node_generations.marker_identity(grant), initialize=True):
                pass
            node_generations.collect(grant, installer)
            self.assertTrue((self.root / "state/node/generations/2.dropped").exists())
            self.assertTrue(marker.exists())

    def test_v2_update_does_not_invent_legacy_retention(self):
        (self.root / installer.COMMON[0]).write_bytes(self.new)
        node_update.update(self.args, installer)
        self.assertFalse((self.root / "state/node/generations/1.legacy-unfenced").exists())
        self.assert_original_state()

    def test_unknown_original_generation_refuses_before_staging_or_stop(self):
        self.identity["identity"]["deployment_generation"] = 0
        node_generations.atomic_json(self.root / "state/node/identity.json", self.identity)
        with self.assertRaisesRegex(installer.InstallError, "original enrolled generation"):
            node_update.update(self.args, installer)
        installer.metadata.assert_not_called()
        installer.checked.assert_not_called()
        self.assertFalse((self.root / "program-update.json").exists())

    def test_corrupt_preparer_never_stops_old_service(self):
        self.sums["node-install.pyz"] = "0" * 64
        with self.assertRaisesRegex(installer.InstallError, "checksum-matched"):
            node_update.update(self.args, installer)
        installer.checked.assert_not_called()
        self.assert_original_state()

    def test_partial_atomic_install_reuses_verified_stage(self):
        node_update.prepare(self.root, self.args, installer)
        original = node_update.atomic_copy
        def interrupted(source, target, mode):
            if target.name == "generation-preparer.pyz":
                raise installer.InstallError("interrupted between files")
            original(source, target, mode)
        with mock.patch.object(node_update, "atomic_copy", side_effect=interrupted):
            with self.assertRaisesRegex(installer.InstallError, "between files"):
                node_update.apply(self.root, self.args, installer)
        installer.metadata.side_effect = AssertionError("resume re-fetched release")
        node_update.prepare(self.root, self.args, installer)
        node_update.apply(self.root, self.args, installer)
        self.assertEqual((self.root / "generation-preparer.pyz").read_bytes(), self.preparer)
        self.assert_original_state()

    def test_root_runs_only_fixed_service_operations_and_drops_privileges_for_files(self):
        account = SimpleNamespace(pw_uid=12345, pw_gid=12345, pw_dir=str(self.home))
        record = {"provider": "docker", "core_url": self.args.core_url}
        units = self.home / "system-units"
        units.mkdir()
        unit = units / installer.unit_name(self.args.installation_id)
        unit.write_text(installer.system_unit(self.root, "docker"))
        with mock.patch.object(node_update.os, "geteuid", return_value=0), mock.patch.object(installer, "host_checks"), mock.patch.object(installer, "node_record", return_value=record), mock.patch.object(installer, "account_plan", return_value=(account, {"uid":12345})), mock.patch.object(installer, "service_account", return_value=account), mock.patch.object(installer, "SERVICE_HOME", self.home), mock.patch.object(installer, "SYSTEM_UNITS", units), mock.patch.object(installer, "host_lock", side_effect=contextlib.nullcontext), mock.patch.object(installer, "run_as") as run_as, mock.patch.object(installer, "prepare_account", side_effect=AssertionError("update recreated account")):
            node_update.update(self.args, installer)
        self.assertEqual([call.args[:2] for call in run_as.call_args_list], [(account, node_update.prepare), (account, node_update.apply), (account, node_update.finish)])
        self.assertEqual([call.args[0] for call in installer.checked.call_args_list], [["systemctl", "stop", unit.name], ["systemctl", "start", unit.name]])
        installer.metadata.assert_not_called()
        self.assert_original_state()


if __name__ == "__main__":
    unittest.main()
