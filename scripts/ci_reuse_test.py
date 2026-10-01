"""Input invalidation, provenance, gate and workflow regression tests."""
from datetime import timedelta
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import ci_plan as ci
import ci_reuse as reuse


class FingerprintTests(unittest.TestCase):
    def setUp(self):
        self.files = {"services/core/main.go": "100644 blob code", "docs/guide.md": "100644 blob docs",
                      "apps/web/src/app.tsx": "100644 blob web", "Makefile": "100644 blob make",
                      "scripts/ci_reuse.py": "100644 blob reuse", ".github/workflows/check.yml": "100644 blob workflow"}

    def keys(self, files=None, mode=False, image=False):
        return reuse.fingerprints(self.files if files is None else files, mode, image)

    def test_docs_followup_keeps_code_evidence(self):
        changed = self.files | {"docs/guide.md": "100644 blob new-docs"}
        a, b = self.keys(), self.keys(changed)
        self.assertNotEqual(a["hygiene"], b["hygiene"])
        self.assertNotEqual(a["website"], b["website"])
        for job in ("backend", "api", "web", "web-acceptance", "native"):
            self.assertEqual(a[job], b[job], job)

    def test_source_addition_deletion_mode_and_shared_inputs_invalidate(self):
        for change in ({"services/core/main.go": "100644 blob new-code"},
                       {"services/core/main.go": "100755 blob code"},
                       {"services/core/new.go": "100644 blob code"}):
            self.assertNotEqual(self.keys()["backend"], self.keys(self.files | change)["backend"])
        self.assertNotEqual(self.keys()["backend"], self.keys({k:v for k,v in self.files.items() if k != "services/core/main.go"})["backend"])
        for path in ("Makefile", "scripts/ci_reuse.py", ".github/workflows/check.yml"):
            self.assertTrue(all(self.keys()[j] != self.keys(self.files | {path: "new"})[j] for j in ci.JOBS))

    def test_runner_and_image_modes_are_part_of_evidence(self):
        self.assertTrue(all(self.keys()[j] != self.keys(mode=True)[j] for j in ci.JOBS))
        self.assertNotEqual(self.keys()["api"], self.keys(image=True)["api"])
        self.assertEqual(self.keys()["backend"], self.keys(image=True)["backend"])

    def test_unrelated_node_module_does_not_invalidate_backend(self):
        self.assertEqual(self.keys()["backend"], self.keys(self.files | {"apps/web/pnpm-lock.yaml": "new"})["backend"])

    def test_empty_diff_is_not_full_and_lint_config_is_checked(self):
        self.assertEqual(ci.select([])["jobs"], ["hygiene"])
        self.assertEqual(set(ci.select([".github/actionlint.yaml"])["jobs"]), {"hygiene", "lint"})


