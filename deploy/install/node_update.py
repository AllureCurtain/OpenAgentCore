"""Resume a node program update without re-enrolling or replacing its Runtime."""
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

import node_generations


def retained(root, args, installer):
    installer.no_links(root)
    if not root.is_dir() or not installer.existing_file(root / "registered.json"):
        raise installer.InstallError("--update requires this user's registered node; use the original installation owner")
    installer.existing_file(root / "state/node/identity.json")
    installer.existing_file(root / "provider.json")
    identity = installer.private_json(root / "state/node/identity.json")
    provider = installer.private_json(root / "provider.json")
    if not identity or not provider:
        raise installer.InstallError("Retained node identity or provider is unreadable; preserve state")
    value = identity["identity"]
    generation = value.get("deployment_generation")
    if (type(generation) is not int or not 1 <= generation <= 9223372036854775807
            or identity.get("core_url") != args.core_url or value.get("installation_id") != args.installation_id
            or provider.get("installation_id") != args.installation_id or provider.get("generation") != generation
            or provider.get("provider") != value.get("provider")
            or installer.node_spec.digest(provider["provider"], provider["specification"]) != value.get("specification_digest")
            or args.provider not in (None, value["provider"])):
        raise installer.InstallError("Cannot identify the original enrolled generation; preserve this node")
    args.provider = value["provider"]
    # Refresh only authentication and backend lineage; the target may have moved.
    installer.node_spec.fetch(args, "", identity, installer.open_request, allow_selection_change=True)
    return identity, provider


def protocol(binary, installer):
    result = subprocess.run([str(binary), "protocol-version"], stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            timeout=10, check=False)
    if result.returncode == 0 and result.stdout == b"2\n" and not result.stderr:
        return 2
    if (result.returncode == 1 and not result.stdout
            and result.stderr.strip() == b"usage: oac-node register|run --config PATH --state-dir PATH"):
        return 1
    raise installer.InstallError("Cannot establish the node executable protocol; preserve state and inspect its release")


