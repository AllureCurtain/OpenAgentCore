# Native failure classification

Runtime adapters may add `code` and `http_status` to a prompt-level `error` frame.
The fields are optional neutral metadata, not a new terminal event or a public
Agents API error contract. Existing error text, Usage, Done, native Session identity
and cancellation receipts retain their existing ownership and order.

Core stores accepted values in the Turn outcome as `engine_error_code` and
`engine_http_status`. Classification is subordinate to terminal status and Core's
`error_code`: it cannot turn a completed or cancelled Turn into a failure, hide an
incomplete event stream, or override persistence and cancellation-receipt failures.
Normal delivery and terminal journal draining use the same extraction rule.

Accepted codes are `authentication_error`, `rate_limit_exceeded`,
`usage_limit_exceeded`, `server_overloaded`, `server_error`, `invalid_request`,
`resource_not_found`, `request_timeout`, `context_length_exceeded`, `cyber_policy`
and `connection_failed`. Core diagnostics expose these categories only for failed
`engine_failed` Turns. Their params are empty except `connection_failed`, whose
`http_status` is a valid integer or null. Only `connection_failed` retains an integer HTTP status
in 100..599; all other status metadata is discarded. Missing, malformed and unknown
optional values degrade to unclassified metadata without discarding Usage or Done.
Old Runtime frames remain generic harness failures. Old readers ignore new fields.
The catalog is not a claim that every harness can produce every classification.

Codex 0.153.4 uses only the exact root terminal Turn's `codexErrorInfo`, never
notification text or a retry notification. Its finite string mappings cover
unauthorized, usageLimitExceeded, rateLimitExceeded, contextWindowExceeded,
serverOverloaded, internalServerError, badRequest and cyberPolicy. The connection
object variants preserve valid upstream status. responseTooManyFailedAttempts maps
to rate_limit_exceeded only for 429; otherwise it is connection_failed. Session
budget, misalignment policy, rollback, sandbox and steering-state errors remain
unclassified. A successful terminal Turn does not inherit an earlier error.

Claude SDK 0.3.269 records finite root assistant errors against current submitted
input UUIDs and the same native Session. Only a matching unsuccessful native result
commits that candidate. Replayed, synthetic, child, foreign and completed-input
messages cannot classify the root failure; recovered results clear candidates.
Authentication variants map to authentication_error, billing_error to
usage_limit_exceeded, rate_limit to rate_limit_exceeded, overloaded to
server_overloaded, invalid_request to invalid_request, model_not_found to
resource_not_found and server_error to server_error. Unknown/max-output-token,
max-turn/budget/structured-retry results and unstructured exceptions remain generic.
The bridge retains native usage and identity and rejects stale classification after
malformed output or an unsuccessful bridge process exit. A validated native failure
may itself have a nonzero native child exit; that is distinct from bridge failure.

Neither adapter classifies error prose, exposes provider messages through safe
metadata, nor synthesizes tool timeouts. request_timeout has no current producer.
MiniMax remains unclassified. Real provider rejection evidence is distinct from
controlled protocol/SDK-stream tests and must be qualified against a matched Runtime.
