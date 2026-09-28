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

## Prerequisites and validation status

Use a current-version daemon binary for the host's OS and architecture, and install
the native harness selected by the Session. The daemon installer saves local
configuration; it does not install the harness, create a container, or register an
OS service. Native harness dependencies must already be available to its process.
Claude on Windows requires Git Bash. The MiniMax adapter does not currently support
Windows.

| Platform | Current validation status |
| --- | --- |
| Linux | Existing Runtime platform; the removal of inner sandboxing needs the current change's focused checks |
| macOS | Native daemon lifecycle and Files tests passed with the race detector; native harness validation is in progress |
| Windows | Native CI build and automated execution tests are required and have not yet passed for this change; cross-compilation is not acceptance, and no manual Windows acceptance is recorded |

These are validation results for the current implementation work, not a claim that
every harness/model combination has passed on every platform.

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
The Windows commands describe the installation interface; they do not replace the
pending native CI validation above.

Optional install flags:

| Flag | Meaning |
| --- | --- |
| `--capability-directory ABS` | Local destination for the installed capability snapshot; defaults to `capabilities` under `OAC_RUNTIME_HOME`. This is an operator setting, not a public API destination |
| `--tool-env-file ABS` | JSON object of string-valued tool environment variables. Use it for explicitly configured tool/MCP variables; it is not a dotenv or shell file |

For example, a tool environment file can contain `{"EXAMPLE_SETTING":"value"}`.
Treat files containing credentials as secrets. Retain the credential and tool
configuration files at their configured paths; installation records those paths.

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

Managed system-package installation remains under its existing contract while the
ordinary apt ownership decision is pending. This guide does not claim that apt has
been migrated or that a native daemon can install packages without the launching
user's required permissions.
