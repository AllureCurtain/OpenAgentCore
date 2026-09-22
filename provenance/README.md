# Source import

This is a tracked-source snapshot from
[`MiniMax-AI-Dev/parsar@72ab4d37d49245f15b63d34f5741780e540bcec0`](https://github.com/MiniMax-AI-Dev/parsar/tree/72ab4d37d49245f15b63d34f5741780e540bcec0).
`source.json` records the original SHA-256 of every imported file. No untracked
files, credentials, build artifacts or source Git history were imported.
The source repository was not modified or stripped of Core.

## Scope

| Included | Purpose |
| --- | --- |
| `services/agents-api` | Service, workers, providers, operator commands, migrations, generated queries, tests and deployment files |
| `contracts/agents-api` | Pinned protocol, schema/types, extensions and coverage evidence |
| `apps/parsar-daemon` | Native execution daemon and existing adapters |
| `internal/agentdaemon`, `internal/agentskill`, `internal/runtimecrypto`, `internal/obs/log` | Shared execution protocol, placement, assets, crypto and logging |
| `packages/agents-client` | Core HTTP client and protocol tests, independent of product business code |
| `packages/codex-executor`, `packages/codex-harness` | Pinned native helpers and Harness artifact builder |
| `packages/claude-sdk-adapter`, `packages/mcode-harness`, `packages/tsconfig` | Existing alternative native execution adapters and their build dependencies |
| Selected `scripts` and workflows | API/runtime build, packaging, database/protocol and native checks |

Parsar's Web, product server/database, business CLI, plugin UI, product deployment
and business documentation remain exclusively in the source repository. Existing
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
