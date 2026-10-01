#!/usr/bin/env python3
"""Reuse successful Actions checks with verified source and task inputs."""

from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import time
import zipfile

import ci_plan as ci

VERSION = 1
MAX_AGE = timedelta(hours=24)
LIMIT = 20
# These jobs inspect external state or Git history and always run when selected.
FRESH = {"hygiene", "compose", "website"}
CONTROL_FILES = ("scripts/ci_plan.py", "scripts/ci_reuse.py")
NATIVE_ARTIFACTS = {f"oac-native-installer-{platform}" for platform in ("Linux-X64", "macOS-ARM64", "Windows-X64")}


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def tree(revision):
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError("Expected an immutable commit")
    try:
        raw = ci.git("ls-tree", "-rz", "--full-tree", revision)
    except subprocess.CalledProcessError:
        ci.git("fetch", "--no-tags", "--depth=2", "origin", revision)
        raw = ci.git("ls-tree", "-rz", "--full-tree", revision)
    return dict(entry.split("\t", 1)[::-1] for entry in raw.decode().split("\0") if entry)


def controls(files):
    return digest({path: value for path, value in files.items()
                   if path in CONTROL_FILES or path.startswith((".github/workflows/", ".github/actions/"))})


def fingerprints(files, runner_mode, image):
    inputs = {job: {} for job in ci.JOBS}
    for path, value in files.items():
        for job in ci.select([path])["jobs"]:
            inputs[job][path] = value
    return {job: digest({"version": VERSION, "files": values, "runner": runner_mode,
                         "image": image if job == "api" else False}) for job, values in inputs.items()}


def now():
    return datetime.now(timezone.utc)


def fresh(timestamp):
    if not isinstance(timestamp, str):
        raise ValueError("Evidence timestamp must be a string")
    parsed = datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ValueError("Evidence timestamp must have a timezone")
    age = now() - parsed
    return timedelta(0) <= age < MAX_AGE


def api(path, binary=False):
    result = subprocess.run(["gh", "api", path], check=True, capture_output=True, timeout=45)
    return result.stdout if binary else json.loads(result.stdout)


