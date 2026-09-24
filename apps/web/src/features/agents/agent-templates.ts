import { type AgentFormValues, valuesFromAgent } from "./agent-form";
import i18n from "../../i18n";

const ta = (key: string) => i18n.t(key as never, { ns: "agents" });

export type AgentTemplateId =
  | "sre-incident-response"
  | "slack-teammate"
  | "data-agent"
  | "github-issue-investigation"
  | "bulk-invoice-contract-review";

export interface AgentTemplate {
  readonly id: AgentTemplateId;
  readonly name: string;
  readonly description: string;
  readonly instructions: string;
}

export const AGENT_TEMPLATES: readonly AgentTemplate[] = [
  {
    id: "sre-incident-response",
    get name() { return ta("templates.sre.name"); },
    get description() { return ta("templates.sre.description"); },
    get instructions() { return ta("templates.sre.instructions"); },
  },
  {
    id: "slack-teammate",
    get name() { return ta("templates.slack.name"); },
    get description() { return ta("templates.slack.description"); },
    get instructions() { return ta("templates.slack.instructions"); },
  },
  {
    id: "data-agent",
    get name() { return ta("templates.data.name"); },
    get description() { return ta("templates.data.description"); },
    get instructions() { return ta("templates.data.instructions"); },
  },
  {
    id: "github-issue-investigation",
    get name() { return ta("templates.github.name"); },
    get description() { return ta("templates.github.description"); },
    get instructions() { return ta("templates.github.instructions"); },
  },
  {
    id: "bulk-invoice-contract-review",
    get name() { return ta("templates.invoices.name"); },
    get description() { return ta("templates.invoices.description"); },
    get instructions() { return ta("templates.invoices.instructions"); },
  },
] as const;

export function valuesFromAgentTemplate(template: AgentTemplate): AgentFormValues {
  return {
    ...valuesFromAgent(),
    name: template.name,
    instructions: template.instructions,
  };
}
