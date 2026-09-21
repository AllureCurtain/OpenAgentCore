# Import acceptance

Verified on 2026-09-21 against source snapshot
`72ab4d37d49245f15b63d34f5741780e540bcec0`. The destination comparison base is
the empty bootstrap commit `6973875`.

## Environment

Checks ran from a standalone copy on Linux amd64 (`zju_a100_2`), with Go
1.25.13, Node 22.22.0, pnpm 10.30.3, Rust 1.95.0, Python 3.10 and PostgreSQL 16.
Only the destination repository was present in the build directory. A dedicated
PostgreSQL container and database `parsar_agents_api_core_import_tests` were used;
no product database or existing deployment was involved. Dependencies used the
existing SSH reverse network path. Credentials remained outside Git and logs.

The official Python client was installed into a task-local virtual environment
from `contracts/agents-api/upstream.json` (commit
`d7c41efee1b0802b79f3f88a678ef2052b06e9ce`).

## Results

| Command | Result |
| --- | --- |
| `python3 scripts/verify-source-copy.py` | Passed: all 1,287 imported paths present, 1,282 byte-identical; only the five documented packaging adaptations differ |
| `make check` | Passed, exit 0: sqlc regeneration, daemon/shared/API/client Go tests, real PostgreSQL tests, standalone API build, Claude SDK tests/package, MiniMax companion checks, Rust format/tests/Clippy and Codex Harness packaging checks |
| `make build-daemon` | Passed using a task-local absolute output directory |
| `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` | Passed, exit 0 |
| `AGENTS_API_SERVER_BIN=.../agents-api python services/agents-api/tests/official_client.py` | Passed, exit 0, using the pinned SDK and dedicated database |
| `AGENTS_API_IMAGE=parsar-core-import:72ab4d37 PARSAR_OFFICIAL_SDK_PYTHON=.../sdk/bin/python make check-agents-api-container` | Passed, exit 0: build standalone image and repeat the official-client suite inside read-only containers |

The official-client suites exercise SDK and raw HTTP responses, generated/upstream
schemas, persistence, service restart, retries, pagination, principal/project
isolation, Agent/Session/Turn lifecycle, vaults/credentials and explicit unsupported
options. They do not establish live model or provider qualification.

The first database attempt used a name outside the test safety allowlist and was
correctly rejected. The final run uses the required `parsar_agents_api_*_tests`
name; no safety check was removed. The task-local Python environment used pip 25.2
with PySocks after its bundled pip failed through the SOCKS proxy. No system
toolchain or shared proxy configuration was changed.

## Evidence and limits

Private full logs are retained under
`~/.parsar/tests/parsar-core-import-20260921/` on the validation host and copied to
`~/.parsar/parsar-core-import/` on the development host. Their SHA-256 digests are:

| Log | SHA-256 |
| --- | --- |
| `make-check.log` | `3777c991688d65c36f81c141f2a3b0b8d1084b2670f787bd405c3af5fd1f2d5d` |
| `official-client.log` | `29133bacfda0ff6c59b748726882557f9a56dea3744eab92927b8f8e3cf9969c` |
| `container-check.log` | `6ffb0814b617844d1b30db3e0dc21d6ba3f565137c7d384e029281cc00370ea9` |

Live paid-model execution, E2B provisioning and opt-in native Harness builds were
not rerun for this source-only import. The MiniMax packaged-native-tools test is
opt-in and skipped without its native prerequisites. Existing implementation,
fixtures and historical acceptance evidence are retained unchanged; this import
does not claim to close their documented protocol gaps.

The full import whitespace check reports existing whitespace in three Codex
patch files and the vendored E2B process proto. Those bytes are deliberately
preserved and verified against the source manifest; the new scaffolding passes
the whitespace check.

The source Parsar checkout and its Core copy remain unchanged. No mx service was
reconfigured, stopped or deployed.
