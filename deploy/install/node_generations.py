"""Private node generation preparation and collection; Core owns all authorization."""
import copy
import contextlib
import fcntl
import stat
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import zipapp


def atomic_json(path, value):
    descriptor, temporary = tempfile.mkstemp(prefix=".generation-", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w") as stream:
            json.dump(value, stream, sort_keys=True)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        descriptor = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def install_helper(root, args, installer):
    """Keep the executed, trusted installer available to the unprivileged node."""
    settings = root / "preparation.json"
    value = {"source_url": args.source_url or args.core_url}
    if settings.exists() and installer.private_json(settings) != value:
        raise installer.InstallError("The retained node artifact origin differs; preserve its configuration")
    target = root / "generation-preparer.pyz"
    installer.existing_file(target)
    source = Path(sys.argv[0])
    if getattr(args, "bundle", None) is not None:
        source = args.bundle / "node-install.pyz"
    with tempfile.TemporaryDirectory(dir=root) as directory:
        staged = Path(directory) / "helper.pyz"
        if source.suffix == ".pyz" and source.is_file() and not source.is_symlink():
            shutil.copyfile(source, staged)
        else:
            package = Path(directory) / "package"
            package.mkdir(mode=0o700)
            source_dir = Path(installer.__file__).parent
            for name in ("node_install.py", "node_spec.py", "distribution.py", "node_generations.py", "node_update.py"):
                shutil.copyfile(source_dir / name, package / ("__main__.py" if name == "node_install.py" else name))
            zipapp.create_archive(package, staged, compressed=True)
        os.chmod(staged, 0o600)
        os.replace(staged, target)
    atomic_json(settings, value)


def retained_configs(root, installer):
    directory = root / "state/node/generations"
    result = {}
    base = installer.private_json(root / "provider.json")
    if base and not (directory / (str(base["generation"]) + ".dropped")).exists():
        result[base["generation"]] = base
    if directory.exists():
        for path in sorted(directory.glob("*.json")):
            if not re.fullmatch(r"[1-9][0-9]*\.json", path.name):
                raise installer.InstallError("Invalid retained generation filename")
            value = installer.private_json(path)
            if value is None or value.get("generation") != int(path.stem):
                raise installer.InstallError("Invalid retained generation configuration")
            if not (directory / (path.stem + ".dropped")).exists():
                result[value["generation"]] = value
    return result


def owned_root(args, installer):
    if os.getuid() == 0:
        raise installer.InstallError("Generation operations run only as the node service user")
    root = Path.home() / ".oac/nodes" / args.installation_id
    if not root.is_dir():
        raise installer.InstallError("Retained node installation is missing")
    installer.safe_directory(root)
    identity = installer.private_json(root / "state/node/identity.json")
    if not identity or identity["identity"]["installation_id"] != args.installation_id:
        raise installer.InstallError("Retained node identity differs")
    return root, identity


def generation_home(root, configuration, base, installer):
    runtime = configuration["specification"]["runtime"]
    previous = base["microsandbox"]
    if (runtime["runtime_sha256"], runtime["firmware_sha256"]) == (previous["runtime_sha256"], previous["firmware_sha256"]):
        return Path(previous["runtime_home"])
    material = ":".join((configuration["installation_id"], runtime["runtime_sha256"], runtime["firmware_sha256"]))
    home = Path.home() / ".oac/m" / hashlib.sha256(material.encode()).hexdigest()[:12]
    if len(os.fsencode(home)) > 48:
        raise installer.InstallError("HOME is too long for versioned microsandbox socket paths")
    return home


def image_available(value, installer):
    try:
        if value["provider"] == "docker":
            seccomp = Path(value["docker"]["seccomp_file"])
            installer.existing_file(seccomp)
            json.loads(seccomp.read_text())
            raw = installer.checked(list(installer.DOCKER) + ["image", "inspect", value["docker"]["image"], "--format", "{{.Id}} {{.Os}}/{{.Architecture}}"], "Cannot inspect pinned image")
            return raw.strip() == value["docker"]["image"] + " linux/amd64"
        micro = value["microsandbox"]
        for key in ("helper_path", "runtime_path", "firmware_path"):
            if not installer.existing_file(Path(micro[key])):
                return False
        if (installer.file_digest(Path(micro["runtime_path"])) != micro["runtime_sha256"]
                or installer.file_digest(Path(micro["firmware_path"])) != micro["firmware_sha256"]):
            return False
        env = dict(os.environ, MSB_BACKEND="local", MSB_HOME=micro["runtime_home"], MSB_PATH=micro["runtime_path"], MSB_LIBKRUNFW_PATH=micro["firmware_path"])
        image = json.loads(installer.checked([micro["runtime_path"], "image", "inspect", micro["image"], "--format", "json"], "Cannot inspect pinned image", env=env))
        return image.get("digest") == micro["image"].split("@", 1)[1] and image.get("architecture") == "amd64" and image.get("os") == "linux"
    except (installer.InstallError, OSError, ValueError):
        return False


def runtime_files(root, value, args, manifest, sums, installer):
    """Restore only absent immutable bytes; existing conflicts are never replaced."""
    source = args.configuration["specification"]["runtime"]["source_commit"]
    release = root / "releases" / source
    if value is not None:
        if args.provider == "microsandbox":
            release = Path(value["microsandbox"]["helper_path"]).parents[2]
            if any(Path(value["microsandbox"][key]) != release / name for key, name in zip(
                    ("helper_path", "runtime_path", "firmware_path"), installer.MICRO)):
                raise installer.InstallError("Retained Runtime artifact paths differ")
        else:
            release = Path(value["docker"]["seccomp_file"]).parents[1]
            if Path(value["docker"]["seccomp_file"]) != release / "runtime/seccomp.json":
                raise installer.InstallError("Retained Runtime seccomp path differs")
        if release not in (root, root / "releases" / source):
            raise installer.InstallError("Retained Runtime artifacts are outside this installation")
    installer.no_links(release)
    installer.safe_directory(release)
    names = ("runtime/seccomp.json",) + (installer.MICRO if args.provider == "microsandbox" else ())
    saved = installer.private_json(release / "manifest.json")
    if (release / "manifest.json").exists():
        if saved is None:
            raise installer.InstallError("Retained Runtime manifest is unreadable")
        installer.node_spec.verify_release(args.configuration, saved)
    # Inspect every existing byte before starting any repair, so a missing file
    # cannot hide a conflicting sibling or redirect a later download.
    for name in names:
        path = release / name
        installer.no_links(path)
        if installer.existing_file(path):
            expected = sums[name] if name == "runtime/seccomp.json" else installer.distribution.artifact(manifest, name)["sha256"]
            if installer.file_digest(path) != expected:
                raise installer.InstallError("Retained Runtime artifact checksum differs")
    for name in names:
        installer.safe_directory((release / name).parent)
        if name == "runtime/seccomp.json":
            installer.download(args.source_url, name, release, sums[name], prefix="releases/" + source + "/")
        else:
            installer.distribution.obtain_artifact(manifest, name, release / name)
            os.chmod(release / name, 0o700)
    atomic_json(release / "manifest.json", manifest)
    return release


def prepare(args, installer):
    root, identity = owned_root(args, installer)
    with installer.install_lock(root), collection_lease(root, args.generation, installer):
        directory = root / "state/node/generations"
        if (directory / (str(args.generation) + ".dropped")).exists():
            raise installer.InstallError("A dropped generation cannot be adopted again")
        args.core_url = identity["core_url"]
        args.configuration = installer.node_spec.fetch(args, "", identity, installer.open_request,
                                                       generation=args.generation, allow_selection_change=True)
        if args.configuration["specification_digest"] != args.specification_digest:
            raise installer.InstallError("Core generation identity differs from its authorization")
        args.provider = args.configuration["provider"]
        configurations = retained_configs(root, installer)
        base = installer.private_json(root / "provider.json")
        runtime = args.configuration["specification"]["runtime"]
        value = configurations.get(args.generation)
        retained = value is not None
        if retained:
            installer.node_spec.verify_provider(value, args.configuration, value.get("docker", {}).get("image"))
        else:
            for candidate in configurations.values():
                if candidate["specification"]["runtime"] == runtime and image_available(candidate, installer):
                    value = copy.deepcopy(candidate)
                    value["generation"] = args.generation
                    value["specification"] = args.configuration["specification"]
                    if args.provider == "microsandbox":
                        value["microsandbox"].update(value["specification"]["resources"])
                    break
        if value is None or not image_available(value, installer):
            settings = installer.private_json(root / "preparation.json")
            args.source_url = installer.origin(settings["source_url"])
            args.bundle = None
            manifest, sums = installer.metadata(args.source_url, prefix="releases/" + runtime["source_commit"] + "/")
            installer.node_spec.verify_release(args.configuration, manifest)
            if args.provider == "microsandbox":
                args.runtime_home = Path(value["microsandbox"]["runtime_home"]) if retained else generation_home(root, args.configuration, base, installer)
            if not retained:
                # Persist the exact authorized identity before any artifact/import
                # mutation. Interrupted preparation must remain discoverable after
                # node restart; file presence still requires a real provider probe.
                value = installer.provider_config(root / "releases" / runtime["source_commit"], args, manifest, runtime["image_id"])
                target = directory / (str(args.generation) + ".json")
                if installer.existing_file(target) and installer.private_json(target) != value:
                    raise installer.InstallError("Immutable generation configuration differs")
                atomic_json(target, value)
                retained = True
            release = runtime_files(root, value, args, manifest, sums, installer)
            if args.provider == "microsandbox":
                installer.safe_directory(args.runtime_home)
                owner = args.runtime_home / "oac-installation.json"
                if not owner.exists() and any(args.runtime_home.iterdir()):
                    raise installer.InstallError("Versioned microsandbox store contains unowned state")
                installer.write_once(owner, installer.json_text({"installation_id": args.installation_id}))
            runtime_image = installer.prepare_runtime(release, args, manifest)
            if retained:
                installer.node_spec.verify_provider(value, args.configuration, runtime_image)
            else:
                value = installer.provider_config(release, args, manifest, runtime_image)
        target = directory / (str(args.generation) + ".json")
        if installer.existing_file(target) and installer.private_json(target) != value:
            raise installer.InstallError("Immutable generation configuration differs")
        atomic_json(target, value)


@contextlib.contextmanager
def collection_lease(root, generation, installer):
    directory = root / "state/node/generations"
    installer.safe_directory(directory)
    path = directory / (str(generation) + ".lease")
    descriptor = os.open(path, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600)
    try:
        info = os.fstat(descriptor)
        if (not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600
                or info.st_uid != os.geteuid() or info.st_nlink != 1):
            raise installer.InstallError("Invalid generation helper lease")
        try:
            fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise installer.InstallError("Generation helper is still active") from error
        named = path.lstat()
        if (named.st_dev, named.st_ino) != (info.st_dev, info.st_ino):
            raise installer.InstallError("Generation helper lease was replaced")
        yield
    finally:
        # Never unlink a lease: a replacement inode could bypass an old helper.
        os.close(descriptor)


def collect(args, installer):
    root, _ = owned_root(args, installer)
    with installer.install_lock(root), collection_lease(root, args.generation, installer):
        configurations = retained_configs(root, installer)
        value = configurations.get(args.generation)
        if value is None:
            return
        if installer.node_spec.digest(value["provider"], value["specification"]) != args.specification_digest:
            raise installer.InstallError("Collection grant does not match the local generation")
        marker = root / "state/node/generations" / (str(args.generation) + ".legacy-unfenced")
        if marker.exists() or marker.is_symlink():
            raise installer.InstallError("The original v1 generation retains unfenced legacy helpers; its local payload is kept")
        others = [item for generation, item in configurations.items() if generation != args.generation]
        if value["provider"] == "microsandbox":
            micro = value["microsandbox"]
            shared = [item for item in others if item["microsandbox"]["runtime_home"] == micro["runtime_home"]]
            env = dict(os.environ, MSB_BACKEND="local", MSB_HOME=micro["runtime_home"], MSB_PATH=micro["runtime_path"], MSB_LIBKRUNFW_PATH=micro["firmware_path"])
            if not any(item["microsandbox"]["image"] == micro["image"] for item in shared):
                # No force: native ownership can still refuse removal after Core's grant.
                installer.checked([micro["runtime_path"], "image", "remove", micro["image"], "--quiet"], "Runtime image is still in use", env=env)
            if not shared:
                raw = installer.checked([micro["runtime_path"], "sandbox", "list", "--format", "json"], "Cannot verify empty microsandbox store", env=env)
                if json.loads(raw) != []:
                    raise installer.InstallError("Microsandbox store still contains native sandboxes")
                home = Path(micro["runtime_home"])
                installer.safe_directory(home)
                if installer.private_json(home / "oac-installation.json") != {"installation_id": args.installation_id}:
                    raise installer.InstallError("Microsandbox store ownership differs")
                # Keep receipt/lock history conservatively; an empty VM list is not
                # authority to discard unknown provider receipts or shared host state.
        else:
            image = value["docker"]["image"]
            if not any(item["docker"]["image"] == image for item in others):
                installer.checked(list(installer.DOCKER) + ["image", "rm", image], "Docker image is still in use")
        directory = root / "state/node/generations"
        installer.safe_directory(directory)
        atomic_json(directory / (str(args.generation) + ".dropped"), {"specification_digest": args.specification_digest})
        (directory / (str(args.generation) + ".json")).unlink(missing_ok=True)
        source = value["specification"]["runtime"]["source_commit"]
        if not any(item["specification"]["runtime"]["source_commit"] == source for item in others):
            release = root / "releases" / source
            if release.exists() and not release.is_symlink() and release.resolve().parent == (root / "releases").resolve():
                shutil.rmtree(release)
