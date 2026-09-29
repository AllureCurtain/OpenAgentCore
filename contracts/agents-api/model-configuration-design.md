# Unified model configuration protocol design

Status: planned protocol. This document defines the target design, not newly
accepted HTTP input or current adapter support. [Model execution](model-execution.md)
owns the implemented API; [Harness onboarding](harness-onboarding.md#native-model-configuration)
lists current native parameters. No runtime, public schema or adapter behavior
changes in this design-only revision. Existing fields below are reused owners,
not a claim that every value already executes on every Harness.

This is the canonical owner of the planned public model field vocabulary,
resolution and application semantics. The planned adapter obligation is indexed
in [agent/harness.go](../../apps/parsar-daemon/internal/agent/harness.go).
Implementation must update the current API documentation and generated schemas
together before advertising new fields.

## 1. Shared path

Web deployment defaults and public Agent/Session configuration use the same value types, validation, resolution and planning functions. Their differences are authority, source and when the complete Agent/input is available.

    Deployment model defaults ─┐
    Saved Agent configuration ─┼─> resolve one immutable model configuration
    Inline Session Agent ──────┘       |
                               validate requested behavior
                                      |
                         compile Harness + transport plan
                                      |
                        native preparation and every request

A default is a reusable model/generation fragment. It contains no instructions, messages, tool definitions, MCP declarations, Skills, workspace settings or lifecycle policy. Saving defaults checks the shared fragment; Session admission supplies Agent tools and other context and completes the same validation. Unresolved contextual obligations are never permission to execute.

There is no capability-certification database, model catalog requirement or second execution loop.

## 2. One public owner per field

Reuse the existing public Agents API fields instead of placing another copy inside generation. Public wire envelopes need not look identical across different resources; their model/generation fragments use the same types and resolver.

| Semantic field | Public Agent / Session request owner | Deployment default owner |
| --- | --- | --- |
| Exact model ID | Saved Agent model; Session agent.model | model |
| Upstream connection | Saved Agent x_agents_core.model_provider; Session x_agents_core.model_provider override | model_provider |
| Model capability declarations and limits | Saved/inline Agent x_agents_core.model_capabilities | model_capabilities |
| Reasoning effort and summary | Agent reasoning.effort and reasoning.summary | reasoning, using the same type |
| Text format/schema and verbosity | Agent text.format and text.verbosity | text, using the same type |
| Service tier | Agent service_tier | service_tier |
| New generation options absent from the public contract | Saved/inline Agent x_agents_core.generation | generation |
| Tool definitions, native search declaration and search options | Existing Agent tools and its typed variants | Not copied into defaults |
| Actual message/file/image content | Existing input / MessageInput and tool-result content | Not copied into defaults |
| Optional vendor-only model options | Saved/inline Agent x_agents_core.harness_config | harness_config |

There is no additional Session-level generation, model_capabilities or harness_config override. A Session overrides its saved Agent through agent.x_agents_core. The connection override remains in its current confidential Session envelope. This removes the current three-location native-parameter precedence rather than retaining aliases.

Provider becomes connection-only: protocol, base_url and write-only api_key. Move its current context_window and max_output_tokens into model_capabilities.limits.context_window and model_capabilities.limits.max_output_tokens. Remove the old provider locations in the same change; do not accept both.

The existing endpoint /core/v1/harnesses/{harness}/model-configuration remains the deployment-default resource. Its response includes safe configuration and a support description, never credentials or a usable local proxy address.

## 3. Common vocabulary

The public vocabulary is not reduced to the switches available in today's three adapters. A recognized option can still be explicitly unsupported by a particular execution plan.

| Group | Fields / concepts | Meaning |
| --- | --- | --- |
| Model limits | context_window, max_output_tokens | Declared model ceilings, not the requested generation budget |
| Modalities | input: text, image, audio, video, file; output: text, image, audio, video, file | Capability vocabulary; modalities need format/placement constraints, not just one boolean |
| Generation limits | generation.max_output_tokens, generation.stop | Per model-inference-call constraints; not a whole Agent Turn or Session budget |
| Sampling | generation.sampling.temperature, top_p, top_k, min_p, presence_penalty, frequency_penalty, repetition_penalty, seed | Distinct controls; no renaming one penalty into another |
| Reasoning | Existing effort and summary; generation.reasoning.mode and budget_tokens where the official fields do not express them | Relative effort, explicit budget and reasoning visibility are different controls |
| Structured output | Existing text.format and schema; planned json_object variant | Text and JSON Schema retain their single current owner; JSON-object mode requires an explicit contract extension and qualified output handling |
| Token probabilities | generation.logprobs, generation.top_logprobs | Token probability outputs need a qualified response/event representation |
| Tool selection | generation.tool_choice, generation.parallel_tool_calls | Selection/parallelism over the resolved Agent tool inventory; not new tool definitions |
| Tool capabilities | Function calling, parallel calling, native web search, tool search, image/tool results | Separate execution capabilities with separate requirements |
| Output modalities | generation.output.modalities and modality-specific settings such as audio format/voice | Typed extensions; message/event representation must also support the requested output |
| Delivery | Streaming, usage accounting, cancellation, continuation | Existing execution protocol capabilities; not duplicated model-generation settings |

Keep the existing public reasoning effort values: none, minimal, low, medium, high, xhigh, max. Support declarations can expose a subset. Effort is relative to the selected model; it is not a promise of equal latency, price or exact reasoning-token count across models.

Proposed reasoning.mode values are disabled, adaptive and budgeted. These describe different intentions. A budgeted request requires a positive budget_tokens value. Explicit mode disabled conflicts with an enabled effort or reasoning budget. Explicit effort none conflicts with adaptive/budgeted mode. Simultaneous relative effort and a token budget require a specifically supported combination; never assume one wins.

Numeric values must be finite and validated using typed ranges. Temperature/top-k/penalties have no universal vendor-compatible range. top_k is an integer; counts and token budgets are positive integers; seed is an integer and does not promise deterministic results. Support declarations provide applicable ranges and combinations. They must not clamp values or substitute the nearest enum.

max_output_tokens is a ceiling on tokens generated by one inference call, including reasoning tokens where the upstream counts them in completion output. If an endpoint can cap only visible text while hidden generation remains uncapped, it does not satisfy that total-output ceiling. An exact native/transport mapping or an explicit rejection is required. It is not an instruction to truncate the returned answer.

A declared context window informs Harness context management; it does not authorize Core or the proxy to delete/truncate conversation history. Unknown limits are not zero. Request budgets above a known model ceiling reject rather than being silently reduced.

Tools, images and audio/video are end-to-end behaviors. Merely forwarding or injecting a request member does not implement native tool handling, input consumption, output events or recovery.

### Field types and constraints

All entries below describe the target contract. Existing public owners retain their
current parsing/update rules until an explicitly qualified implementation changes
them. A recognized field is not automatically executable. Planned fields must not
appear as accepted HTTP input or advertised support before their implementation.

Objects reject unknown/duplicate members. Numbers must be finite. Optional values
may be absent; absence is not zero. Counts use positive integers unless the table
explicitly permits zero. Route descriptors can narrow ranges, never silently
clamp or rename a value. No arbitrary provider-specific key/value bag is added.

| Field (relative to its owner above) | Type / basic range | Meaning and combinations |
| --- | --- | --- |
| model | Nonempty string | Exact upstream identity; no alias or name-based capability inference |
| model_provider.protocol | Enum: responses, anthropic, chat_completions | Upstream protocol, independently selected from Harness |
| model_provider.base_url / api_key | HTTPS URL / nonempty write-only string | Existing connection validation and secret-storage rules remain in [model execution](model-execution.md); replace as one bundle |
| model_capabilities.limits.context_window | Positive integer | Declared total context ceiling; absence means unknown |
| model_capabilities.limits.max_output_tokens | Positive integer, no greater than declared context_window | Model output ceiling, distinct from the requested per-call budget |
| model_capabilities.input_modalities / output_modalities | Optional unique arrays of text, image, audio, video, file | Absent list means unknown; supplied list is exhaustive, so an omitted modality is unsupported. These are declarations, not actual content |
| model_capabilities.features | Typed object of feature status enums | Keys: reasoning, function_calling, parallel_tool_calls, structured_output, web_search, tool_search, logprobs. Each is supported, unsupported or unknown; absent means unknown |
| reasoning.effort | Existing enum: none, minimal, low, medium, high, xhigh, max | Relative effort; conflicts with incompatible explicit mode/budget; supported subset depends on the plan |
| reasoning.summary | Existing enum: auto, concise, detailed | Requested summary visibility/detail; does not authorize revealing private reasoning |
| text.format | Discriminated object, type text or json_schema; json_object is planned | json_schema requires its schema object. text/json_object reject schema. json_object requests valid JSON without a schema; output representation and route must be qualified |
| text.verbosity | Existing enum: low, medium, high | Output detail preference, not a token ceiling |
| service_tier | Existing enum: auto, default, flex, priority, fast | Preserve public vocabulary; qualification may accept a subset |
| generation.max_output_tokens | Positive integer, at most a known model output ceiling | Per-inference generation ceiling, including counted reasoning tokens |
| generation.stop | Array of distinct nonempty strings; [] means no configured stop sequences | Stop-generation sequences; provider-specific count/length limits come from route support |
| generation.sampling.temperature | Finite number >= 0 | Randomness control; zero is explicit. Require a qualified combination if other samplers are also supplied |
| generation.sampling.top_p | Number in (0, 1] | Cumulative-probability sampling threshold |
| generation.sampling.top_k | Integer >= 1 | Candidate-count threshold; zero is invalid rather than a vendor-specific disable alias |
| generation.sampling.min_p | Number in [0, 1] | Minimum candidate probability relative to the highest-probability token; zero is explicit |
| generation.sampling.presence_penalty / frequency_penalty | Finite numbers; qualified mapper supplies min/max | Presence-based / frequency-based penalties; independent meanings, not aliases |
| generation.sampling.repetition_penalty | Finite number > 0 | Multiplicative repetition penalty; do not map to additive presence/frequency penalties |
| generation.sampling.seed | Integer in [-(2^53-1), 2^53-1] | Random seed, limited to exact client JSON integers; not a determinism guarantee |
| generation.reasoning.mode | Enum: disabled, adaptive, budgeted | budgeted requires budget_tokens; other modes reject budget_tokens. disabled conflicts with enabled effort |
| generation.reasoning.budget_tokens | Positive integer | Reasoning budget within the total per-call budget when supplied; cannot exceed a known model output ceiling. Relative effort + budget requires a qualified combination |
| generation.tool_choice | Tagged object: type auto, none, required or function; function requires name | Function name must identify a callable tool in the resolved inventory; other types reject name. required/function need tools; none remains meaningful without tools |
| generation.parallel_tool_calls | Boolean | Explicit false constrains tool-call parallelism; true needs end-to-end consumption support |
| generation.logprobs | Boolean | Request token log probabilities; requires qualified response/event representation, not just request forwarding |
| generation.top_logprobs | Integer >= 0 | Number of alternative token probabilities; requires explicit logprobs: true; upper bound comes from the route |
| generation.output.modalities | Nonempty unique array of text, image, audio, video, file | Requested output modalities; each needs native consumption, public output/events and route qualification |
| generation.output.audio | Typed object with nonempty format and optional nonempty voice strings | Requires audio in output.modalities; formats/voices are exact provider values restricted by qualified mappings, not interchangeable labels |
| harness_config | Optional bounded JSON object with adapter-owned vendor-only keys | Current native schema remains implemented until migration. A future common-field mapping removes its native duplicate atomically |

Images/files in messages and tool results retain the existing MessageInput and
placement types. Their supported forms, sizes and event contracts have existing
owners. No image payload, tool definition, instruction or message is moved into
model_capabilities or generation. Constraints involving format, placement or
feature combinations use typed support descriptions and pure validators; the
feature statuses alone do not establish those constraints.

## 4. Capability facts and unknowns

Keep three distinguishable facts:

1. Model declaration: supplied by the operator/application or trustworthy provider metadata. Missing information is unknown; a declaration is not execution evidence.
2. Harness expression: the adapter and installed native version can implement the requested behavior, either natively or through an explicitly safe proxy path.
3. Route preservation: the chosen native/upstream protocol pair preserves the relevant behavior and values.

Use supported / unsupported / unknown, with field-specific ranges, accepted values and combinations. A generic protocol name, a translator registration or a model-name pattern is not proof.

Known model unsupported, or unqualified local Harness/route behavior, rejects before native submission. Unknown model support is allowed when the local Harness/route plan is qualified; the upstream then confirms acceptance or returns its normal error. Return the model status as unknown, not supported. Never force users to register every model in a catalog.

Do not introduce a rules DSL. Shared typed descriptors describe ordinary ranges and supported choices; adapter-owned pure functions validate combinations. The same declarations serve Core admission, Runtime and Web.

## 5. One application owner per setting

Every explicit generation field receives exactly one owner:

- native-config: the Harness adapter applies it through native configuration/SDK/Turn settings. The route must preserve the resulting semantics.
- proxy-request: modeltransport applies it to every final upstream request through the selected upstream protocol mapper. The adapter must declare that this override is safe for its loop.

No CLI flag does not imply unsupported. For example, a temperature field may be proxy-owned even when the native CLI has no temperature option. Whether any particular adapter/protocol supports this is a qualification result, not assumed in the protocol.

Proxy-owned values come from the frozen configuration. A later Harness Turn cannot overwrite them. Revalidate after protocol conversion and before forwarding. Native-owned values are checked for preservation rather than independently injected again. Native defaults conflicting with an authorized proxy-owned override must be accounted for by the adapter's safety declaration.

Select ownership once while compiling the plan. Do not silently switch owners after a failure, retry with weaker controls, drop unsupported members or add prompt instructions to imitate a setting.

There is one transport pipeline for native and converted protocols. A native protocol route may still need proxy-request parameter application. This is independent of whether protocol conversion is required.

Fields affecting native tool handling, context accounting, reasoning state/signatures or output consumption require an adapter-specific safety check. A proxy cannot claim to implement these by modifying a request alone.

## 6. Required Harness contract

The existing Executor/Turn lifecycle stays unchanged. agent/harness.go should require a registered model-planning contract alongside the factory:

~~~go
// Sketch: names and exact package placement are provisional.
type ModelConfigurationAdapter interface {
    DescribeModelSupport() HarnessModelSupport

    // Pure: no subprocess, proxy, credentials lookup or model input.
    // The same implementation is used at admission and in Runtime.
    PlanModelConfiguration(
        config ResolvedModelConfiguration,
        route ModelRoute,
        context ModelExecutionContext,
    ) (AdapterModelPlan, error)
}

type FieldApplication struct {
    Field   ModelField
    Owner   ApplicationOwner // native-config | proxy-request
}

type AdapterModelPlan struct {
    Applications []FieldApplication
    Requirements ModelRequirements
    // Native options remain adapter-private; not another public config map.
}
~~~

Mappings are ordinary tested pure functions within adapters/transport. Existing wire and adapter versions identify compatibility. Do not add mapping IDs, a mapping registry or a version/rules DSL.

The shared planner combines adapter support with route support and model declarations. No Harness-name branches in Core, source-specific adapter flows or duplicate validators.

The compiled plan must account for every resolved explicit non-clear field and every requested behavioral requirement. Missing, duplicate or conflicting ownership is an error. Identity, model limits and model declarations have their own typed handling; they are not arbitrary proxy request parameters.

Core freezes the resolved configuration, its sources and semantic/wire version in the existing Session record. It stores neither an executable local plan nor ephemeral proxy credentials. Runtime recompiles using the installed adapter and transport version, and rejects a plan it cannot preserve.

Preparation starts native resources only after local planning succeeds. The factory receives the final connection and compiled native settings. Preparation failure, cleanup ownership and uncertain submission retain the existing contracts.

Apply/check settings on every inference request: first Turn, subsequent Turns, retries owned by the native Harness, steering, tool continuations and cold recovery. New content requirements are rechecked before sending messages or tool results; a previously text-only Session does not authorize a later unsupported image.

The scope is the configured Agent's inference calls. Existing subagent configuration/override rules remain authoritative; do not silently apply the parent's model-specific limits or native settings to a child with a different model binding.

Errors identify safe field paths, reason and blocking layer. They never include keys, submitted secret values or raw upstream bodies.

## 7. Presence, inheritance, replacement and conflicts

Use presence-aware typed inputs. Do not let JSON omitempty or truthiness turn zero/false/disabled into absence.

For the new generation extension, the object is the unit of replacement:

| Input | Meaning |
| --- | --- |
| generation omitted | Inherit the entire object from the next applicable source |
| generation: null or {} | Clear the entire inherited generation object |
| Nonempty generation object | Replace the entire object; omitted fields and nested groups do not inherit |
| Explicit 0 or false inside that object | A requested value; validate it, never treat it as omission |
| Nested null | Invalid; clear by replacing the object without that field |
| A supplied list | Use that list as supplied; an empty stop list requests no stop sequences |

There is no field-level or nested-group inheritance. A replacement such as
{"sampling":{"temperature":0}} discards an inherited top_p and output budget.
A cleared option has no applied-value obligation, while explicit false/disabled
does. Safe reads expose the resolved values and their source.

Keep the existing documented public Agent update/override/null semantics for reasoning, text, tools and service_tier. The input envelope translates those operations into the same resolved representation; the new extension does not retroactively reinterpret official fields.

Provider replacement is atomic: protocol, address and key come from one source. Never combine a new endpoint with an inherited credential. API-key rotation alone is not a change in model capabilities.

Bind model capability declarations and native escape-hatch options to the selected model, provider endpoint/protocol and Harness as applicable. Changing that binding clears inherited declarations/native options unless replacements are explicitly supplied. Key-only rotation preserves the binding.

Portability rule: unified generation values explicitly stored in the Agent represent portable application intent. Preserve and revalidate them on a model change rather than silently dropping them. Do not inherit model-specific tuning from a deployment default bound to a different explicitly selected model. Existing official reasoning/text/tools inheritance remains unchanged. This deliberately differs from retaining opaque native settings belonging to the former model.

Native escape-hatch fields cannot duplicate any canonical common field, even with an equal value. For example, native model_reasoning_effort/effort must move to the common reasoning owner when that mapping lands. A conflict rejects; there is no "native wins" precedence or old alias.

Validate combinations such as reasoning-disabled plus budget, incompatible samplers, forced tool choice missing from the resolved inventory, tool choice with no tools, parallel calls that the Harness cannot consume, and requested budget above declared ceilings.

Saved/default edits affect only future Sessions. New versions must not silently reinterpret an existing Session's frozen native keys as common fields. Keep existing Session records/data untouched. If the new Runtime cannot execute an older snapshot contract, return an explicit unsupported-version error; do not translate native keys implicitly or delete data. Any required transition of executable stored snapshots needs an explicit deployment plan, not a fallback parser.

## 8. Example of the shared fragment

Deployment-default input:

~~~json
{
  "model": "provider-model-id",
  "model_provider": {
    "protocol": "responses",
    "base_url": "https://provider.example/v1",
    "api_key": "<write-only>"
  },
  "model_capabilities": {
    "limits": { "context_window": 128000, "max_output_tokens": 16000 }
  },
  "reasoning": { "effort": "low" },
  "generation": {
    "max_output_tokens": 4000
  }
}
~~~

Equivalent explicit Session configuration:

~~~json
{
  "agent": {
    "model": "provider-model-id",
    "reasoning": { "effort": "low" },
    "x_agents_core": {
      "model_capabilities": {
        "limits": { "context_window": 128000, "max_output_tokens": 16000 }
      },
      "generation": { "max_output_tokens": 4000 }
    }
  },
  "environment": { "type": "openai_hosted" },
  "x_agents_core": {
    "model_provider": {
      "protocol": "responses",
      "base_url": "https://provider.example/v1",
      "api_key": "<write-only>"
    }
  }
}
~~~

These are planned shapes, not currently accepted requests. Both must reach identical model/generation validation and planning after source resolution.

## 9. Delivery boundaries and acceptance

1. Establish this field table, value semantics and ownership contract. Put the accepted mandatory contract in harness.go and link one public field owner document.
2. Introduce shared typed fragments and resolver; separate provider connection from model limits. Remove superseded aliases/duplicate inputs together. Update Web and public clients in the same change.
3. The first implementation phase unifies existing reasoning, text, tools and images through the common planner, then qualifies per-call output budgets, temperature and top_p. Keep native/proxy ownership and immutable Session behavior. Common fields are the normal input; native JSON remains only a vendor-specific escape hatch.
4. Qualify sampling, per-call output ceilings and other proxy-applicable controls per adapter/protocol pair. Do not advertise untested combinations. Use maintained third-party mappings where they preserve the contract, behind a thin local adapter.
5. Add audio/video/file output and other operations only with native input/output, event, cancellation and recovery coverage. Defining the vocabulary now does not claim these operations are implemented.

Required evidence: equivalent default/public inputs produce the same resolved plan; presence/null/zero semantics; no secrets in safe reads; immutable snapshots and retries; new/reused/resumed native Turns; final upstream request assertions for proxy-owned fields; no double application; explicit rejection before sending unsupported inputs; focused real-model acceptance for newly claimed behavior.

Web should render ordinary unified controls from the shared support description, distinguish unknown model support from unavailable local implementation, and show the reason for an unavailable option. Do not build another handwritten per-Harness parameter schema in the frontend.

Not in this draft: model catalog/autodiscovery service, capability-certification storage, cost optimization, Session-wide budgets, new tool or Agent loops, cross-platform installers, or replacing a native Harness with a direct-model execution engine.