def atomic_copy(source, target, mode):
    descriptor, temporary = tempfile.mkstemp(prefix=".node-update-", dir=target.parent)
    try:
        with os.fdopen(descriptor, "wb") as stream, source.open("rb") as original:
            shutil.copyfileobj(original, stream)
            os.fchmod(stream.fileno(), mode)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, target)
        descriptor = os.open(target.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def journal(root, installer):
    value = installer.private_json(root / "program-update.json")
    if value is None:
        if (root / "program-update.json").exists():
            raise installer.InstallError("Node update journal is unreadable; preserve it")
        return None
    if (value.get("format") != 1 or value.get("phase") not in ("prepared", "installed", "complete")
            or any(not re.fullmatch(r"[0-9a-f]{64}", value.get(key, ""))
                   for key in ("node_sha256", "preparer_sha256", "settings_sha256"))):
        raise installer.InstallError("Node update journal differs; preserve it")
    return value


def verify_stage(root, value, installer):
    stage = root / "node-programs" / value["node_sha256"]
    installer.no_links(stage)
    for name, key in ((installer.COMMON[0], "node_sha256"), ("generation-preparer.pyz", "preparer_sha256"),
                      ("preparation.json", "settings_sha256")):
        path = stage / name
        installer.no_links(path)
        if not installer.existing_file(path) or installer.file_digest(path) != value[key]:
            raise installer.InstallError("Retained update payload differs; preserve the journal and retry with its release")
    return stage


def prepare(root, args, installer):
    with installer.install_lock(root):
        identity, provider = retained(root, args, installer)
        value = journal(root, installer)
        node_id = identity["identity"]["node_id"]
        generation = provider["generation"]
        if value and value["phase"] != "complete":
            if value.get("node_id") != node_id or value.get("enrolled_generation") != generation:
                raise installer.InstallError("Pending update belongs to another retained identity")
            verify_stage(root, value, installer)
            retain_legacy(root, value, provider, installer)
            return
        binary = root / installer.COMMON[0]
        installer.existing_file(binary)
        old_protocol = protocol(binary, installer)
        manifest, sums = installer.metadata(args.source_url, args.bundle)
        checksum = installer.distribution.artifact(manifest, installer.COMMON[0])["sha256"]
        programs = root / "node-programs"
        installer.safe_directory(programs)
        stage = programs / checksum
        installer.safe_directory(stage)
        installer.safe_directory((stage / installer.COMMON[0]).parent)
        installer.distribution.obtain_artifact(manifest, installer.COMMON[0], stage / installer.COMMON[0], args.bundle)
        os.chmod(stage / installer.COMMON[0], 0o700)
        if protocol(stage / installer.COMMON[0], installer) != 2:
            raise installer.InstallError("This release does not support node generation updates")
        node_generations.install_helper(stage, args, installer)
        if installer.file_digest(stage / "generation-preparer.pyz") != sums.get("node-install.pyz"):
            raise installer.InstallError("Use the checksum-matched node-install.pyz from the selected release")
        # Preserve the previous executable independently of every Runtime release.
        old_sha = installer.file_digest(binary)
        previous = programs / old_sha
        installer.safe_directory(previous)
        saved = previous / "previous-node"
        if not installer.existing_file(saved):
            atomic_copy(binary, saved, 0o700)
        elif installer.file_digest(saved) != old_sha:
            raise installer.InstallError("Preserved node executable differs")
        value = {"format": 1, "phase": "prepared", "node_id": node_id, "enrolled_generation": generation,
                 "legacy_unfenced_generation": generation if old_protocol == 1 else None,
                 "node_sha256": checksum, "preparer_sha256": installer.file_digest(stage / "generation-preparer.pyz"),
                 "settings_sha256": installer.file_digest(stage / "preparation.json")}
        verify_stage(root, value, installer)
        # The durable journal must never refer to unwritten staged bytes or
        # directory entries after a power interruption.
        paths = [stage / installer.COMMON[0], stage / "generation-preparer.pyz", stage / "preparation.json", saved]
        directories = {root, programs, stage, previous, (stage / installer.COMMON[0]).parent, (stage / installer.COMMON[0]).parent.parent}
        for path in paths + sorted(directories, key=lambda entry: len(entry.parts), reverse=True):
            descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
            try:
                os.fsync(descriptor)
            finally:
                os.close(descriptor)
        node_generations.atomic_json(root / "program-update.json", value)
        retain_legacy(root, value, provider, installer)


def retain_legacy(root, value, provider, installer):
    generation = value.get("legacy_unfenced_generation")
    if generation is None:
        return
    if type(generation) is not int or generation != provider["generation"]:
        raise installer.InstallError("Legacy helper retention differs from the enrolled generation")
    directory = root / "state/node/generations"
    installer.safe_directory(directory)
    path = directory / (str(generation) + ".legacy-unfenced")
    expected = {"specification_digest": installer.node_spec.digest(provider["provider"], provider["specification"])}
    if path.exists() and installer.private_json(path) != expected:
        raise installer.InstallError("Legacy helper retention marker differs")
    node_generations.atomic_json(path, expected)
    descriptor = os.open(directory.parent, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def apply(root, args, installer):
    with installer.install_lock(root):
        identity, provider = retained(root, args, installer)
        value = journal(root, installer)
        if not value or value.get("node_id") != identity["identity"]["node_id"] or value.get("enrolled_generation") != provider["generation"]:
            raise installer.InstallError("No matching prepared node update")
        stage = verify_stage(root, value, installer)
        retain_legacy(root, value, provider, installer)
        for name, mode in ((installer.COMMON[0], 0o700), ("generation-preparer.pyz", 0o600), ("preparation.json", 0o600)):
            installer.existing_file(root / name)
            atomic_copy(stage / name, root / name, mode)
        node_generations.atomic_json(root / "program-update.json", dict(value, phase="installed"))


def finish(root, args, installer):
    installer.wait_ready(root, args)
    with installer.install_lock(root):
        value = journal(root, installer)
        if not value or value["phase"] != "installed":
            raise installer.InstallError("Node update journal changed before readiness confirmation")
        node_generations.atomic_json(root / "program-update.json", dict(value, phase="complete"))


def update(args, installer):
    if os.geteuid() == 0:
        os.environ["PATH"] = installer.SAFE_PATH
        installer.host_checks()
        record = installer.node_record(args.installation_id)
        account, account_record = installer.account_plan()
        if not record or not account or not account_record or record["core_url"] != args.core_url:
            raise installer.InstallError("--update requires the existing registered system node and service account")
        if args.provider not in (None, record["provider"]):
            raise installer.InstallError("The asserted provider differs from the retained node")
        args.provider, args.system = record["provider"], True
        root = installer.SERVICE_HOME / ".oac/nodes" / args.installation_id
        unit = installer.SYSTEM_UNITS / installer.unit_name(args.installation_id)
        if unit.is_symlink() or unit.read_text() != installer.system_unit(root, args.provider):
            raise installer.InstallError("The existing system node unit differs; preserve it")
        with installer.host_lock():
            current = installer.service_account()
            if installer.node_record(args.installation_id) != record or current != account:
                raise installer.InstallError("The system node changed meanwhile; rerun --update")
            if unit.is_symlink() or unit.read_text() != installer.system_unit(root, args.provider):
                raise installer.InstallError("The system node unit changed meanwhile")
            installer.run_as(account, prepare, root, args, installer)
            installer.checked(["systemctl", "stop", unit.name], "Cannot stop the node main process")
            installer.run_as(account, apply, root, args, installer)
            installer.checked(["systemctl", "start", unit.name], "Cannot restart the retained node; rerun --update")
            installer.run_as(account, finish, root, args, installer)
    else:
        root = Path.home() / ".oac/nodes" / args.installation_id
        unit = root / installer.unit_name(args.installation_id)
        if not installer.existing_file(unit) or unit.read_text() != installer.service_unit(root):
            raise installer.InstallError("The existing user node unit differs; preserve it")
        with installer.host_lock():
            prepare(root, args, installer)
            installer.checked(["systemctl", "--user", "stop", unit.name], "Cannot stop the node main process")
            apply(root, args, installer)
            installer.checked(["systemctl", "--user", "start", unit.name], "Cannot restart the retained node; rerun --update")
            finish(root, args, installer)
    print("Node program updated with retained identity and Runtime state. Inspect the Nodes page for generation readiness.")
