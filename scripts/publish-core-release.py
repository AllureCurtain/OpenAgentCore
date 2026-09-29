#!/usr/bin/env python3
"""Upload a draft, verify its source, then publish once without cleanup on failure."""

import argparse
import importlib.util
import json
import os
import pathlib
import re
import subprocess
from urllib.parse import quote

spec = importlib.util.spec_from_file_location(
    "distribution", pathlib.Path(__file__).with_name("core-distribution-manifest.py"))
distribution = importlib.util.module_from_spec(spec)
spec.loader.exec_module(distribution)


def api(repository, endpoint, *args):
    return json.loads(subprocess.check_output(
        ["gh", "api", "repos/" + repository + "/" + endpoint, *args], text=True))


def verify_tag(repository, tag, revision):
    endpoint = "git/ref/tags/" + quote(tag, safe="")
    for _ in range(10):
        obj = api(repository, endpoint)["object"]
        if obj["type"] == "commit":
            if obj["sha"] != revision:
                raise ValueError("Version tag no longer points to the built source")
            return
        if obj["type"] != "tag":
            raise ValueError("Version tag does not resolve to a commit")
        endpoint = "git/tags/" + obj["sha"]
    raise ValueError("Too many nested annotated tags")


def publish(assets, repository, revision, tag, mode):
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise ValueError("Expected an owner/repository")
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError("Expected a full source commit SHA")
    if mode not in ("publish", "draft"):
        raise ValueError("Expected publish or draft mode")
    if mode == "publish":
        if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?", tag):
            raise ValueError("Version tags must use vMAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]")
    elif tag != "build-" + revision:
        raise ValueError("Manual builds use build-<full source SHA>")

    files = sorted(assets.iterdir())
    if not files or any(p.is_symlink() or not p.is_file() for p in files):
        raise ValueError("Expected a nonempty directory containing only regular asset files")
    # Verify the archives after transfer between jobs. Their manifests bind the
    # remaining Runtime assets; the builder validates that contract.
    stem = "oac-" + revision + "-linux-amd64"
    archives = [assets / (stem + ".tar.gz")]
    if mode == "publish" or (assets / (stem + "-offline.tar.gz")).exists():
        archives.append(assets / (stem + "-offline.tar.gz"))
    for archive in archives:
        checksum = archive.with_name(archive.name + ".sha256")
        if checksum.read_text() != distribution.sha256(archive) + "  " + archive.name + "\n":
            raise ValueError("Distribution archive checksum mismatch")

    command = ["gh", "release", "create", tag, "--repo", repository,
               "--target", revision, "--title", "OpenAgentCore " + tag,
               "--notes", "Linux amd64 distribution from commit " + revision + "."]
    if mode == "publish":
        verify_tag(repository, tag, revision)
        command.append("--verify-tag")
        if "-" in tag.split("+", 1)[0]:
            command.append("--prerelease")
    # Explicit drafts keep gh's error cleanup away from the publication request.
    # Existing releases fail creation; do not overwrite or replay an ambiguous write.
    command.append("--draft")
    subprocess.run(command + [str(p.resolve()) for p in files], check=True)
    if mode == "draft":
        return

    release = api(repository, "releases/tags/" + quote(tag, safe=""))
    if (not release["draft"] or release["tag_name"] != tag
            or release["target_commitish"] != revision):
        raise ValueError("Release draft identity changed")
    release_id = release["id"]
    if type(release_id) is not int or release_id <= 0:
        raise ValueError("Invalid Release ID")
    # Uploads can take minutes. Recheck the tag immediately before publishing the
    # fixed Release ID, so a moved tag cannot silently relabel this build.
    verify_tag(repository, tag, revision)
    result = api(repository, "releases/" + str(release_id),
                 "--method", "PATCH", "-F", "draft=false")
    if result["id"] != release_id or result["draft"] or result["tag_name"] != tag:
        raise ValueError("Publication result is unknown; inspect the existing Release")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--assets", required=True, type=pathlib.Path)
    args = parser.parse_args()
    publish(args.assets, os.environ["GH_REPO"], os.environ["RELEASE_REVISION"],
            os.environ["RELEASE_TAG"], os.environ["RELEASE_MODE"])
