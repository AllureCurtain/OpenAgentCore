# Source import

This is a tracked-source snapshot from
[`MiniMax-AI-Dev/parsar@72ab4d37d49245f15b63d34f5741780e540bcec0`](https://github.com/MiniMax-AI-Dev/parsar/tree/72ab4d37d49245f15b63d34f5741780e540bcec0).
`source.json` records the original SHA-256 of every imported Core file. No
untracked files, credentials, build artifacts or source Git history were imported
in that initial Core extraction.
The source repository was not modified or stripped of Core.

`web-source.json` separately records the later Agents Core Web import. That import
intentionally includes a bounded, hashed local Environment Templates patch because
it targets APIs already present in this Core. It records the source commit, diff
digest, Base64-encoded replayable patch and each formerly untracked file rather than representing
the working tree as a clean upstream revision. `scripts/verify-web-source.py`
checks those recorded bytes and destination mappings during `make check`.

## Scope

| Included | Purpose |
| --- | --- |
| `services/agents-api` | Service, workers, providers, operator commands, migrations, generated queries, tests and deployment files |
| `contracts/agents-api` | Pinned protocol, schema/types, extensions and coverage evidence |
| `apps/parsar-daemon` | Native execution daemon and existing adapters |
| `apps/web` | Standalone browser console for the public Core HTTP/SSE contract |
| `internal/agentdaemon`, `internal/agentskill`, `internal/runtimecrypto`, `internal/obs/log` | Shared execution protocol, placement, assets, crypto and logging |
| `packages/agents-client` | Go and TypeScript Core HTTP clients and protocol tests, independent of product business code |
| `packages/codex-executor`, `packages/codex-harness` | Pinned native helpers and Harness artifact builder |
| `packages/claude-sdk-adapter`, `packages/mcode-harness`, `packages/tsconfig` | Existing alternative native execution adapters and their build dependencies |
| Selected `scripts` and workflows | API/runtime build, packaging, database/protocol and native checks |

Parsar's product Web, product server/database, business CLI, plugin UI, product
deployment and business documentation remain exclusively in the source repository. Existing
daemon compatibility helpers stay with the unchanged daemon; their presence does
not bring a product backend or make it an execution prerequisite.

## Adaptations

Execution sources, tests, migrations and protocol artifacts are byte-identical to
the source snapshot. Five imported files have packaging-only adaptations:

- `go.mod`/`go.sum`: prune dependencies used only by the excluded product.
- `pnpm-lock.yaml`: prune excluded product workspace importers/dependencies using
  pinned pnpm 10.30.3; preserve the native adapter versions.
- `services/agents-api/RELEASE.md` and `HOSTED-RELEASE.md`: link new release commit
  IDs to this repository.

Root README, contributor rules, Makefile, Node workspace and the complete-check
workflow are standalone scaffolding. The contributor guide retains source Core
architecture rules. Product-only checks are absent; equivalent Core persistence,
native/runtime and generated-query checks remain required.

To audit the initial import:

```sh
python3 scripts/verify-source-copy.py
```

This audit is specific to the import commit. It does not prohibit subsequent Core
changes. Upstream native sources remain pinned in their existing manifests and
must be fetched using the documented builders. This repository is not an offline
vendor archive and does not claim additional live model coverage.

## Validation

See [the import acceptance record](verification.md) for commands, environment,
results and limits. No existing mx deployment is changed by this import.
