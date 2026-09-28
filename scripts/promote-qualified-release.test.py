"""Controller refusal tests; these fixtures never count as live qualification."""

import hashlib
import importlib.util
import io
import json
import pathlib
import subprocess
import tarfile
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location(
    "promotion", pathlib.Path(__file__).with_name("promote-qualified-release.py"))
promotion = importlib.util.module_from_spec(spec)
spec.loader.exec_module(promotion)


class PromotionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        self.assets = self.root / "assets"
        self.assets.mkdir()
        self.tree = "b" * 40
        self.metadata = {
            "source_commit": promotion.SOURCE, "source_tree": self.tree,
            "artifact_base_url": promotion.BASE, "platform": "linux/amd64",
            "images": {name: "sha256:" + "a" * 64 for name in ("core", "web", "runtime", "database")},
            "image_manifest_digests": {name: "sha256:" + "b" * 64 for name in ("core", "web", "runtime", "database")},
            "runtime_ref": "oac-runtime@sha256:" + "c" * 64, "artifacts": {},
        }
        for logical, suffix in promotion.distribution.ARTIFACTS.items():
            name = promotion.STEM + "-" + suffix
            (self.assets / name).write_bytes(logical.encode())
            self.metadata["artifacts"][logical] = dict(promotion.file_identity(self.assets / name), filename=name)
        self.archives()

    def archives(self):
        for offline in (False, True):
            data = {"manifest.json": json.dumps(self.metadata).encode(), "source.tar.gz": b"source fixture"}
            data["SHA256SUMS"] = "".join(
                hashlib.sha256(value).hexdigest() + "  " + key + "\n" for key, value in data.items()).encode()
            if offline:
                data.update({"artifacts/" + entry["filename"]: (self.assets / entry["filename"]).read_bytes()
                             for entry in self.metadata["artifacts"].values()})
            name = promotion.STEM + ("-offline" if offline else "") + ".tar.gz"
            with tarfile.open(self.assets / name, "w:gz") as archive:
                for key, value in data.items():
                    entry = tarfile.TarInfo(promotion.STEM + "/" + key)
                    entry.size = len(value)
                    archive.addfile(entry, io.BytesIO(value))
            checksum = promotion.file_identity(self.assets / name)["sha256"]
            (self.assets / (name + ".sha256")).write_text(checksum + "  " + name + "\n")

    def test_full_asset_inventory_and_archive_contents(self):
        metadata, inventory = promotion.inspect_candidate(self.assets)
        self.assertEqual(metadata, self.metadata)
        promotion.verify_files(self.assets, inventory)
        name = next(iter(self.metadata["artifacts"].values()))["filename"]
        (self.assets / name).write_bytes(b"changed")
        with self.assertRaisesRegex(ValueError, "checksum/size"):
            promotion.inspect_candidate(self.assets)
        with self.assertRaisesRegex(ValueError, "bytes/set"):
            promotion.verify_files(self.assets, inventory)

    def test_source_and_url_mismatch(self):
        for key in ("source_commit", "artifact_base_url"):
            with self.subTest(key=key):
                old = self.metadata[key]
                self.metadata[key] = "wrong"
                self.archives()
                with self.assertRaisesRegex(ValueError, "source/platform/release URL"):
                    promotion.inspect_candidate(self.assets)
                self.metadata[key] = old

    def test_missing_or_extra_asset_rejected(self):
        extra = self.assets / "pass.json"
        extra.write_text('{"passed":true}')
        with self.assertRaisesRegex(ValueError, "asset set mismatch"):
            promotion.inspect_candidate(self.assets)
        extra.unlink()
        name = next(iter(self.metadata["artifacts"].values()))["filename"]
        (self.assets / name).unlink()
        with self.assertRaisesRegex(ValueError, "checksum/size"):
            promotion.inspect_candidate(self.assets)

    def test_missing_manifest_artifact_rejected(self):
        self.metadata["artifacts"].pop(next(iter(self.metadata["artifacts"])))
        self.archives()
        with self.assertRaisesRegex(ValueError, "Missing or unexpected"):
            promotion.inspect_candidate(self.assets)

    def test_unlanded_and_unrelated_main_changes_rejected(self):
        for comparison in ({"status": "diverged", "files": []},
                           {"status": "ahead", "files": [{"filename": "services/agents-api/main.go"}]},
                           {"status": "ahead", "files": [{"filename": "Makefile", "previous_filename": "go.mod"}]}):
            with self.subTest(comparison=comparison), mock.patch.object(
                    promotion, "api", side_effect=[{"commit": {"tree": {"sha": self.tree}}}, comparison]):
                with self.assertRaises(ValueError):
                    promotion.verify_landed(self.tree, "d" * 40)

    def test_landed_source_keeps_original_identity(self):
        with mock.patch.object(promotion, "api", side_effect=[
            {"commit": {"tree": {"sha": self.tree}}},
            {"status": "ahead", "files": [{"filename": "scripts/promote-qualified-release.py"}]},
            {"commit": {"tree": {"sha": "e" * 40}}},
            {"sha": "f" * 40, "commit": {"tree": {"sha": "e" * 40}}},
            {"status": "ahead"},
        ]):
            promotion.verify_landed(self.tree, "d" * 40)

    def test_conflicting_tag_rejected(self):
        with mock.patch.object(promotion, "api", return_value=[{
            "ref": "refs/tags/" + promotion.TAG, "object": {"type": "commit", "sha": "f" * 40},
        }]):
            with self.assertRaisesRegex(ValueError, "Conflicting"):
                promotion.verify_tag()

    def test_main_tree_mismatch_blocks_old_candidate(self):
        with mock.patch.object(promotion, "api", side_effect=[
            {"commit": {"tree": {"sha": self.tree}}}, {"status": "ahead", "files": []},
            {"commit": {"tree": {"sha": "d" * 40}}},
            {"sha": "e" * 40, "commit": {"tree": {"sha": "f" * 40}}},
        ]):
            with self.assertRaisesRegex(ValueError, "Main tree differs"):
                promotion.verify_landed(self.tree, "a" * 40)

    def test_makefile_allowance_cannot_hide_build_changes(self):
        encode = lambda value: promotion.base64.b64encode(value.encode()).decode()
        anchor = "\tPYTHONDONTWRITEBYTECODE=1 python3 scripts/core-distribution-manifest.test.py\n"
        with mock.patch.object(promotion, "api", side_effect=[
            {"commit": {"tree": {"sha": self.tree}}},
            {"status": "ahead", "files": [{"filename": "Makefile"}]},
            {"content": encode(anchor)}, {"content": encode(anchor + "build:\n\techo changed\n")},
        ]):
            with self.assertRaisesRegex(ValueError, "exceeds promotion test"):
                promotion.verify_landed(self.tree, "a" * 40)

    def test_adapter_not_ready_has_no_remote_side_effects(self):
        with mock.patch.object(promotion, "gh") as gh, mock.patch.object(promotion, "api") as api:
            with self.assertRaisesRegex(ValueError, "not connected"):
                promotion.promote(self.assets, self.root / "state", "mx2", "/tmp/acceptance", "d" * 40)
            gh.assert_not_called()
            api.assert_not_called()
            self.assertFalse((self.root / "state").exists())

    def qualify(self, checks=None, failure=False, wrong_identity=False):
        adapter = self.root / "adapter.py"
        adapter.write_text("# Transport test fixture, never live acceptance\n")
        request = {"source": promotion.SOURCE, "tree": self.tree, "run_id": "test-run",
                   "inventory_sha256": "c" * 64, "directory": "/tmp/test", "inventory": {}}

        def transport(argv, **kwargs):
            if "exec(compile" in argv[-1]:
                if failure:
                    raise subprocess.CalledProcessError(1, argv)
                result = {key: request[key] for key in (
                    "source", "tree", "run_id", "inventory_sha256", "adapter_sha256")}
                result["checks"] = checks if checks is not None else {name: "passed" for name in promotion.REQUIRED_CHECKS}
                if wrong_identity:
                    result["run_id"] = "previous-run"
                return json.dumps(result)
            return ""

        with mock.patch.object(promotion, "run", side_effect=transport):
            return promotion.qualification(request, "mx2", "/tmp/test", adapter, self.assets)

    def test_nonzero_ssh_missing_checks_and_replayed_result_rejected(self):
        with self.assertRaises(subprocess.CalledProcessError):
            self.qualify(failure=True)
        with self.assertRaisesRegex(ValueError, "missing, failed or skipped"):
            self.qualify(checks={"fresh-install": "passed"})
        with self.assertRaisesRegex(ValueError, "identity mismatch"):
            self.qualify(wrong_identity=True)

    def test_complete_transport_result(self):
        self.assertEqual(self.qualify()["checks"], {name: "passed" for name in promotion.REQUIRED_CHECKS})

    def test_publication_waits_for_live_result_and_merge(self):
        for failure in ("qualification", "verify_landed"):
            with self.subTest(failure=failure):
                state = self.root / failure
                draft = {"id": 1, "draft": True, "target_commitish": promotion.SOURCE}
                ready = json.dumps({"ready": True, "required_checks": list(promotion.REQUIRED_CHECKS)})
                with mock.patch.object(promotion, "run", return_value=ready), \
                     mock.patch.object(promotion, "verify_tooling"), \
                     mock.patch.object(promotion, "api", return_value={"commit": {"tree": {"sha": self.tree}}}), \
                     mock.patch.object(promotion, "verify_tag"), \
                     mock.patch.object(promotion, "release_state", return_value=draft), \
                     mock.patch.object(promotion, "download"), \
                     mock.patch.object(promotion, "verify_files"), \
                     mock.patch.object(promotion, "qualification", return_value={}) as qualify, \
                     mock.patch.object(promotion, "verify_landed") as landed, \
                     mock.patch.object(promotion, "gh") as gh:
                    (qualify if failure == "qualification" else landed).side_effect = ValueError("blocked")
                    with self.assertRaisesRegex(ValueError, "blocked"):
                        promotion.promote(self.assets, state, "mx2", "/tmp/acceptance", "d" * 40)
                    gh.assert_not_called()

    def test_success_publishes_only_after_final_download_verification(self):
        draft = {"id": 1, "draft": True, "target_commitish": promotion.SOURCE}
        final = dict(draft, draft=False, prerelease=False, html_url="https://example.invalid/release")
        ready = json.dumps({"ready": True, "required_checks": list(promotion.REQUIRED_CHECKS)})
        events = []

        def download(path, inventory):
            events.append(path.name)

        with mock.patch.object(promotion, "run", return_value=ready), \
                     mock.patch.object(promotion, "verify_tooling"), \
             mock.patch.object(promotion, "api", return_value={"commit": {"tree": {"sha": self.tree}}}), \
             mock.patch.object(promotion, "verify_tag"), \
             mock.patch.object(promotion, "release_state", side_effect=[draft, draft, draft, final]), \
             mock.patch.object(promotion, "download", side_effect=download), \
             mock.patch.object(promotion, "verify_files"), \
             mock.patch.object(promotion, "qualification", side_effect=lambda *args: events.append("qualified") or {}), \
             mock.patch.object(promotion, "verify_landed", side_effect=lambda *args: events.append("landed")), \
             mock.patch.object(promotion, "gh", side_effect=lambda *args: events.append(args[:3])), \
             mock.patch("builtins.print"):
            promotion.promote(self.assets, self.root / "success", "mx2", "/tmp/acceptance", "d" * 40)
        self.assertEqual(events, ["downloaded", "qualified", "landed", "before-publication",
                                  ("release", "edit", promotion.TAG), "published"])

    def test_download_hashes_stream_without_storing_an_archive(self):
        payload = b"downloaded bytes"
        inventory = {"asset.tar.gz": {"sha256": hashlib.sha256(payload).hexdigest(), "size": len(payload)}}
        class Process:
            def __init__(self, *args, **kwargs):
                self.stdout = io.BytesIO(payload)
            def __enter__(self):
                return self
            def __exit__(self, *args):
                self.stdout.close()
            def wait(self):
                return 0
        with mock.patch.object(promotion, "release_state", return_value={"id": 1}), \
             mock.patch.object(promotion, "run", return_value='[[{"name":"asset.tar.gz"}]]'), \
             mock.patch.object(promotion.subprocess, "Popen", Process):
            promotion.download(self.root / "streamed", inventory)
            self.assertTrue((self.root / "streamed.json").is_file())
            self.assertFalse((self.root / "streamed").exists())
            inventory["asset.tar.gz"]["sha256"] = "0" * 64
            with self.assertRaisesRegex(ValueError, "bytes changed"):
                promotion.download(self.root / "bad-stream", inventory)
            self.assertFalse((self.root / "bad-stream.json").exists())

    def test_recorded_pass_cannot_resume_an_interrupted_run(self):
        state = self.root / "previous-run"
        state.mkdir()
        (state / "qualification.json").write_text('{"passed":true}')
        ready = json.dumps({"ready": True, "required_checks": list(promotion.REQUIRED_CHECKS)})
        with mock.patch.object(promotion, "run", return_value=ready), \
                     mock.patch.object(promotion, "verify_tooling"), mock.patch.object(promotion, "gh") as gh:
            with self.assertRaises(FileExistsError):
                promotion.promote(self.assets, state, "mx2", "/tmp/acceptance", "d" * 40)
            gh.assert_not_called()


if __name__ == "__main__":
    unittest.main()