class Evidence:
    def __init__(self):
        self.repository = os.environ["GITHUB_REPOSITORY"]
        self.prefix = f"repos/{self.repository}/actions"
        self.revision = ci.git("rev-parse", "HEAD").decode().strip()
        self.files = tree(self.revision)
        self.control = controls(self.files)
        self.mode = os.environ.get("OAC_USE_GITHUB_RUNNERS", "") == "true"
        self.run_id = int(os.environ["GITHUB_RUN_ID"])
        self.attempt = int(os.environ["GITHUB_RUN_ATTEMPT"])
        self.release = os.environ.get("CI_NATIVE_ARTIFACTS") == "true"
        self.event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
        self.event_name = os.environ["GITHUB_EVENT_NAME"]
        self.artifacts_required = self.release or (self.event_name == "push" and self.event.get("ref") == "refs/heads/main")
        self.cache = {}

    def eligible(self, run):
        if (run["id"] == self.run_id or run.get("path") != ".github/workflows/check.yml"
                or (run.get("head_repository") or {}).get("full_name") != self.repository):
            return False
        if run.get("event") == "push" and run.get("head_branch") == "main":
            return True
        # PR evidence stays within the same repository PR. Main may promote an
        # identical tested tree, but release callers only read main evidence.
        if self.release or run.get("event") != "pull_request":
            return False
        if self.event_name == "pull_request":
            pr = self.event.get("pull_request", {})
            return (pr.get("head", {}).get("repo", {}).get("full_name") == self.repository
                    and run.get("head_branch") == pr.get("head", {}).get("ref"))
        return self.event_name == "push" and self.event.get("ref") == "refs/heads/main"

    def artifacts(self, run_id):
        return api(f"{self.prefix}/runs/{run_id}/artifacts?per_page=100")["artifacts"]

    def receipt(self, run):
        if not self.eligible(run) or run.get("conclusion") != "success" or run.get("status") != "completed":
            raise ValueError("Run is not an eligible success")
        if not fresh(run["created_at"]):
            raise ValueError("Run is too old")
        attempt = run["run_attempt"]
        name = f"ci-evidence-{attempt}"
        matches = [a for a in self.artifacts(run["id"]) if a["name"] == name and not a["expired"]]
        if len(matches) != 1 or matches[0]["size_in_bytes"] > 100_000:
            raise ValueError("Missing or invalid evidence artifact")
        raw = api(f"{self.prefix}/artifacts/{matches[0]['id']}/zip", binary=True)
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            info = archive.getinfo("evidence.json")
            if info.file_size > 100_000:
                raise ValueError("Oversized evidence")
            receipt = json.loads(archive.read(info))
        if (receipt["version"] != VERSION or receipt["run_id"] != run["id"]
                or receipt["attempt"] != attempt or receipt["repository"] != self.repository
                or receipt["runner"] != self.mode or type(receipt["image"]) is not bool):
            raise ValueError("Evidence identity mismatch")
        revision = receipt["revision"]
        files = tree(revision)
        # Verify the producer implementation, not a fingerprint claimed in an
        # artifact. PR workflow code must also match at the event's head SHA.
        if controls(files) != self.control or controls(tree(run["head_sha"])) != self.control:
            raise ValueError("Evidence producer changed")
        if run["event"] == "push":
            if revision != run["head_sha"]:
                raise ValueError("Push evidence has another checkout")
        else:
            parents = ci.git("show", "-s", "--format=%P", revision).decode().split()
            if len(parents) != 2 or parents[1] != run["head_sha"]:
                raise ValueError("PR evidence is not the tested merge")
            if self.event_name == "push" and files != self.files:
                raise ValueError("Main can only promote the identical PR tree")
        expected = fingerprints(files, self.mode, receipt["image"])
        if not isinstance(receipt["passed"], dict):
            raise ValueError("Evidence checks must be an object")
        for job, item in receipt["passed"].items():
            if (not isinstance(item, dict) or job not in expected
                    or item.get("key") != expected[job] or not fresh(item.get("verified_at"))):
                raise ValueError("Invalid or expired check evidence")
        return receipt

    def load(self, run_id):
        if type(run_id) is not int or run_id <= 0:
            raise ValueError("Invalid source run ID")
        if run_id not in self.cache:
            run = api(f"{self.prefix}/runs/{run_id}")
            self.cache[run_id] = self.receipt(run)
        return self.cache[run_id]

    def native_available(self, receipt):
        if receipt["revision"] != self.revision:
            return False
        run_id = receipt.get("native_run_id")
        if not run_id:
            return False
        run = api(f"{self.prefix}/runs/{run_id}")
        if (run.get("event") != "push" or run.get("head_branch") != "main"
                or run.get("head_sha") != self.revision or run.get("conclusion") != "success"
                or run.get("path") != ".github/workflows/check.yml"
                or (run.get("head_repository") or {}).get("full_name") != self.repository):
            return False
        names = {a["name"] for a in self.artifacts(run_id) if not a["expired"]}
        return NATIVE_ARTIFACTS <= names

    def matching(self, receipt, job, key):
        item = receipt["passed"].get(job)
        return (job not in FRESH and item is not None and item["key"] == key
                and fresh(item["verified_at"])
                and (job != "native" or not self.artifacts_required or self.native_available(receipt)))

    def candidates(self):
        runs = api(f"{self.prefix}/workflows/check.yml/runs?per_page={LIMIT}")["workflow_runs"]
        # A release waits for an existing main run of its exact source, bounded
        # to ten minutes. Failure/cancellation falls back to executing checks.
        pending = [r for r in runs if self.release and self.eligible(r)
                   and r["head_sha"] == self.revision and r["status"] != "completed"]
        if pending:
            run = pending[0]
            deadline = time.monotonic() + 600
            while run["status"] != "completed" and time.monotonic() < deadline:
                print(f"Waiting for main checks: {run['html_url']}", flush=True)
                time.sleep(20)
                run = api(f"{self.prefix}/runs/{run['id']}")
            runs = [run] + [r for r in runs if r["id"] != run["id"]]
        return runs

    def main_plan(self, plan, runs):
        if self.event_name != "push" or self.event.get("ref") != "refs/heads/main" or os.environ.get("REQUESTED_REF"):
            return plan
        # GitHub replaces pending runs even with cancel-in-progress=false. Use
        # the last verified main success so intermediate pushes remain covered.
        for run in runs:
            if run.get("event") != "push" or run.get("head_branch") != "main":
                continue
            try:
                receipt = self.receipt(run)
                ci.git("merge-base", "--is-ancestor", receipt["revision"], self.revision)
                pending = ci.select(ci.changed_paths(receipt["revision"], self.revision))
                return dict(plan, jobs=[j for j in ci.JOBS if j in set(plan["jobs"]) | set(pending["jobs"])],
                            image=plan["image"] or pending["image"],
                            reasons=plan["reasons"] + [f"Include changes since successful main run {run['id']}"] + pending["reasons"])
            except (subprocess.SubprocessError, ValueError, KeyError, TypeError, zipfile.BadZipFile):
                continue
        return ci.full("No verified main baseline; execute the full plan")

    def plan(self, plan):
        runs = []
        if os.environ.get("CI_FORCE") != "true":
            try:
                runs = self.candidates()
            except (subprocess.SubprocessError, ValueError, KeyError):
                print("Evidence lookup unavailable; executing selected checks.")
        plan = self.main_plan(plan, runs)
        keys = fingerprints(self.files, self.mode, plan["image"])
        result = dict(plan, execute=list(plan["jobs"]), reused={}, keys=keys)
        for run in runs:
            if not self.eligible(run) or run.get("conclusion") != "success":
                continue
            try:
                receipt = self.receipt(run)
                for job in list(result["execute"]):
                    if self.matching(receipt, job, keys[job]):
                        result["execute"].remove(job)
                        result["reused"][job] = {"run_id": run["id"], "key": keys[job]}
                        print(f"Reuse {job}: {run['html_url']}")
                if set(result["execute"]) <= FRESH:
                    break
            except (subprocess.SubprocessError, ValueError, KeyError, TypeError, zipfile.BadZipFile):
                continue
        return result

    def verify(self, plan):
        if plan["keys"] != fingerprints(self.files, self.mode, plan["image"]):
            raise ValueError("Plan inputs changed")
        for job, source in plan["reused"].items():
            if source["key"] != plan["keys"][job] or not self.matching(self.load(source["run_id"]), job, source["key"]):
                raise ValueError(f"Cannot verify reused check: {job}")

    def record(self, plan):
        passed = {job: {"key": plan["keys"][job], "verified_at": now().isoformat()}
                  for job in plan["execute"]}
        for job, source in plan["reused"].items():
            passed[job] = self.load(source["run_id"])["passed"][job]
        native_run = None
        if "native" in plan["execute"] and (self.release or self.event_name == "push" and self.event.get("ref") == "refs/heads/main"):
            native_run = self.run_id
        elif "native" in plan["reused"]:
            receipt = self.load(plan["reused"]["native"]["run_id"])
            if receipt["revision"] == self.revision:
                native_run = receipt.get("native_run_id")
        receipt = {"version": VERSION, "repository": self.repository, "run_id": self.run_id,
                   "attempt": self.attempt, "revision": self.revision, "runner": self.mode,
                   "image": plan["image"], "passed": passed, "native_run_id": native_run}
        directory = Path(os.environ["RUNNER_TEMP"]) / "ci-evidence"
        directory.mkdir(exist_ok=True)
        (directory / "evidence.json").write_text(json.dumps(receipt))
        with open(os.environ["GITHUB_OUTPUT"], "a") as output:
            output.write(f"native-run-id={native_run or ''}\n")
