"""Prepare one installation's data directory. Release identity comes from the environment."""
import base64
import fcntl
import hashlib
import json
import os
from pathlib import Path
import secrets
import tarfile
import urllib.request
import uuid

MEMBERS = ("manifest.json", "SHA256SUMS", "node-install.pyz", "runtime/seccomp.json", "standard-sizes.json")
OWNERS = {"database": 70, "secrets": 65532, "state": 65532, "caddy": 65532, "domain": 65532, "node-payload": 65532}


def required(name):
    value = os.environ.get(name, "")
    if not value:
        raise RuntimeError(name + " is required")
    return value


def digest(data):
    return hashlib.sha256(data).hexdigest()


def download():
    revision = required("OAC_REVISION")
    base = required("OAC_RELEASE_BASE")
    checksum_expected = required("OAC_ARCHIVE_CHECKSUM")
    archive_name = "oac-" + revision + "-linux-amd64"
    print("Downloading and verifying the matched node installation metadata", flush=True)
    checksum = hashlib.sha256()
    with urllib.request.urlopen(base + archive_name + ".tar.gz", timeout=60) as response:
        class Reader:
            def read(self, count=-1):
                data = response.read(count)
                checksum.update(data)
                return data
        reader, files = Reader(), {}
        with tarfile.open(fileobj=reader, mode="r|gz") as archive:
            for member in archive:
                name = member.name.removeprefix(archive_name + "/")
                if member.name == archive_name + "/" + name and name in MEMBERS:
                    if name in files or not member.isfile() or member.size > 1024 * 1024:
                        raise RuntimeError("Invalid release metadata member")
                    files[name] = archive.extractfile(member).read()
        while reader.read(1024 * 1024):
            pass
    if checksum.hexdigest() != checksum_expected or set(files) != set(MEMBERS):
        raise RuntimeError("Release metadata checksum mismatch")
    manifest = json.loads(files["manifest.json"])
    if manifest["source_commit"] != revision or manifest["platform"] != "linux/amd64":
        raise RuntimeError("Release identity mismatch")
    return files


def write(path, data, owner=65532):
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_bytes(data)
    temporary.chmod(0o600)
    os.chown(temporary, owner, owner)
    os.replace(temporary, path)


def initialize(root, fetch=download):
    revision = required("OAC_REVISION")
    root = Path(root)
    root.chmod(0o755)
    for name, owner in OWNERS.items():
        directory = root / name
        directory.mkdir(exist_ok=True)
        directory.chmod(0o755 if name in ("caddy", "domain") else 0o700)
        os.chown(directory, owner, owner)
    site = root / "caddy" / "site.caddy"
    if not site.exists():
        write(site, b"\n")
    for name in ("core", "web", "database"):
        directory = root / "secrets" / name
        directory.mkdir(exist_ok=True)
        directory.chmod(0o700)
        os.chown(directory, 65532, 65532)
    with (root / "secrets/.init.lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        marker = root / "installation.json"
        if marker.exists():
            receipt = json.loads(marker.read_text())
            if receipt["source_commit"] != revision:
                raise RuntimeError("This data directory belongs to another release; create a new installation")
            for name, checksum in receipt["files"].items():
                if digest((root / name).read_bytes()) != checksum:
                    raise RuntimeError("Installation files changed; restore the matching data directory")
            print("Existing installation verified", flush=True)
            return
        if any((root / "database").iterdir()) or any((root / "state").iterdir()):
            raise RuntimeError("Existing data requires its original installation files")
        files = fetch()
        prefix = "node-payload/releases/" + revision + "/"
        for name, data in files.items():
            write(root / (prefix + name), data)
        write(root / "node-payload/active.json", json.dumps({"source_commit": revision}).encode())
        for path in (root / "node-payload").rglob("*"):
            if path.is_dir():
                path.chmod(0o755)
                os.chown(path, 65532, 65532)
        generators = {
            "secrets/web/core.key": lambda: secrets.token_hex(32),
            "secrets/database/password": lambda: secrets.token_hex(32),
            "secrets/core/credential.key": lambda: base64.b64encode(secrets.token_bytes(32)).decode(),
            "secrets/core/installation.id": lambda: str(uuid.uuid4()),
        }
        for name, generate in generators.items():
            if not (root / name).exists():
                write(root / name, (generate() + "\n").encode())
        key = (root / "secrets/web/core.key").read_text().strip()
        write(root / "secrets/core/core-key-digests.json", json.dumps([digest(key.encode())]).encode())
        names = [*generators, "secrets/core/core-key-digests.json", "node-payload/active.json",
                 *(prefix + name for name in MEMBERS)]
        receipt = {"source_commit": revision, "files": {name: digest((root / name).read_bytes()) for name in names}}
        write(marker, json.dumps(receipt).encode())
        print("Installation initialized; use the credentials service to retrieve the sign-in key", flush=True)


if __name__ == "__main__":
    os.umask(0o077)
    initialize(Path("/data"))
