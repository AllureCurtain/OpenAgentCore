# Runtime model protocol conversion

The model provider bundle names the upstream protocol, endpoint, credential and
limits. The Agent model and execution harness are separate selections. Runtime
uses the adapter's native protocols to choose a direct connection or a local
conversion endpoint automatically; model names never select a protocol.

| Engine | Anthropic Messages upstream | OpenAI Responses upstream | OpenAI Chat Completions upstream |
| --- | --- | --- | --- |
| Claude SDK / Claude Code | Native | Convert Messages to Responses and back | Convert Messages to Chat Completions and back |
| Codex | Convert Responses to Messages and back | Native | Convert Responses to Chat Completions and back |
| MiniMax Code | Native | Native | Native |

These are model communication paths, not claims that every model implements every
engine capability. MiniMax Code uses its pinned native `custom_provider.api`
selection (`anthropic-messages`, `openai-responses`, `openai-completions`). Do not
force it through another protocol merely to exercise a converter.

## Ownership and transport

Core freezes one confidential provider bundle and sends it through the existing
execution contract. It does not emit engine-specific provider options. The same
Runtime implementation serves Core-created and self-hosted environments. Session
scheduling, environments, native execution, tools, Skills and MCP retain their
existing owners. No gateway service, accounts, routing catalog, billing subsystem
or additional Agent loop is installed.

A conversion endpoint binds only loopback and requires a random per-endpoint
credential. Native configuration receives that credential; the upstream key stays
inside Runtime. Only the configured model endpoint is exposed. Upstream redirects
are refused, and upstream error bodies are never exposed as native diagnostics.
Native URL query flags are accepted on the configured operation but are not
forwarded to the provider. Streaming Chat requests enable `include_usage` so the
provider can send its final usage frame. Token conversion remains in the SDK.
SSE data is converted incrementally and flushed without waiting for the complete
answer. Chat completion requires either `[DONE]` after a valid finish reason, or
a separate empty-choice usage tail after that finish reason. The latter is used by compatible providers that omit `[DONE]`. A bare EOF without a protocol end cannot complete an exchange.
Closing the execution resource cancels requests and releases the endpoint;
individual Turn completion does not establish Session resource ownership.
Same-protocol execution keeps the original native connection and behavior.

The upstream base URL has the native protocol's usual semantics: Responses and
Chat Completions append `/responses` or `/chat/completions`; include `/v1` in that
base when the provider requires it. Messages appends `/v1/messages`, avoiding a
second `/v1` when already present. Public provider admission requires HTTPS.
There is no endpoint probing or automatic model substitution.

## Conversion limits

The conversion dependency is pinned to CLIProxyAPI v8.0.3 (MIT). Only its
public translator SDK is embedded; gateway, account and management services are
not started. The SDK owns field, tool, image, reasoning and usage conversion.
Runtime supplies per-request SDK state and the HTTP/SSE connection. It does not
maintain field allowlists, parameter restoration, tool-name mappings, reasoning
signature interpretation or token accounting alongside the dependency.

Cross-protocol conversion inherits the pinned library's behavior. Provider-only
tools, grammar enforcement, opaque reasoning state, detailed reasoning usage and
other native extensions are not universally portable. The SDK may omit or
normalize those fields. Exact preservation of arbitrary provider extensions is
not a supported cross-protocol guarantee. The SDK's Responses target applies
Codex defaults to stream/store/parallel-tool settings and can omit sampling and
token-limit controls. Custom grammar tools become string-argument functions;
the target does not enforce the original grammar. For Chat providers that send
empty or provisional usage on a finish chunk before definitive usage, the pinned
SDK's Messages output can retain the earlier counts. These inherited behaviors
are not repaired with local conversion rules. Same-protocol connections retain
the native API. Gemini conversion is not enabled in this first set.

Dependency updates are explicit version changes, validated with request/response,
stream and real engine regression tests. Prefer an upstream release for conversion
repairs; any unavoidable local patch requires a separately justified narrow scope.

The project is unpublished. Historical engine-specific provider option snapshots
and alternate legacy wire shapes are unsupported. No automatic migration, old
configuration reader or data deletion accompanies this change.

## Validation

Use the same engine/protocol/capability matrix for hosted and self-hosted model
communication. Validate multiple tool rounds, text deltas, supported image and
reasoning forms, cancellation, abnormal termination, errors and usage. A synthetic
HTTP fixture or one-line native answer alone is not real model qualification.
Pure conversion benchmarks and HTTP control tests establish separate evidence;
record real native-model acceptance and any unverified cells explicitly.

See the [pinned library evaluation](model-protocol-library-evaluation.md) and
[product benchmarks](model-protocol-benchmarks.md) for dependency and measured
conversion costs. These measurements are separate from live model acceptance.
