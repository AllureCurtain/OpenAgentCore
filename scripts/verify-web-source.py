#!/usr/bin/env python3
"""Verify the recorded Agents Core Web import provenance."""

import base64
import hashlib
import json
import re
from pathlib import Path


root = Path(__file__).resolve().parent.parent
manifest_path = root / "provenance" / "web-source.json"
manifest = json.loads(manifest_path.read_text())
imported_files = json.loads((root / manifest["imported_files_manifest"]).read_text())
errors: list[str] = []

if imported_files["source_commit"] != manifest["source_commit"]:
    errors.append("Web imported-files manifest uses a different source commit")

patch_path = root / manifest["working_tree_patch"]
if not patch_path.is_file():
    errors.append(f"missing recorded patch: {patch_path.relative_to(root)}")
else:
    encoded_patch = patch_path.read_bytes()
    if manifest.get("working_tree_patch_encoding") != "base64":
        errors.append("unsupported Web working-tree patch encoding")
        patch_bytes = b""
    else:
        try:
            patch_bytes = base64.b64decode(encoded_patch, validate=False)
        except ValueError:
            errors.append("recorded Web working-tree patch is not valid base64")
            patch_bytes = b""
    actual_digest = hashlib.sha256(patch_bytes).hexdigest()
    if actual_digest != manifest["working_tree_diff_sha256"]:
        errors.append("recorded Web working-tree patch digest does not match web-source.json")

    patch_files = []
    for source, destination in re.findall(
        rb"^diff --git a/(.+?) b/(.+?)$", patch_bytes, re.MULTILINE
    ):
        source_name = source.decode("utf-8")
        destination_name = destination.decode("utf-8")
        if source_name != destination_name:
            errors.append(f"unexpected renamed patch path: {source_name} -> {destination_name}")
        patch_files.append(source_name)
    if sorted(patch_files) != sorted(manifest["working_tree_diff_files"]):
        errors.append("recorded patch paths do not match working_tree_diff_files")

for entry in manifest["copied_paths"]:
    destination = root / entry["destination"]
    if not destination.exists():
        errors.append(f"missing imported destination: {entry['destination']}")

adapted_destinations = set(manifest["integration_adaptation_files"])
adapted_hashes = manifest["integration_adaptation_sha256"]
removed_sources = set(manifest["removed_source_files"])
recorded_destinations = {entry["destination"] for entry in imported_files["files"]}
if set(adapted_hashes) != adapted_destinations:
    errors.append("adaptation hash keys do not match integration_adaptation_files")
unknown_adaptations = adapted_destinations - recorded_destinations
if unknown_adaptations:
    errors.append(
        "adaptation files are absent from the imported snapshot: "
        + ", ".join(sorted(unknown_adaptations))
    )

recorded_sources = {entry["source"] for entry in imported_files["files"]}
unknown_removals = removed_sources - recorded_sources
if unknown_removals:
    errors.append(
        "removed files are absent from the imported snapshot: "
        + ", ".join(sorted(unknown_removals))
    )

for entry in imported_files["files"]:
    source_name = entry["source"]
    destination_name = entry["destination"]
    destination = root / destination_name
    if source_name in removed_sources:
        if destination.exists():
            errors.append(f"removed source unexpectedly imported: {destination_name}")
        continue
    if not destination.is_file():
        errors.append(f"missing imported file: {destination_name}")
        continue
    actual_digest = hashlib.sha256(destination.read_bytes()).hexdigest()
    if destination_name in adapted_destinations:
        if actual_digest != adapted_hashes.get(destination_name):
            errors.append(f"recorded post-import adaptation changed: {destination_name}")
        continue
    if actual_digest != entry["sha256"]:
        errors.append(f"unrecorded post-import change: {destination_name}")

for source_name, expected_digest in manifest["untracked_files"].items():
    destination_name = source_name
    if source_name.startswith("docs/"):
        destination_name = "docs/web/" + source_name.removeprefix("docs/")
    destination = root / destination_name
    if not destination.is_file():
        errors.append(f"missing formerly untracked import: {destination_name}")
        continue
    actual_digest = hashlib.sha256(destination.read_bytes()).hexdigest()
    if actual_digest != expected_digest:
        errors.append(f"formerly untracked import changed: {destination_name}")

if errors:
    raise SystemExit("\n".join(errors))

print(
    "Web import provenance verified: "
    f"{len(imported_files['files'])} source files, "
    f"{len(manifest['integration_adaptation_files'])} adaptations, "
    f"{len(manifest['working_tree_diff_files'])} patched files, "
    f"{len(manifest['untracked_files'])} formerly untracked files"
)
