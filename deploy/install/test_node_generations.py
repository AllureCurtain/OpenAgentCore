"""Collection retains bytes until local helper ownership settles."""
import fcntl
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import node_generations
import node_install as installer
import node_spec


class CollectionTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.directory = self.root / "state/node/generations"
        self.directory.mkdir(parents=True, mode=0o700)
        self.value = {"generation": 1, "provider": "docker", "docker": {"image": "sha256:" + "a" * 64},
                      "specification": {"resources": {"cpus": 1, "memory_mib": 1024}, "runtime": {"source_commit": "b" * 40, "image_id": "sha256:" + "a" * 64, "image_manifest_digest": "sha256:" + "c" * 64, "microsandbox_ref": "oac-runtime@sha256:" + "d" * 64, "runtime_sha256": "e" * 64, "firmware_sha256": "f" * 64}}}
        self.args = SimpleNamespace(generation=1, specification_digest=node_spec.digest("docker", self.value["specification"]))
        self.release = self.root / "releases" / ("b" * 40)
        self.release.mkdir(parents=True)
        (self.release / "artifact").write_bytes(b"immutable bytes")
        node_generations.atomic_json(self.root / "provider.json", self.value)
        for patch in (mock.patch.object(node_generations, "owned_root", return_value=(self.root, {})),
                      mock.patch.object(installer, "checked", return_value="")):
            patch.start()
            self.addCleanup(patch.stop)

    def test_busy_helper_refuses_all_mutations_then_same_inode_collects(self):
        lease = self.directory / "1.lease"
        descriptor = os.open(lease, os.O_CREAT | os.O_RDWR, 0o600)
        self.addCleanup(os.close, descriptor)
        original = os.fstat(descriptor)
        fcntl.flock(descriptor, fcntl.LOCK_SH)
        with self.assertRaisesRegex(installer.InstallError, "helper is still active"):
            node_generations.collect(self.args, installer)
        installer.checked.assert_not_called()
        self.assertEqual((self.release / "artifact").read_bytes(), b"immutable bytes")
        self.assertFalse((self.directory / "1.dropped").exists())
        fcntl.flock(descriptor, fcntl.LOCK_UN)
        node_generations.collect(self.args, installer)
        self.assertFalse(self.release.exists())
        self.assertTrue((self.directory / "1.dropped").exists())
        node_generations.collect(self.args, installer)
        installer.checked.assert_called_once()
        final = lease.stat()
        self.assertEqual((original.st_dev, original.st_ino), (final.st_dev, final.st_ino))

    def test_collection_holds_exclusive_lease_during_deletion(self):
        lease = self.directory / "1.lease"
        def remove(*_args, **_kwargs):
            descriptor = os.open(lease, os.O_RDWR)
            try:
                with self.assertRaises(BlockingIOError):
                    fcntl.flock(descriptor, fcntl.LOCK_SH | fcntl.LOCK_NB)
            finally:
                os.close(descriptor)
            return ""
        installer.checked.side_effect = remove
        node_generations.collect(self.args, installer)

    def test_symlink_and_shared_inode_are_not_collection_authority(self):
        lease = self.directory / "1.lease"
        foreign = self.root / "foreign"
        foreign.write_bytes(b"")
        foreign.chmod(0o600)
        for link in (os.symlink, os.link):
            link(foreign, lease)
            try:
                with self.assertRaises((OSError, installer.InstallError)):
                    node_generations.collect(self.args, installer)
                installer.checked.assert_not_called()
                self.assertTrue((self.release / "artifact").exists())
            finally:
                lease.unlink()


if __name__ == "__main__":
    unittest.main()
