"""Release publication contract tests; no GitHub writes or Docker builds."""
import contextlib
import hashlib
import importlib.util
import json
import pathlib
import subprocess
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location(
    "publisher", pathlib.Path(__file__).with_name("publish-core-release.py"))
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class PublicationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.assets = pathlib.Path(self.temp.name)
        self.revision = "a" * 40
        self.stem = "oac-" + self.revision + "-linux-amd64"
        for suffix in (".tar.gz", "-offline.tar.gz"):
            name = self.stem + suffix
            (self.assets / name).write_bytes(b"archive fixture")
            (self.assets / (name + ".sha256")).write_text(
                hashlib.sha256(b"archive fixture").hexdigest() + "  " + name + "\n")
        stack = contextlib.ExitStack()
        self.addCleanup(stack.close)
        self.gh_run = stack.enter_context(mock.patch.object(publisher.subprocess, "run"))
        self.api = stack.enter_context(mock.patch.object(publisher.subprocess, "check_output",
            side_effect=self.response))

    def response(self, command, **kwargs):
        if "git/" in command[2]:
            return json.dumps({"object": {"type": "commit", "sha": self.revision}})
        return json.dumps({"id": 7, "draft": "--method" not in command,
                           "tag_name": "v1.2.3", "target_commitish": self.revision})

    def publish(self, tag="v1.2.3", mode="publish"):
        publisher.publish(self.assets, "MiniMax-AI/parsar-core", self.revision, tag, mode)

    def test_version_tag_publishes_all_assets(self):
        self.publish()
        command = self.gh_run.call_args.args[0]
        self.assertIn("--verify-tag", command)
        self.assertIn("--draft", command)
        self.assertNotIn("--prerelease", command)
        self.assertNotIn("--clobber", command)
        self.assertEqual(command[-4:], sorted(str(p.resolve()) for p in self.assets.iterdir()))
        self.assertEqual(self.api.call_args.args[0][2:],
            ["repos/MiniMax-AI/parsar-core/releases/7", "--method", "PATCH", "-F", "draft=false"])

    def test_annotated_tag_and_prerelease(self):
        def response(command, **kwargs):
            endpoint = command[2]
            if "git/ref/" in endpoint:
                return json.dumps({"object": {"type": "tag", "sha": "b" * 40}})
            return self.response(command).replace("v1.2.3", "v1.2.3-rc.1")
        self.api.side_effect = response
        self.publish("v1.2.3-rc.1")
        self.assertIn("--prerelease", self.gh_run.call_args.args[0])
        self.assertEqual(sum("git/tags/" in c.args[0][2] for c in self.api.call_args_list), 2)

    def test_build_metadata_is_not_prerelease(self):
        self.api.side_effect = lambda c, **kw: self.response(c).replace("v1.2.3", "v1.2.3+build-test")
        self.publish("v1.2.3+build-test")
        self.assertNotIn("--prerelease", self.gh_run.call_args.args[0])

    def test_manual_draft_does_not_publish(self):
        (self.assets / (self.stem + "-offline.tar.gz")).unlink()
        (self.assets / (self.stem + "-offline.tar.gz.sha256")).unlink()
        self.publish("build-" + self.revision, "draft")
        self.assertIn("--draft", self.gh_run.call_args.args[0])
        self.api.assert_not_called()

    def test_wrong_tag_revision_is_refused(self):
        self.api.side_effect = lambda *a, **kw: json.dumps({"object": {"type": "commit", "sha": "b" * 40}})
        with self.assertRaisesRegex(ValueError, "built source"):
            self.publish()
        self.gh_run.assert_not_called()

    def test_failed_tag_lookup_never_creates_release(self):
        self.api.side_effect = subprocess.CalledProcessError(1, ["gh", "api"])
        with self.assertRaises(subprocess.CalledProcessError):
            self.publish()
        self.gh_run.assert_not_called()

    def test_failed_or_existing_release_is_not_overwritten_or_replayed(self):
        self.gh_run.side_effect = subprocess.CalledProcessError(1, ["gh", "release", "create"])
        with self.assertRaises(subprocess.CalledProcessError):
            self.publish()
        self.assertEqual(self.gh_run.call_count, 1)

    def test_invalid_version_tag_is_refused(self):
        with self.assertRaisesRegex(ValueError, "Version tags"):
            self.publish("version1")
        self.gh_run.assert_not_called()

    def test_missing_offline_archive_is_refused(self):
        (self.assets / (self.stem + "-offline.tar.gz")).unlink()
        with self.assertRaises(FileNotFoundError):
            self.publish()
        self.gh_run.assert_not_called()

    def test_changed_archive_is_refused(self):
        (self.assets / (self.stem + ".tar.gz")).write_bytes(b"changed")
        with self.assertRaisesRegex(ValueError, "checksum"):
            self.publish()
        self.gh_run.assert_not_called()

    def test_empty_or_linked_payload_is_refused(self):
        for p in self.assets.iterdir():
            p.unlink()
        with self.assertRaisesRegex(ValueError, "nonempty"):
            self.publish()
        (self.assets / "link").symlink_to(__file__)
        with self.assertRaisesRegex(ValueError, "regular"):
            self.publish()
        self.gh_run.assert_not_called()


    def test_tag_moved_during_upload_keeps_draft_unpublished(self):
        tag_reads = 0
        def response(command, **kwargs):
            nonlocal tag_reads
            if "git/" in command[2]:
                tag_reads += 1
                if tag_reads == 2:
                    return json.dumps({"object": {"type": "commit", "sha": "b" * 40}})
            return self.response(command)
        self.api.side_effect = response
        with self.assertRaisesRegex(ValueError, "built source"):
            self.publish()
        self.assertEqual(self.gh_run.call_count, 1)
        self.assertFalse(any("--method" in c.args[0] for c in self.api.call_args_list))

    def test_lost_publication_response_never_deletes_or_retries(self):
        def response(command, **kwargs):
            if "--method" in command:
                raise subprocess.CalledProcessError(1, command)
            return self.response(command)
        self.api.side_effect = response
        with self.assertRaises(subprocess.CalledProcessError):
            self.publish()
        writes = [c.args[0] for c in self.api.call_args_list if "--method" in c.args[0]]
        self.assertEqual(len(writes), 1)
        self.assertIn("PATCH", writes[0])
        self.assertNotIn("DELETE", str(self.api.call_args_list))
        self.assertEqual(self.gh_run.call_count, 1)

    def test_changed_draft_identity_is_refused(self):
        self.api.side_effect = lambda c, **kw: self.response(c).replace('"target_commitish": "' + self.revision, '"target_commitish": "' + "b" * 40)
        with self.assertRaisesRegex(ValueError, "draft identity"):
            self.publish()
        self.assertFalse(any("--method" in c.args[0] for c in self.api.call_args_list))


if __name__ == "__main__":
    unittest.main()
