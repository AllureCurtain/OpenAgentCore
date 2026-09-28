# Use OpenAgentCore

Complete the [first Session walkthrough](getting-started/quickstart.md) before using
this guide. It provides `client` and `session`; the examples below continue that
Session. For exact request fields, use the [API reference](api/README.md).

## Choose where a task runs

| Environment | What you prepare | Next step |
| --- | --- | --- |
| `openai_hosted` | Ready managed capacity and a configured model | Core creates the Environment and connects its daemon |
| `self_hosted` | Your machine, workspace, daemon and model provider | [Connect the Runtime](getting-started/self-hosted.md) to the returned Environment |
| `none` | An existing device connection and its configured Harness | Check the qualified profile in [Harness selection](../contracts/agents-api/harness-selection.md) |

Use the [Environment contract](../contracts/agents-api/environments.md) for placement,
expiry and supported combinations. Model selection and provider precedence are
explained once in the [application quickstart](getting-started/quickstart.md#model-providers).

## Continue a Session

A Session keeps the conversation and its fixed execution configuration. Submit a
follow-up after the previous Turn settles:

```python
client.beta.agents.sessions.events.create(
    session.id,
    events=[{
        "type": "agent.session.input.message",
        "input": [{
            "role": "user",
            "content": [{"type": "input_text", "text": "Summarize the file you created."}],
        }],
    }],
)
```

Read the new Turn and Items using the same observation loop as the quickstart.
Submitting while a Turn is active steers that Turn where supported; it does not
create an independent parallel task. Create a new Session for a different fixed
configuration or workspace. See [message input](../contracts/agents-api/message-input.md)
for supported content, steering and idempotency rules.

## Prepare Skills, Plugins and MCP

Select capabilities when creating the Session, before sending work that needs them.
Runtime installs one snapshot and passes its Skill paths and MCP declarations to the
Harness. Reconnect reuses that snapshot; changing local source files does not alter
an existing Session.

| Task | Guide |
| --- | --- |
| Upload a Skill source and select a version | [Source Files and Skills](../contracts/agents-api/source-files.md) |
| Use local Skill/Plugin directories | [Local capability directories](getting-started/self-hosted.md#local-capability-directories) |
| Provide initial files, user-directory packages or setup | [Environment Templates](../contracts/agents-api/environment-templates.md) |
| Declare tools, MCP and function callbacks | [Execution tools](../contracts/agents-api/execution-tools.md) |
| Choose a supported tool policy | [Tool policy](../contracts/agents-api/tool-policy.md) |

System dependencies must already exist on the host or in its image/template.
See the [Runtime installation guide](self-hosted-native.md) for dependencies and
platform support. Missing preparation fails before execution; do not retry a task
as a new Session to work around an unresolved preparation error.

## Read results and files

Turn state tells you whether execution is active, completed, failed or cancelled.
Items hold the recorded conversation and tool results; live events describe changes
as they happen. A disconnected stream is not a completion receipt. The
[history and events contract](../contracts/agents-api/history-events-usage.md) explains
pagination, event delivery and Usage.

| Resource | Use it for | Reference |
| --- | --- | --- |
| Environment Files | Inspect or transfer workspace contents | [Environment Files](../contracts/agents-api/environment-files.md) |
| Artifacts | Retrieve captured outputs associated with execution | [Artifacts and public resources](api/public-agent-api.md) |
| Source Files | Upload input used by a Skill or other declared resource | [Source Files](../contracts/agents-api/source-files.md) |

These resources have different lifetimes. Deleting a Session does not erase files
from a user-owned machine. Reclaiming managed compute is a separate Environment
operation; stopping an executor does not imply reclamation.

## Cancel and recover

To cancel current work:

```python
client.beta.agents.sessions.events.create(
    session.id,
    events=[{"type": "agent.session.input.cancel"}],
)
```

Read the Turn until its terminal state is recorded. HTTP acceptance alone does not
prove native processes have stopped. A cancellation or delivery error must be
interpreted using the returned state and the
[error contract](../contracts/agents-api/core-errors.md).

After a lost response or connection, first retrieve the existing Session, Turns
and Items. Do not assume work was never submitted and send it again. For self-hosted
execution, restart the same installation with its retained workspace and native
history; follow the [operator lifecycle](self-hosted-native.md#operate-the-same-installation).
A new Session captures a new configuration, not the history of a deleted one.

## Diagnose a failure

1. Check the Session, latest Turn and required actions in the API or Web.
2. Confirm that the Environment is connected and its selected Harness is available.
3. Check the model/provider and capability combination against
   [supported execution](../contracts/agents-api/execution-tools.md).
4. Follow [operations and troubleshooting](getting-started/operations.md) for service
   logs, credentials and node readiness. Runtime observations may be stale; the
   [diagnostics contract](../contracts/agents-api/session-diagnostics.md) identifies
   which observations are available.

Administrative actions use a different credential from application work. Keep the
[API namespace reference](api/README.md) at hand when diagnosing authentication errors.
