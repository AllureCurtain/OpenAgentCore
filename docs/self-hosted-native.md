# Native self-hosted Runtime

A self-hosted Runtime runs the same `oac-daemon` on Linux, macOS or Windows.
Core uses the same Environment protocol on all three platforms; native harness
adapters handle harness differences. Core-managed Providers remain Linux-only.

The daemon runs tools with the permissions of the user who starts it. It adds no
filesystem, permission or network sandbox. Tools can access whatever that user
can access, including Runtime state and credentials. Authentication, ordinary
credential storage, atomic state writes and process cleanup do not isolate tools
from the host. When isolation is needed, run the Runtime inside an outer sandbox;
Core creates that outer boundary for managed Docker/E2B/microsandbox Environments.
The daemon does not enforce `disabled` or `restricted` network modes; combinations
without corresponding outer enforcement are unsupported.

## Supported platforms

| Platform | Codex | Claude Code | MiniMax Code |
| --- | --- | --- | --- |
| Linux | Supported | Supported | Supported |
| macOS | Supported | Supported | Supported |
| Windows | Supported | Supported | Not supported by the current adapter |

Native CI passing is the platform support criterion. Windows has not yet had
manual machine acceptance. Managed Providers support Linux only.

Use a current-version daemon binary for the host's OS and architecture. The daemon
installer saves local configuration; native harnesses and their dependencies must
already be available to its process. Claude on Windows requires Git Bash.

Create a `self_hosted` Session with its model provider and an existing absolute
`workspace_directory` on the executor host. Obtain its `environment.id` and
`environment.remote_url`, then have the Core administrator issue an
[executor credential](getting-started/self-hosted.md#without-web). Save the returned
JSON as a private local file. It contains `key_id`, `environment_id` and
`executor_token`; do not put the token in command arguments.

Use the exact `remote_url` from Core. It uses `wss://` outside loopback; `ws://` is
accepted only for a loopback Core. The Environment ID must be a canonical UUID.
Paths must be clean absolute paths in the host's syntax, and the workspace must
already exist. The install workspace must match the Session's configured workspace.

## Install and start

Choose a separate `OAC_RUNTIME_HOME` for each installation. It contains local
configuration, connection state, logs and the default capability snapshot. Keep
this same value for install, start, status, logs and stop.

On Linux or macOS:

```sh
export OAC_RUNTIME_HOME="$HOME/.oac/runtime-example"
mkdir -p "$HOME/agent-workspace"
chmod 600 "$HOME/executor-credential.json"
oac-daemon install \
  --remote 'wss://core.example/api/v1/agent-daemon/ws' \
  --environment-id '11111111-2222-4333-8444-555555555555' \
  --workspace "$HOME/agent-workspace" \
  --credential-file "$HOME/executor-credential.json"
oac-daemon start
```

On Windows, use native absolute paths in PowerShell:

```powershell
$env:OAC_RUNTIME_HOME = "$HOME\.oac\runtime-example"
New-Item -ItemType Directory -Force "$HOME\agent-workspace" | Out-Null
oac-daemon.exe install `
  --remote 'wss://core.example/api/v1/agent-daemon/ws' `
  --environment-id '11111111-2222-4333-8444-555555555555' `
  --workspace "$HOME\agent-workspace" `
  --credential-file "$HOME\executor-credential.json"
oac-daemon.exe start
```

Replace the example URL and UUID with the Session's actual values. On Windows, keep
the credential file under the intended user's normal private storage permissions.


Optional install flags:

| Flag | Meaning |
| --- | --- |
| `--capability-directory ABS` | Local destination for the installed capability snapshot; defaults to `capabilities` under `OAC_RUNTIME_HOME`. This is an operator setting, not a public API destination |
| `--tool-env-file ABS` | JSON object of string-valued tool environment variables. Use it for explicitly configured tool/MCP variables; it is not a dotenv or shell file |

For example, a tool environment file can contain `{"EXAMPLE_SETTING":"value"}`.
Treat files containing credentials as secrets. Retain the credential and tool
configuration files at their configured paths; installation records those paths.

Runtime preparation uses one Go implementation on all platforms. Its local
initialization and package directories default to `initialization` and `packages`
under `OAC_RUNTIME_HOME`. Operators can select absolute paths with
`OAC_RUNTIME_INITIALIZATION_DIRECTORY` and `OAC_RUNTIME_PACKAGE_DIRECTORY`;
managed images set these to `/environment/initialization` and
`/environment/packages`. They change storage layout, not execution permissions.
`OAC_RUNTIME_TOOL_ENV_FILE` selects an explicit tool-variable JSON file; preparation
does not overwrite an existing configuration.

npm and Python packages install only into the Runtime package directory using
npm's prefix and pip's target options. Node/npm and Python/pip must already be
installed. Setup requires Bash; on Windows it requires Git Bash, found from Git's
installation or `CLAUDE_CODE_GIT_BASH_PATH`. Missing dependencies fail preparation;
Runtime does not substitute PowerShell, cmd.exe or WSL for Bash setup.


`oac-daemon start --foreground` stays attached to the terminal instead of starting
in the background. An operator can use their own service manager to run it; the
native installer does not configure one.

## Operate the same installation

With the same `OAC_RUNTIME_HOME` still set:

```sh
oac-daemon status
oac-daemon logs -n 100
oac-daemon logs -f
oac-daemon stop
```

Use `oac-daemon.exe` for these commands on Windows. A connected Environment proves
the machine connection, not model execution: send a Turn to validate the selected
native harness and model. Stopping the daemon preserves the workspace, native
history and capability snapshot; deleting a Session does not remove host files.

After credential rotation, stop the daemon, replace the JSON at the configured
credential path with the newly issued credential for the same `key_id`, then start
it again. Do not rerun `install` over an existing installation. Revocation prevents
the old credential from reconnecting.

## Capabilities and versions

`capability_directories` supplies local source directories when the Session is
created. Runtime snapshots them before execution and resolves Skills and Plugin
MCP from the common `installed.json` format used for managed bundles. Paths use the
executor host's native syntax, not Core's filesystem. Reconnect reuses the snapshot;
source edits do not silently replace it. A new Session captures new configuration.
See [local capability directories](getting-started/self-hosted.md#local-capability-directories).
Read-only snapshot modes are an integrity hint, not protection from the same user.

Installation is current-version only. It refuses an existing installation, and
`start` refuses configuration from another daemon version. There is no in-place
upgrade, adoption of a historical container installation, or cross-version native
history migration. Preserve an old installation's files and use a fresh Runtime
home and Session when moving versions.

System dependencies must be present before Runtime execution: install them while
building the managed image/template, or prepare them as the self-hosted user.
Runtime does not run apt, request sudo or elevate the daemon's privileges.
`packages.system` is explicitly rejected on Session and Template input, including
an empty or null value; it is never ignored. A missing preinstalled dependency
fails the operation that requires it. npm/Python package installation and setup retain their supported
initialization flow using the launching user's existing permissions.


On Windows, stdio MCP commands named `npm` or `npx` (including explicit
`.cmd` paths) use the selected installation's JavaScript entrypoint with Node.
Ordinary executables run directly. Other batch wrappers must configure
`cmd.exe` explicitly, with the wrapper arguments required by that command.
