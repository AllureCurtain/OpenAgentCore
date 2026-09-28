# Parsar Agent workbench

A small product application on OpenAgentCore, using Parsar UI. Manage models,
Skills, HTTP MCP services and runtime configurations; compose reusable Agent
templates; bind a runtime to create an independent Agent. This iteration has no
task product or execution dashboard.

## Run

Use Node 22.13+ (for built-in SQLite) and pnpm 10.30.3. Configure
[Core](../../docs/getting-started/install.md), a deployment model/provider, and a
Project API key. Store secrets in a private environment file outside the checkout:

```sh
export OAC_EXAMPLE_CORE_URL='http://127.0.0.1:8091'
export OAC_EXAMPLE_PROJECT_KEY='<project-api-key>'
pnpm install --frozen-lockfile
pnpm --filter @oac/parsar-example dev
```

Open http://127.0.0.1:18180. The Core URL is an origin without `/v1`;
remote origins require HTTPS. `OAC_EXAMPLE_PORT` changes the local port.
For a built version, run `pnpm --filter @oac/parsar-example build` and then
`pnpm --filter @oac/parsar-example start`.

## Main flow

1. **Models:** save readable names and deployment model IDs. This catalog does
   not configure providers or hold model API keys; Core owns provider credentials.
2. **Skills:** create a SKILL.md resource or upload a ZIP. Inspect versions,
   upload a new version, and choose the default. Core validates and stores bundles.
3. **MCP:** save anonymous HTTPS HTTP MCP endpoints. Bind them to templates by
   selection. Authentication, Vault management and OAuth setup are not included.
4. **Runtimes:** name a Core-managed hosted sandbox configuration or a text-only
   environment. These are placement configurations, not online machine identities.
   Machine enrollment, fixed-node routing and self-hosted registration are omitted.
5. **Templates:** combine a model, harness, instructions, Skills and MCP services.
   A template has no runtime; the harness remains part of its configuration.
6. **Agents:** select a template and runtime. The application copies configuration,
   creates a saved Core Agent and, when needed, a Core environment template with
   Skill references. Agent edits are independent of the source template.

Creating or editing an Agent does not start a Session or allocate a sandbox. Each
instance retains a `core_agent_id` and official `environment` input for future
execution. New Sessions using that pair resolve the Skill default versions;
existing Core Sessions retain their original snapshots. Templates and resource
catalog edits affect future copies or explicit Agent saves, not existing Agents
in the background. No readiness, tool connectivity or successful execution is
implied by saving a configuration.

## Ownership and storage

The loopback Node server has two small responsibilities: a fixed allowlist proxy
for public Core resources, and `/app/` CRUD for product-owned configuration. One
SQLite table stores typed JSON records. There is no ORM, background scheduler,
execution database or imported Parsar backend.

SQLite files live in `~/.oac/data/parsar-example/`. Set the absolute
`OAC_EXAMPLE_DATA_DIR` to relocate them. A hash of the Core origin and Project key
selects the file; changing either selects a different local catalog. Back up the
SQLite file with the server stopped. No key is stored in it. Builds and caches
use `${OAC_DEV_HOME:-$HOME/.oac}`.

Use one server process and a dedicated Project for this local single-user example.
The server rejects cross-origin writes and keeps the Project key out of browser
responses. Core writes and SQLite writes are sequential, not a distributed
transaction: an interrupted resource save can leave an unused Core Agent or
environment template. This example has no automatic orphan cleanup or mutation
retries. Deleting a local resource is blocked while another local record uses it.
Deleting a Skill is a separate explicit Core operation.

## Validation

```sh
make check-example
make check
```

The example gate runs TypeScript, proxy and SQLite persistence/binding tests,
a production build and browser acceptance. Install Chrome with
`pnpm exec playwright install chrome` if necessary. Browser fixtures use
ports 18180/18181 and isolated SQLite data under `~/.oac/tests/parsar-example/`.
They verify resource management, template-to-instance binding, independent edits,
Skill versions and mobile/help behavior. Synthetic fixtures are not model execution.

The opt-in live browser probe requires an already started example backed by an
isolated real Project:

```sh
OAC_EXAMPLE_LIVE_URL=http://127.0.0.1:18180 \
OAC_EXAMPLE_LIVE_MODEL='<configured-model>' \
pnpm --filter @oac/parsar-example test:live
```

It leaves sample resources for inspection and verifies saved Core Agent and Skill
bindings; it does not claim MCP connectivity or start model execution.

## UI provenance

The copied UI primitives, `src/lib/utils.ts`, scroll-anchor helper, `src/style.css`
and `public/*` originate in [Parsar](https://github.com/MiniMax-AI-Dev/parsar)
revision `90fafede`, under [MIT](LICENSE). The responsive body minimum width and
application pages are local adaptations. Navigation animation adapts
[Motion Primitives](https://github.com/ibelick/motion-primitives), with reduced-motion
support and its [MIT notice](MOTION-PRIMITIVES-LICENSE). Multica's resource and
Agent/template organization informs the product flow; no Multica code is copied.
