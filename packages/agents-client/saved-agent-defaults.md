# Saved Agent execution defaults

The TypeScript client accepts a complete provider bundle when creating or updating
a saved Agent. Keep its key in private application configuration:

```ts
import { OpenAIAgentsClient } from "@agents-core-web/agents-client";

const client = new OpenAIAgentsClient({ baseUrl: coreURL, token: tenantToken });
const agent = await client.createAgent({
  model: "requested-model",
  x_agents_core: {
    harness: "codex",
    harness_config: { model_reasoning_effort: "high" },
    model_provider: {
      protocol: "responses",
      base_url: modelBaseURL,
      api_key: modelAPIKey,
    },
  },
});

// Future Sessions inherit the saved defaults; saving itself does not execute.
const session = await client.createSession({
  agent_id: agent.id,
  environment: { type: "openai_hosted" },
  input: "Follow the saved Agent instructions.",
});

// Change only the model; the saved harness and provider remain configured.
await client.updateAgent(agent.id, { model: "another-model" });

// Clear only the provider, retaining the harness.
await client.updateAgent(agent.id, { x_agents_core: { model_provider: null } });
```

Reads return `ModelProviderView`, containing safe endpoint/limit fields and
`api_key_configured`, never `api_key`. It is distinct from `ModelProviderInput`:
do not submit a read response as an update. Replacing a provider requires its full
protocol, endpoint and key; MiniMax Code also requires both token limits.
Defaults may be absent from a read: an Agent saved with only a provider has
no `harness`, and one saved with an empty extension reads `x_agents_core: {}`.

On update, omitted fields follow the [extension contract](../../contracts/agents-api/harness-selection.md).
A null provider clears its saved bundle, while
`x_agents_core: null` clears the extension and its secret. An omitted harness
defers protocol compatibility to Session admission. Existing Sessions retain their
configuration snapshots. Session inline `agent.x_agents_core` accepts the harness
and its native `harness_config`; one-off provider overrides belong in the Session's
top-level `x_agents_core`. The model remains the ordinary `agent.model` field.
Use `{}` to clear native parameters; parameter names and validation belong to the
selected harness. See [the extension contract](../../contracts/agents-api/harness-selection.md)
for resolution and inheritance rules.

A Session read preserves its explicit harness and native parameters in
`agent.x_agents_core`; it does not invent an explicit harness selection.
Administrators can inspect the configuration committed for one Session, including
the provider selection, without reading credentials:

```ts
const frozen = await admin.retrieveSessionExecutionConfiguration(projectId, session.id);
console.log(frozen.model.value, frozen.model.source, frozen.harness.value);
// Historical provider snapshots may be unavailable.
if (frozen.model_provider.status === "available") {
  console.log(frozen.model_provider.configuration?.protocol);
}
```

Configuration reads do not execute or wake Sessions. See
[the query contract](../../contracts/agents-api/execution-configuration.md).

Deployment defaults use the same provider, model and native-parameter fields through
`AdminClient`:

```ts
await admin.setHarnessModelConfiguration("codex", {
  model_provider: { protocol: "responses", base_url: modelBaseURL, api_key: modelAPIKey },
  model: "requested-model",
  harness_config: { model_reasoning_effort: "high" },
});
const defaults = await admin.retrieveHarnessModelConfiguration("codex");
console.log(defaults.model, defaults.harness_config, defaults.model_provider.api_key_configured);
```

`ModelConfigurationInput` carries the write-only key. `ModelConfigurationView`
contains a safe provider view, and `CoreHarness.model_configuration` exposes the
same deployment resource with observation timestamps. Existing Sessions keep their
frozen configuration when defaults change.
