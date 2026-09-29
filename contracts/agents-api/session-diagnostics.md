# Root Session and Turn diagnostics

These read-only routes require a Core key. The Project ID selects the target
space; it does not authenticate. Both return `Cache-Control: no-store`:

- `GET /core/v1/projects/{project_id}/sessions/{session_id}/diagnostics`
- `GET /core/v1/projects/{project_id}/sessions/{session_id}/turns/{turn_id}/diagnostics`

Missing, malformed, foreign and deleted resources use the existing Session/Turn
not-found rules. A Subagent Turn is not a root Turn and returns 404. Reads never
contact an executor, provision an Environment, repair history or change execution.

## Session snapshot

The object is `core.session_diagnostics`, with `session_id`, official Session
`status`, and nullable `failure`. Each read uses one repeatable-read database
snapshot and the existing public status projection. Each Session or Turn snapshot
read has a five-second budget, shortened by any earlier caller deadline.
The transaction is closed after the read, including cancellation. Failure is null
unless that projection says `failed`. Its fields are:

| Field | Value |
| --- | --- |
| `source` | `turn`, `environment` or `environment_input` |
| `turn_id` | Present only for `source: turn` |
| `code` | Fixed category from [the diagnostics catalog](core-errors.md#diagnostic-failure-categories) |
| `params` | An object of fixed safe values; `{}` for categories without parameters |
| `failed_at` | RFC3339 timestamp, or null when historical time is unknown |

Hosted provisioning failure overrides input activity; input activity overrides
the latest root Turn. The same precedence drives the public Session response.
Private outcome text, native messages, provider bodies, command text, paths and
credentials are never included. Unknown outcome codes become `internal_error`;
no prefix matching or historical reason-text parsing is used.

Provisioning parameters come from the confirmed structured receipt, persisted in
the existing failure transaction with Environment/input settlement and events.
`step` is `setup`, `python`, `npm`, `system`, `file`, `skill` or null. `index` is a
nonnegative JSON-safe integer for setup only, otherwise null. `exit_code` is 1–255
for script steps, otherwise null. Unknown or historical details remain null.
Changing these private fields does not change the public failure reason or SSE.

## Turn snapshot and Item receipt timing

The object is `core.turn_diagnostics`, with `session_id`, `turn_id`, the official
root Turn `status`, nullable `failure`, `items` and `items_truncated`. Turn failure
has `code`, `params` and nullable `failed_at`, without a source or Turn ID. Only
failed Turns have failure details; cancelled/completed Turns never inherit an
error classification from a private outcome.

`items` contains at most 1000 root Items, ordered by `(created_at, position, id)`
ascending, matching public Items order. Storage reads at most 1001 rows to detect
truncation. Each entry has `item_id`, `started_at`, nullable `completed_at`, and
nullable integer `observed_duration_ms`.

- `started_at` is the first persisted Core input/event receipt.
- `completed_at` is the first terminal input/event receipt. An Item first observed
  terminal settles at that same receipt (zero observed duration).
- An Item still in progress when its root Turn terminates settles with one database
  `clock_timestamp()` sampled after the existing Session lock and terminal
  projection. It does not use PostgreSQL transaction-start `now()` or the native
  Turn completion timestamp. All such Items in that transaction share that time.
- Repeated terminal projections preserve settlement. Already-terminal historical
  Items with no settlement retain null; no backfill or duration estimate is made.
- `observed_duration_ms` is the integer millisecond difference when both receipts
  are known, otherwise null. It is not native execution duration. Journal batching,
  transport, persistence and database wall-clock behavior affect this interval;
  the event journal normally flushes about every 100 ms. Values are not clamped.

The public successful Turn completion time may originate at the native executor;
that public timestamp remains unchanged. Public Session, Turn, Item, SSE and
`duration_ms` contracts are unchanged. Subagent diagnostics are separate work.
Native failure classification uses only
the finite top-level outcome metadata of a failed `engine_failed` Turn; see
[native classification](native-error-classification.md). Connection failure params
contain `http_status` (100–599 or null); other native categories have empty params.

The typed client exposes `retrieveSessionDiagnostics` and
`retrieveTurnDiagnostics` on `AdminClient`, retaining request cancellation and
validating scope, categories, nullability and safe parameter values. No browser
credential or Core Web implementation is added by this contract.

## Implementation rules

Root Session/Turn diagnostics are Core-key reads from one committed database
snapshot, reusing public status projection and precedence. Keep their finite
failure catalog separate from private outcome text. Persist safe provisioning
details in the existing failure transaction; never reconstruct historical details
from reason strings. Item settlement belongs to the existing Session lock and
terminal transaction: event/input receipt for normal terminal projections, one
post-lock database wall-clock sample for forced incomplete Items. Preserve
historical terminal nulls and public native completion times. See the
[diagnostics contract](session-diagnostics.md).
