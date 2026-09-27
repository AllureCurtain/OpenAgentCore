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
            raw = installer.checked(list(installer.DOCKER) + ["image", "inspect", value["docker"]["image"], "--format", "{{.Id}} {{.Os}}/{{.Architecture}}"], "Cannot inspect pinned image")
            return raw.strip() == value["docker"]["image"] + " linux/amd64"
        micro = value["microsandbox"]
        env = dict(os.environ, MSB_BACKEND="local", MSB_HOME=micro["runtime_home"], MSB_PATH=micro["runtime_path"], MSB_LIBKRUNFW_PATH=micro["firmware_path"])
        image = json.loads(installer.checked([micro["runtime_path"], "image", "inspect", micro["image"], "--format", "json"], "Cannot inspect pinned image", env=env))
        return image.get("digest") == micro["image"].split("@", 1)[1] and image.get("architecture") == "amd64" and image.get("os") == "linux"
    except (installer.InstallError, OSError, ValueError):
        return False


def prepare(args, installer):
    root, identity = owned_root(args, installer)
    with installer.install_lock(root):
        directory = root / "state/node/generations"
        installer.safe_directory(directory)
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
        value = None
        for candidate in configurations.values():
            if candidate["specification"]["runtime"] == runtime and image_available(candidate, installer):
                value = copy.deepcopy(candidate)
                value["generation"] = args.generation
                value["specification"] = args.configuration["specification"]
                if args.provider == "microsandbox":
                    value["microsandbox"].update(value["specification"]["resources"])
                break
        if value is None:
            settings = installer.private_json(root / "preparation.json")
            args.source_url = installer.origin(settings["source_url"])
            args.bundle = None
            prefix = "releases/" + runtime["source_commit"] + "/"
            manifest, sums = installer.metadata(args.source_url, prefix=prefix)
            installer.node_spec.verify_release(args.configuration, manifest)
            release = root / "releases" / runtime["source_commit"]
            installer.safe_directory(release.parent)
            names = ("runtime/seccomp.json",) + (installer.MICRO if args.provider == "microsandbox" else ())
            if not release.exists():
                with tempfile.TemporaryDirectory(prefix=".release-", dir=release.parent) as temporary:
                    stage = Path(temporary)
                    for name in names:
                        installer.safe_directory((stage / name).parent)
                        if name == "runtime/seccomp.json":
                            with installer.fetch(args.source_url, prefix + name) as stream:
                                raw = stream.read(1024 * 1024 + 1)
                            if hashlib.sha256(raw).hexdigest() != sums.get(name):
                                raise installer.InstallError("Runtime seccomp checksum differs")
                            (stage / name).write_bytes(raw)
                            os.chmod(stage / name, 0o600)
                        else:
                            installer.distribution.obtain_artifact(manifest, name, stage / name)
                            os.chmod(stage / name, 0o700)
                    atomic_json(stage / "manifest.json", manifest)
                    os.rename(stage, release)
            saved = installer.private_json(release / "manifest.json")
            # The origin URL is transport metadata, not immutable release identity.
            installer.node_spec.verify_release(args.configuration, saved)
            for name in names:
                installer.existing_file(release / name)
                expected = sums[name] if name == "runtime/seccomp.json" else installer.distribution.artifact(manifest, name)["sha256"]
                if installer.file_digest(release / name) != expected:
                    raise installer.InstallError("Retained Runtime artifact checksum differs")
            if args.provider == "microsandbox":
                args.runtime_home = generation_home(root, args.configuration, base, installer)
                installer.safe_directory(args.runtime_home)
                owner = args.runtime_home / "oac-installation.json"
                if not owner.exists() and any(args.runtime_home.iterdir()):
                    raise installer.InstallError("Versioned microsandbox store contains unowned state")
                installer.write_once(owner, installer.json_text({"installation_id": args.installation_id}))
            runtime_image = installer.prepare_runtime(release, args, manifest)
            value = installer.provider_config(release, args, manifest, runtime_image)
        target = directory / (str(args.generation) + ".json")
        if installer.existing_file(target) and installer.private_json(target) != value:
            raise installer.InstallError("Immutable generation configuration differs")
        atomic_json(target, value)
        print(json.dumps(value))


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