class EvidenceTests(unittest.TestCase):
    def setUp(self):
        self.e = object.__new__(reuse.Evidence)
        self.e.repository = "owner/repo"
        self.e.prefix = "repos/owner/repo/actions"
        self.e.revision = "a" * 40
        self.e.files = {"scripts/ci_reuse.py": "reuse", ".github/workflows/check.yml": "workflow", "services/core/code.go": "code"}
        self.e.control = reuse.controls(self.e.files)
        self.e.run_id = 99
        self.e.attempt = 1
        self.e.mode = False
        self.e.release = False
        self.e.artifacts_required = False
        self.e.event_name = "pull_request"
        self.e.event = {"pull_request": {"head": {"ref": "feature", "repo": {"full_name": "owner/repo"}}}}
        self.e.cache = {}
        self.run = {"id": 12, "run_attempt": 1, "path": ".github/workflows/check.yml", "event": "push",
                    "head_branch": "main", "head_sha": "a" * 40, "head_repository": {"full_name": "owner/repo"},
                    "created_at": reuse.now().isoformat(), "status": "completed", "conclusion": "success", "html_url": "https://example.invalid/12"}
        self.keys = reuse.fingerprints(self.e.files, False, False)
        self.receipt = {"version": reuse.VERSION, "repository": "owner/repo", "run_id": 12, "attempt": 1,
                        "revision": "a" * 40, "runner": False, "image": False, "native_run_id": 12,
                        "passed": {"backend": {"key": self.keys["backend"], "verified_at": reuse.now().isoformat()}}}

    def read(self, receipt=None, run=None, files=None):
        archive = io.BytesIO()
        with zipfile.ZipFile(archive, "w") as z:
            z.writestr("evidence.json", json.dumps(self.receipt if receipt is None else receipt))
        artifacts = [{"name": "ci-evidence-1", "expired": False, "size_in_bytes": 100, "id": 1}]
        with patch.object(self.e, "artifacts", return_value=artifacts), patch.object(reuse, "api", return_value=archive.getvalue()), patch.object(reuse, "tree", return_value=self.e.files if files is None else files):
            return self.e.receipt(self.run if run is None else run)

    def test_success_and_producer_are_verified(self):
        self.assertEqual(self.read(), self.receipt)
        for fields in ({"conclusion": "failure"}, {"conclusion": "cancelled"}, {"status": "in_progress"},
                       {"path": ".github/workflows/untrusted.yml"}, {"head_repository": {"full_name": "fork/repo"}},
                       {"id": 99}, {"created_at": (reuse.now() - timedelta(days=2)).isoformat()},
                       {"run_attempt": 2}, {"head_sha": "b" * 40}):
            with self.subTest(fields=fields), self.assertRaises(ValueError):
                self.read(run=self.run | fields)
        with self.assertRaises(ValueError):
            self.read(files=self.e.files | {"scripts/ci_reuse.py": "forged producer"})

    def test_forged_stale_and_wrong_mode_receipts_are_rejected(self):
        for fields in ({"run_id": 13}, {"attempt": 2}, {"repository": "fork/repo"}, {"runner": True},
                       {"image": "false"}, {"passed": {"backend": {"key": "forged", "verified_at": reuse.now().isoformat()}}},
                       {"passed": {"backend": {"key": self.keys["backend"], "verified_at": (reuse.now() - timedelta(days=2)).isoformat()}}}):
            with self.subTest(fields=fields), self.assertRaises(ValueError):
                self.read(receipt=self.receipt | fields)

    def test_pr_scope_and_release_trust(self):
        pr = self.run | {"event": "pull_request", "head_branch": "feature"}
        self.assertTrue(self.e.eligible(pr))
        self.assertFalse(self.e.eligible(pr | {"head_branch": "another-pr"}))
        self.e.release = True
        self.assertFalse(self.e.eligible(pr))
        self.assertTrue(self.e.eligible(self.run))

    def test_pr_merge_parent_and_main_tree_are_verified(self):
        pr = self.run | {"event": "pull_request", "head_branch": "feature", "head_sha": "b" * 40}
        with patch.object(ci, "git", return_value=("c"*40 + " " + "b"*40).encode()):
            self.assertEqual(self.read(run=pr), self.receipt)
        with patch.object(ci, "git", return_value=b"wrong parents"), self.assertRaises(ValueError):
            self.read(run=pr)
        self.e.event_name = "push"
        self.e.event = {"ref": "refs/heads/main"}
        with patch.object(ci, "git", return_value=("c"*40 + " " + "b"*40).encode()), self.assertRaises(ValueError):
            self.read(run=pr, files=self.e.files | {"docs/new.md": "changed"})

    def test_native_artifacts_require_exact_revision_and_trusted_source(self):
        artifacts = [{"name": name, "expired": False} for name in reuse.NATIVE_ARTIFACTS]
        with patch.object(reuse, "api", return_value=self.run), patch.object(self.e, "artifacts", return_value=artifacts):
            self.assertTrue(self.e.native_available(self.receipt))
            self.assertFalse(self.e.native_available(self.receipt | {"revision": "b" * 40}))
        with patch.object(reuse, "api", return_value=self.run), patch.object(self.e, "artifacts", return_value=artifacts[:-1]):
            self.assertFalse(self.e.native_available(self.receipt))
        with patch.object(reuse, "api", return_value=self.run | {"event": "pull_request"}):
            self.assertFalse(self.e.native_available(self.receipt))

    def test_lookup_failure_and_force_execute_checks(self):
        plan = ci.select(["services/core/code.go"])
        with patch.object(self.e, "candidates", side_effect=ValueError("unavailable")):
            self.assertEqual(self.e.plan(plan)["execute"], plan["jobs"])
        with patch.dict(os.environ, {"CI_FORCE": "true"}), patch.object(self.e, "candidates") as candidates:
            self.assertEqual(self.e.plan(plan)["execute"], plan["jobs"])
            candidates.assert_not_called()

    def test_matching_result_reuses_only_selected_jobs_and_gate_rechecks_it(self):
        plan = ci.select(["services/core/code.go"])
        with patch.object(self.e, "candidates", return_value=[self.run]), patch.object(self.e, "receipt", return_value=self.receipt):
            result = self.e.plan(plan)
        self.assertEqual(result["execute"], ["hygiene", "api"])
        self.assertEqual(set(result["reused"]), {"backend"})
        needs = {j: {"result": "success" if j in result["execute"] else "skipped"} for j in ci.JOBS}
        needs["plan"] = {"result": "success"}
        ci.check_results(result, needs)
        with patch.object(self.e, "load", return_value=self.receipt):
            self.e.verify(result)
        with patch.object(self.e, "load", return_value=self.receipt | {"passed": {}}), self.assertRaises(ValueError):
            self.e.verify(result)
        for bad in (result | {"execute": ["hygiene"]}, result | {"reused": {}}, result | {"execute": plan["jobs"]}):
            with self.assertRaises(ValueError):
                ci.check_results(bad, needs)
        with self.assertRaises(ValueError):
            ci.check_results(result, needs | {"api": {"result": "skipped"}})

    def test_reuse_does_not_renew_expiry(self):
        verified = (reuse.now() - timedelta(hours=20)).isoformat()
        self.receipt["passed"]["backend"]["verified_at"] = verified
        plan = {"keys": self.keys, "execute": ["hygiene"], "reused": {"backend": {"run_id": 12}}, "image": False}
        with tempfile.TemporaryDirectory() as tmp, patch.dict(os.environ, {"RUNNER_TEMP": tmp, "GITHUB_OUTPUT": tmp + "/output"}), patch.object(self.e, "load", return_value=self.receipt):
            self.e.record(plan)
            recorded = json.loads((Path(tmp) / "ci-evidence/evidence.json").read_text())
        self.assertEqual(recorded["passed"]["backend"]["verified_at"], verified)

    def test_release_waits_for_exact_main_run_and_failure_falls_back(self):
        self.e.release = True
        pending = self.run | {"status": "in_progress", "conclusion": None}
        with patch.object(reuse, "api", side_effect=[{"workflow_runs": [pending]}, self.run]), patch.object(reuse.time, "sleep") as sleep:
            self.assertEqual(self.e.candidates(), [self.run])
            sleep.assert_called_once_with(20)
        failed = self.run | {"conclusion": "failure"}
        with patch.object(reuse, "api", side_effect=[{"workflow_runs": [pending]}, failed]), patch.object(reuse.time, "sleep"):
            self.assertEqual(self.e.plan(ci.full("release"))["execute"], list(ci.JOBS))


class WorkflowTests(unittest.TestCase):
    def test_site_has_one_trigger_owner_and_build_owner(self):
        root = Path(__file__).resolve().parents[1]
        check = (root / ".github/workflows/check.yml").read_text()
        site = (root / ".github/workflows/website.yml").read_text()
        self.assertIn("uses: ./.github/workflows/website.yml", check)
        self.assertNotIn("make check-website", check)
        self.assertIn("  workflow_call:", site)
        self.assertNotIn("  pull_request:", site)
        self.assertNotIn("  push:", site)

    def test_browser_suite_can_follow_a_reused_unit_result(self):
        root = Path(__file__).resolve().parents[1]
        body = (root / ".github/workflows/check.yml").read_text().split("  web-acceptance:\n")[1].split("    strategy:")[0]
        self.assertIn("always() && !cancelled()", body)
        self.assertIn("reused.web != null", body)


if __name__ == "__main__":
    unittest.main()
