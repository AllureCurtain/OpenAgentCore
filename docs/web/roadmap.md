# Web roadmap

Agents Core Web and the reusable TypeScript client are maintained in the same
repository as Core, but remain separate protocol-facing components. Roadmap work
must preserve that boundary: browser code consumes the public HTTP/SSE contract;
it does not reproduce scheduling, execution, secrets, or runtime ownership.

## M0 — monorepo integration

- build the Web and TypeScript client from the root pnpm workspace;
- run Web type checks, unit tests, production builds, and Core Doctor from
  `make check`;
- keep Core, Web, and operator documentation aligned to one reviewed revision;
- retain independent deployment of `services/agents-api` and `apps/web`.

## M1 — contract co-evolution

- update TypeScript types, fixtures, protocol coverage, and UI states in the same
  change when a Web-consumed Core resource changes;
- add raw HTTP fixtures for every response and event variant used by the UI;
- verify reconnect recovery, cancellation, Function results, pagination, and error
  presentation against the in-repository Core;
- keep unsupported capabilities explicit and fail closed.

## M2 — operator experience

- qualify repeatable local and production startup paths without exposing Docker or
  Core credentials to the browser;
- surface versioned runtime capability and readiness data only when Core publishes
  a proven public contract;
- add browser acceptance coverage for supported hosted and self-hosted Environment
  lifecycle flows;
- report missing runtime behavior as a Core gap instead of emulating it in Web.

## M3 — optional Web Cloud layer

- introduce a BFF, user sessions, authorization, audit, and secret custody as a
  separate deployment layer;
- keep product organization, role, billing, and marketplace behavior outside the
  open TypeScript client and standalone execution Core.
