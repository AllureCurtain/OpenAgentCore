# Parsar example

A small, complete Parsar application on OpenAgentCore: configure an Agent, submit
a task, inspect its history, continue the conversation and cancel execution.
The UI reuses the original product's Issue Ledger, dialogs, controls, status icons,
light/dark tokens and brand assets. There is one task list and one Agent list;
task details are a second page, configuration lives in dialogs, and optional help
uses focusable question-mark tooltips.

## Run

Use Node 22+ and pnpm 10.30.3 from the repository root. First install and configure
[Core](../../docs/getting-started/install.md), including a supported model/provider
and a ready node for hosted execution. Create a dedicated Project and issue its
API key in the Core administrator console. A Core key cannot execute tasks.

Keep the following settings in your shell or a private environment file outside
the checkout. They are server-only and must never use a `VITE_` prefix:

```sh
export OAC_EXAMPLE_CORE_URL='http://127.0.0.1:8091'
export OAC_EXAMPLE_PROJECT_KEY='<project-api-key>'
pnpm install --frozen-lockfile
pnpm --filter @oac/parsar-example dev
```

Open http://127.0.0.1:18180. The Core URL is an origin without `/v1`; use HTTPS
for a remote deployment. Set `OAC_EXAMPLE_PORT` to change the local port.
For a built version:

```sh
pnpm --filter @oac/parsar-example build
pnpm --filter @oac/parsar-example start
```

Builds and caches go under `${OAC_DEV_HOME:-$HOME/.oac}`. The example binds only
to loopback and is intended for one trusted local user. It has no user login or
public hosting mode. The server injects the Project key into an allowlisted set
of `/v1` requests and rejects cross-origin browser calls. It exposes no Core
administrator or machine routes.

1. Open **Agents**, create an Agent with a model available in your deployment,
   and optionally select a harness. Provider credentials come from Core deployment
   configuration; this example does not edit provider secrets.
2. Open **Tasks**, create a task and select that Agent. Use the hosted environment
   for coding/tools, or `none` for the deployment's qualified text-only profile.
3. Open the task to inspect messages and expandable tool output, send a follow-up
   or cancel execution. Refreshing the page reloads Core's saved history.

## Boundaries

- The example uses `packages/agents-client`'s `OpenAIAgentsClient` and no product
  server, database, administrator client or native runtime code.
- Each task is one Core Session. Its display title is Session metadata; Agents,
  Turns, Items and execution status remain Core-owned. Lists show the configured
  Project's resources, so use a dedicated Project for an isolated trial.
- The detail page polls persisted history every two seconds; it does not promise
  token streaming or SSE replay. Item history is paginated up to 10,000 items;
  exceeding that bound produces an explicit error. Task and Agent lists support
  cursor pagination.
- Session `idle` is not a success claim. Details use the latest persisted Turn
  for completed/cancelled status. A cancellation acknowledgement is not completion.
- Creation and message retries retain the same request and idempotency key in
  this tab's session storage, including after reload. A lost response locks the
  pending input for an explicit same-request retry. No write is retried automatically.
  Session storage contains task text; closing the tab clears this local retry aid.
- Team orchestration, approval/function-result handling, self-hosted enrollment,
  files/artifact browsing, multiple users, permissions, billing and IM are omitted.
  Required actions remain visible and can be cancelled; they are never approved
  automatically. Existing resources are read through the same Project API.

## Validate

```sh
make check-example
make check
```

`check-example` includes TypeScript, proxy isolation/error tests, history/status
tests, a build and Playwright browser workflows. Install Chrome with
`pnpm exec playwright install chrome` if needed. Browser fixtures use ports
18180/18181 and synthetic model results; they do not prove live model execution.
Their screenshots/traces go under `~/.oac/tests/parsar-example/` (or `OAC_DEV_HOME`).
The opt-in browser probe runs against an already started example server backed by
an isolated live Project (it creates one Agent and task):

```sh
OAC_EXAMPLE_LIVE_URL=http://127.0.0.1:18180 \
OAC_EXAMPLE_LIVE_MODEL='<available-model>' \
pnpm --filter @oac/parsar-example test:live
```

It defaults to Claude Code; set `OAC_EXAMPLE_LIVE_HARNESS` to the UI option label
for another qualified harness. It exercises actual output, reload, follow-up and
persisted cancellation and leaves its task available for inspection.

Manual acceptance additionally needs your configured Core, Project key and provider:
create an Agent and task, verify actual output, reload its history, then send a
longer follow-up and cancel it. Verify Core's final Turn status before claiming
cancellation. Never commit credentials or real task content as test fixtures.

## UI provenance

The copied `src/components/ui/*`, `src/lib/utils.ts`,
`src/lib/use-ledger-scroll-anchor.ts`, `src/style.css` and `public/*` come from
[Parsar](https://github.com/MiniMax-AI-Dev/parsar) revision `90fafede`, under
the accompanying [MIT license](LICENSE). Strict indexed-access adjustments in the
Ledger, the responsive body minimum width and the example's application pages are
local adaptations.

The animated navigation selection and execution shimmer adapt the
[Motion Primitives](https://github.com/ibelick/motion-primitives) Animated Background
and Text Shimmer patterns, using Motion with reduced-motion support and Parsar
tokens. Its [MIT notice](MOTION-PRIMITIVES-LICENSE) is retained. Markdown uses the
same React Markdown foundation as Parsar, with GFM and copyable code blocks; no
additional agent runtime or model SDK is introduced for presentation.
