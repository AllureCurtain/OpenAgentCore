# Model protocol conversion library evaluation (2026-09-28) — historical, retired

> Historical record of the retired built-in model proxy and converter. The
> implementation, dependency choices, commands and results below describe that
> earlier revision only; they are not current setup instructions, dependencies
> or supported protocol combinations. The current native-only contract is
> [model execution](model-execution.md#saved-defaults-and-precedence). No
> compatibility alias, automatic migration or restoration of this converter is supported.

Scope: in-process conversion among OpenAI Chat Completions, OpenAI Responses and Anthropic Messages. No gateway processes, account pools, billing, agent loops, real model requests or credentials were used.

## Decision

The repository is MIT-licensed. Prefer native protocol operation when a harness already supports the configured provider protocol. For cross-protocol operation, use the maintained CLIProxyAPI public translator SDK at v8.0.3 behind the Runtime-owned Exchange interface. This keeps the upstream transformation implementation upgradeable. The dependency owns protocol conversion, including field selection, tools, reasoning and usage. The host only selects registered conversion routes, supplies per-request SDK state and handles HTTP/SSE transport. Pin the dependency and qualify upgrades; do not maintain a second converter through local field maps or parameter restoration. SDK behavior is not universally lossless; limitations need explicit documentation and engine-level regression evidence.

RelayKit is the cleanest small protocol-only module, but is AGPL-3.0. Do not add it as the MIT repository's default dependency without resolving licensing compatibility. Bifrost is Apache-2.0, faster than CLIProxyAPI in this bounded sample, but its provider/schema surface is substantially broader than a converter, requires Go 1.27, and brings MCP and transport dependencies even when only the two provider packages are imported.

## Fixed sources

- RelayKit: new-api commit 789c970199ea527e6a26e071915f4a4cd2c64178; module github.com/QuantumNous/new-api/relaykit; Go 1.25.1. Published relaykit/v0.2.1 points to 0aec08fee811ec6136828fda790551b49e410301 (2026-09-21). Bench/tests used the newer pinned commit.
- CLIProxyAPI: v8.0.3, acdace936fa7df2905500c7f5e0a97d683138dea; github.com/router-for-me/CLIProxyAPI/v8; MIT; Go 1.26.0 minimum.
- Bifrost: 51172dbe37df018384d0f15f94940e01c40a7fff; github.com/maximhq/bifrost/core; Apache-2.0; Go 1.27.0.

CLI v6.10.9 (2026-05-07), v6.9.0 and v6.8.55 already require Go 1.26.0. v6.8.9 uses Go 1.24.0, but predates the relevant recent translator repairs; no recent equivalent Go-1.25 version was established. Do not silently lower its go directive.

## Import closure and reusable APIs

| Candidate/import | Non-standard packages | Modules, including candidate | Relevant dependencies |
|---|---:|---:|---|
| RelayKit relayconvert | 35 | 8 | uuid, lo, gjson/sjson, match/pretty, x/text |
| CLI sdk/translator/builtin | 135 | 26 | Gin, Redis, logrus, protobuf, crypto, gjson/sjson |
| Bifrost providers/anthropic + providers/openai | 126 | 32 | sonic, fasthttp, websocket, MCP, compression, JSON-schema utilities |

Counts are actual go list -deps results, not root go.mod requirement counts. Raw evidence remains in the Linux qualification workspace; it is not a repository artifact.

RelayKit exposes ConvertRequest, ConvertResponse, NewResponseStreamState, ConvertStreamResponseChunk and FinalizeStreamResponse. It owns no HTTP or SSE reading/writing. Result objects include route, quality, usage and diagnostics. Images needing materialization require a host MediaResolver; the library does not download content.

CLI exposes sdk/translator.Registry and sdk/translator/builtin.Registry(). Request translation returns bytes; response/stream translation returns bytes/chunks plus caller-owned per-exchange state through *any. Responses as an incoming client format is openai-response; the outgoing generic Responses-shaped implementation is registered as codex. The bare registry imports only 4 internal packages and logrus/gjson/sjson/yaml, but contains no registered concrete conversions. builtin imports 50 internal packages. A static closure of the six relevant conversion directions still traverses 23 internal packages and Gin/config/util dependencies. Internal conversion packages cannot be imported outside that module. A copied subset would require removing registration init files and extracting tool/schema/JSON helpers from internal/util plus common/signature/thinking helpers; this is a maintained fork, not a drop-in tiny module.

Bifrost can be used without constructing its gateway. OpenAIChatRequest.ToBifrostChatRequest plus anthropic.ToAnthropicChatRequest is the tested conversion path. AnthropicMessageRequest.ToBifrostResponsesRequest, ToAnthropicResponsesRequest, response methods and AnthropicResponsesStreamState/ToBifrostResponsesStream/ToAnthropicResponsesStreamResponse cover the corresponding typed conversions. Chat/Responses interchange additionally uses core/schemas conversion methods. Importing those packages also compiles their provider HTTP code and shared schema/MCP definitions.

## Evidence of maintenance and defects

RelayKit commit 4eb3b9160566dc4c1f4f0e9340de254f7febf056 (2026-09-21) repairs tool_result image blocks previously sent as text and Responses reasoning segmentation after a mid-stream finish. It adds 292 lines of tests across request/response tests. Current tests include conversion matrix golden fixtures, tool-loss policies, terminal stream tails, usage and failed Responses events. Strict loss policy rejects only request conversion loss; response and stream loss still return successful results with diagnostics. This is a deliberate documented limitation, not a hard fail-closed library.

CLI commit 0a45f253344089c2f47ae698c34abe8837ec6135 (2026-09-28) repairs missing blank-line SSE terminators and adds regression tests. 4ad5bba repairs false namespace-prefix matches (2026-09-28); 75b854e repairs Claude tool-name sanitation and parameterless schemas (2026-09-24). Tests cover parallel tools, cached usage, incomplete Responses, custom-tool replay, namespace collisions and tool-result adjacency. However:
- The public transforms have no error return; missing registrations return the original body.
- TestConvertOpenAIResponsesRequestToClaude_DropsApplyPatchCustomTool explicitly requires dropping the reserved apply_patch custom grammar tool.
- The codex target rewrites generic parameters, reasoning defaults, stream/store/include.
- Claude-to-Chat and Claude-to-Responses non-stream functions expect an aggregated SSE transcript, while the Codex non-stream response functions expect a response.completed/incomplete envelope.
- Some stream conversions emit client completion at a Chat finish_reason before upstream [DONE].
- Claude-to-Responses estimates reasoning_tokens from visible text length.

The integration uses only public registered transformers. Responses-to-Messages
can compose the library's Responses-to-Chat and Chat-to-Messages routes, retaining
separate SDK state and original/translated request bytes for each step. This avoids
the direct route's reserved custom-tool behavior without local tool-name or schema
mapping. Composition must pass the same tool-history and stream tests as direct
routes. HTTP framing handles stream termination; it does not calculate usage or
interpret reasoning signatures. Non-stream Messages conversion can request an
upstream stream and pass its bounded transcript to the library's aggregate entry.
No local synthetic Messages events or field restoration are retained.

The dependency may normalize parameters, omit provider-specific extensions or
estimate a reasoning-token breakdown. Those behaviors belong to the pinned SDK;
this integration does not claim native fidelity across protocols. Prefer a native
connection when exact provider-specific semantics are needed. A library upgrade
must pass the stored contract and real engine checks before adoption.

Bifrost commit a44106ad (2026-09-24) repairs Responses types and Anthropic tool-result is_error preservation with dedicated tests; e0ad6638 (2026-09-23) preserves Anthropic extra parameters through Bedrock conversion. Its current tests also deliberately drop forced tool choice on unsupported model capabilities. Typed conversion and model-aware normalization can therefore still change semantics; typed APIs do not imply universal field preservation.

## First-batch boundary

CLIProxyAPI registers Gemini conversions in the same public registry. Enabling
Gemini also requires a provider configuration contract, model/credential URL
handling and real engine qualification. Those are not established in this batch;
the admitted protocols remain the initial three. No parallel Gemini gateway or
new provider manager is introduced.

The measured costs support using CLIProxyAPI despite its larger dependency
closure: the chosen license and tested tool coverage avoid a maintained fork.
The product qualification budget for this host is a short-request p99 below 1 ms,
a 256 KiB request p99 below 100 ms and an executable increase below 12 MiB. These
are review thresholds for dependency upgrades, not production SLOs. Concurrent
heap samples are retained for comparison and do not establish a memory ceiling.

## Bounded performance sample

All final samples ran on the same Linux host using Go 1.27.0, GOMAXPROCS=1, stripped binaries (-s -w), no HTTP or real model. Direction: Chat -> Claude, including JSON decode/encode for typed libraries. CLI's public API operates directly on bytes. Baseline: JSON decode and re-encode only. Inputs: 94-byte single text; 263,212-byte history with 32 alternating user/assistant messages (8 KiB text each); 1,496-byte function declaration/call/result/follow-up. testing.Benchmark supplies mean and allocation counts; a separate 1,000-operation sample after GC supplies percentiles. These are comparative microbenchmarks, not production SLOs.

| Candidate | Input | Mean | p95 | p99 | Bytes/op | Allocs/op |
|---|---|---:|---:|---:|---:|---:|
| Baseline | small | 6.44 us | 6.29 us | 8.47 us | 1,176 | 32 |
| Baseline | large | 0.636 ms | 0.603 ms | 0.671 ms | 548,346 | 350 |
| Baseline | tools | 24.86 us | 34.50 us | 40.57 us | 8,273 | 132 |
| RelayKit | small | 6.77 us | 10.29 us | 15.91 us | 5,008 | 22 |
| RelayKit | large | 0.721 ms | 0.898 ms | 0.961 ms | 567,051 | 194 |
| RelayKit | tools | 31.12 us | 41.94 us | 89.09 us | 15,698 | 132 |
| CLIProxyAPI | small | 19.60 us | 21.79 us | 58.41 us | 3,536 | 59 |
| CLIProxyAPI | large | 7.788 ms | 7.705 ms | 8.402 ms | 3,252,011 | 1,040 |
| CLIProxyAPI | tools | 93.12 us | 129.67 us | 176.01 us | 27,677 | 212 |
| Bifrost | small | 9.00 us | 12.31 us | 41.72 us | 3,802 | 32 |
| Bifrost | large | 1.221 ms | 1.753 ms | 1.956 ms | 1,988,175 | 296 |
| Bifrost | tools | 46.38 us | 90.94 us | 144.14 us | 25,284 | 173 |

Stripped binary sizes: baseline 2,834,592 B; RelayKit 6,549,767 B (+3,715,175 B); CLI 14,782,727 B (+11,948,135 B); Bifrost 9,924,871 B (+7,090,279 B). This is an independent harness delta, not a prediction of the exact final daemon delta.

## Validation and reproducing

RelayKit GOWORK=off go test ./... passed at the pinned commit. CLI sdk/translator and the selected Claude/OpenAI conversion test packages passed. Bifrost selected pure converter tests (TestToAnthropic, TestToBifrost, tool conversion/stream names) passed. None of these results establishes live provider compatibility.

Each checkout contains benchprotocol/main.go; baseline/main.go is the identical fixture/measurement harness without conversion. *-bench.log, *-tests.log, and *-deps.json hold raw output. The root task's product worktree contains its separate Exchange contract tests and HTTP mock-provider tests; this report does not claim full repository make check.

## Primary source links

- [RelayKit API and license](https://github.com/QuantumNous/new-api/blob/789c970199ea527e6a26e071915f4a4cd2c64178/relaykit/README.md)
- [RelayKit actual dependencies](https://github.com/QuantumNous/new-api/blob/789c970199ea527e6a26e071915f4a4cd2c64178/relaykit/go.mod)
- [RelayKit media/stream repair](https://github.com/QuantumNous/new-api/commit/4eb3b9160566dc4c1f4f0e9340de254f7febf056)
- [CLI public registry](https://github.com/router-for-me/CLIProxyAPI/blob/acdace936fa7df2905500c7f5e0a97d683138dea/sdk/translator/registry.go)
- [CLI reserved custom-tool test](https://github.com/router-for-me/CLIProxyAPI/blob/acdace936fa7df2905500c7f5e0a97d683138dea/internal/translator/claude/openai/responses/claude_openai-responses_request_test.go)
- [CLI SSE repair](https://github.com/router-for-me/CLIProxyAPI/commit/0a45f253344089c2f47ae698c34abe8837ec6135)
- [Bifrost module](https://github.com/maximhq/bifrost/blob/51172dbe37df018384d0f15f94940e01c40a7fff/core/go.mod)
- [Bifrost Responses converters](https://github.com/maximhq/bifrost/blob/51172dbe37df018384d0f15f94940e01c40a7fff/core/providers/anthropic/responses.go)
