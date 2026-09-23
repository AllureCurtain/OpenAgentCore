import type {
  AgentReasoningEffort,
  AgentReasoningSummary,
  AgentServiceTier,
  AgentTextFormat,
  CoreHarnessKind,
  CreateAgentInput,
  SavedAgent,
  ConfigurableAgentToolInput,
  ServiceHttpMcpToolInput,
  UpdateAgentInput,
} from "@agents-core-web/agents-client";

import { isCoreWhitespaceOnly } from "./session-admission";
import type { VaultCatalog } from "../vaults/vault-catalog";
import i18n from "../../i18n";

const ta = (key: string, options?: Record<string, unknown>) => i18n.t(key as never, { ns: "agents", ...options });

export type TextVerbosity = "low" | "medium" | "high";

export interface FunctionToolDraft {
  kind: "function";
  name: string;
  description: string;
  parameters: string;
}

export interface McpToolDraft {
  kind: "mcp";
  serverLabel: string;
  serverUrl: string;
  allowedToolsMode: "all" | "list";
  allowedTools: string;
  /** Preserve omitted/null while this existing profile is untouched. */
  allowedToolsValue: string[] | null | undefined;
  required: boolean | undefined;
  credentialId?: string | null;
}

export interface ReadOnlyToolDraft {
  kind: "read-only";
  value: unknown;
  label: string;
}

export type AgentToolDraft = FunctionToolDraft | McpToolDraft | ReadOnlyToolDraft;
export type AgentFormSubmitInput = CreateAgentInput | UpdateAgentInput;

export interface AgentFormValues {
  name: string;
  harness: CoreHarnessKind | "";
  harnessModified: boolean;
  model: string;
  instructions: string;
  metadata: string;
  reasoningEffort: AgentReasoningEffort | "";
  reasoningSummary: AgentReasoningSummary | "";
  serviceTier: AgentServiceTier;
  textFormat: AgentTextFormat;
  textVerbosity: TextVerbosity;
  tools: AgentToolDraft[];
  toolsModified: boolean;
}

export interface AgentFormValidation {
  configurationError?: string;
  harnessError?: string;
  input?: AgentFormSubmitInput;
  metadataError?: string;
  modelError?: string;
  nameError?: string;
  toolsError?: string;
}

export type AgentFormIntent = "create" | "update";

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function hasOnlyKeys(value: Record<string, unknown>, allowed: readonly string[]): boolean {
  return Object.keys(value).every((key) => allowed.includes(key));
}

function isEmptyObject(value: unknown): boolean {
  return value == null || isRecord(value) && Object.keys(value).length === 0;
}

function safeMcpUrl(value: unknown): value is string {
  if (
    typeof value !== "string"
    || /^\p{White_Space}|\p{White_Space}$/u.test(value)
    || /[\u0000-\u0020\u007f\\]/u.test(value)
    || value.includes("?")
    || value.includes("#")
    || /%(?![0-9A-Fa-f]{2})/u.test(value)
  ) return false;
  const schemeSeparator = value.indexOf("://");
  const authority = schemeSeparator >= 0 ? value.slice(schemeSeparator + 3).split("/", 1)[0] : "";
  if (!authority || authority.includes("@") || authority.includes("%") || /[{}\x60]/u.test(authority)) return false;
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol) && Boolean(url.hostname) && !url.username && !url.password;
  } catch {
    return false;
  }
}

function editableFunction(value: unknown): FunctionToolDraft | null {
  if (!isRecord(value) || value.type !== "function" || !hasOnlyKeys(value, ["type", "name", "description", "parameters", "defer_loading"])) return null;
  if (
    typeof value.name !== "string"
    || isCoreWhitespaceOnly(value.name)
    || new TextEncoder().encode(value.name).length > 512
    || typeof value.description !== "string"
    || !isRecord(value.parameters)
    || value.defer_loading === true
    || value.defer_loading !== undefined && typeof value.defer_loading !== "boolean"
  ) return null;
  return {
    kind: "function",
    name: value.name,
    description: value.description,
    parameters: JSON.stringify(value.parameters, null, 2),
  };
}

function editableMcp(value: unknown, catalog: VaultCatalog | null): McpToolDraft | null {
  if (!isRecord(value) || value.type !== "mcp" || !hasOnlyKeys(value, [
    "type", "server_label", "transport", "allowed_tools", "connection_origin", "credential_id", "request_metadata", "required",
  ])) return null;
  const transport = value.transport;
  const allowedTools = value.allowed_tools;
  if (
    typeof value.server_label !== "string"
    || isCoreWhitespaceOnly(value.server_label)
    || !isRecord(transport)
    || !hasOnlyKeys(transport, ["type", "server_url", "headers"])
    || transport.type !== "http"
    || !safeMcpUrl(transport.server_url)
    || !isEmptyObject(transport.headers)
    || value.connection_origin !== "service"
    || value.credential_id !== undefined && value.credential_id !== null && typeof value.credential_id !== "string"
    || !isEmptyObject(value.request_metadata)
    || value.required !== undefined && typeof value.required !== "boolean"
    || allowedTools !== undefined && allowedTools !== null
      && (!Array.isArray(allowedTools) || allowedTools.some((name) => typeof name !== "string" || name.length === 0))
  ) return null;
  const credentialId = typeof value.credential_id === "string" ? value.credential_id : null;
  if (credentialId) {
    const credential = catalog?.credentials.find((candidate) => candidate.id === credentialId);
    if (!credential || credential.auth.mcp_server_url !== transport.server_url) return null;
  }
  return {
    kind: "mcp",
    serverLabel: value.server_label,
    serverUrl: transport.server_url,
    allowedToolsMode: Array.isArray(allowedTools) ? "list" : "all",
    allowedTools: Array.isArray(allowedTools) ? allowedTools.join("\n") : "",
    allowedToolsValue: Array.isArray(allowedTools) ? allowedTools : allowedTools as null | undefined,
    required: value.required as boolean | undefined,
    credentialId,
  };
}

function readOnlyToolLabel(value: unknown): string {
  if (!isRecord(value) || typeof value.type !== "string") return ta("savedTool.malformed");
  if (value.type === "tool_search" && hasOnlyKeys(value, ["type"])) return ta("savedTool.toolSearch");
  if (
    value.type === "programmatic_tool_calling"
    && hasOnlyKeys(value, ["type", "enabled"])
    && (value.enabled === undefined || typeof value.enabled === "boolean")
  ) return ta("savedTool.programmatic");
  if (value.type === "function" && value.defer_loading === true) return ta("savedTool.deferred");
  if (value.type === "mcp" && value.credential_id != null) return ta("savedTool.credentialedMcp");
  if (value.type === "function") return ta("savedTool.unsupportedFunction");
  if (value.type === "mcp") return ta("savedTool.unsupportedMcp");
  return ta("savedTool.unsupported");
}

export function projectSavedTool(value: unknown, catalog: VaultCatalog | null = null): AgentToolDraft {
  return editableFunction(value) ?? editableMcp(value, catalog) ?? {
    kind: "read-only",
    value,
    label: readOnlyToolLabel(value),
  };
}

export function toolDraftsFromAgent(agent: SavedAgent | undefined, catalog: VaultCatalog | null): AgentToolDraft[] {
  return (agent?.tools ?? []).map((tool) => projectSavedTool(tool, catalog));
}

export function valuesFromAgent(agent?: SavedAgent, catalog: VaultCatalog | null = null): AgentFormValues {
  return {
    name: agent?.name ?? "",
    harness: agent?.x_agents_core?.harness ?? "",
    harnessModified: false,
    model: agent?.model ?? "",
    instructions: agent?.instructions ?? "",
    metadata: JSON.stringify(agent?.metadata ?? {}, null, 2),
    reasoningEffort: agent?.reasoning.effort ?? "",
    reasoningSummary: agent?.reasoning.summary ?? "",
    serviceTier: agent?.service_tier ?? "auto",
    textFormat: agent?.text.format ?? { type: "text" },
    textVerbosity: agent?.text.verbosity ?? "medium",
    tools: toolDraftsFromAgent(agent, catalog),
    toolsModified: false,
  };
}

export function serializeAgentToolDrafts(
  drafts: readonly AgentToolDraft[],
  catalog: VaultCatalog | null,
): { error?: string; tools: ConfigurableAgentToolInput[] } {
  const functionNames = new Set<string>();
  const mcpLabels = new Set<string>();
  let functionCount = 0;
  const tools: ConfigurableAgentToolInput[] = [];
  for (const draft of drafts) {
    if (draft.kind === "read-only") {
      return { error: ta("errors.readOnlyTools"), tools };
    }
    if (draft.kind === "function") {
      functionCount += 1;
      const name = draft.name;
      if (isCoreWhitespaceOnly(name)) return { error: ta("errors.functionName"), tools };
      if (new TextEncoder().encode(name).length > 512) return { error: ta("errors.functionNameLength"), tools };
      if (functionNames.has(name)) return { error: ta("errors.functionUnique"), tools };
      functionNames.add(name);
      let parameters: unknown;
      try {
        parameters = JSON.parse(draft.parameters);
      } catch {
        return { error: ta("errors.functionJson", { name }), tools };
      }
      if (!isRecord(parameters)) return { error: ta("errors.functionSchema", { name }), tools };
      tools.push({ type: "function", name, description: draft.description, parameters, defer_loading: false });
      continue;
    }
    const label = draft.serverLabel;
    if (isCoreWhitespaceOnly(label)) return { error: ta("errors.mcpLabel"), tools };
    if (mcpLabels.has(label)) return { error: ta("errors.mcpUnique"), tools };
    if (!safeMcpUrl(draft.serverUrl)) return { error: ta("errors.mcpUrl", { name: label }), tools };
    const allowedTools = draft.allowedToolsMode === "all"
      ? draft.allowedToolsValue
      : draft.allowedTools.split("\n").map((name) => name.trim()).filter(Boolean);
    mcpLabels.add(label);
    const selectedCredential = draft.credentialId
      ? catalog?.credentials.find((credential) => credential.id === draft.credentialId)
      : null;
    if (draft.credentialId && !selectedCredential) {
      return { error: ta("errors.credentialMissing", { name: label }), tools };
    }
    if (selectedCredential && selectedCredential.auth.mcp_server_url !== draft.serverUrl) {
      return { error: ta("errors.credentialUrl", { name: label }), tools };
    }
    const mcp: ServiceHttpMcpToolInput = {
      type: "mcp",
      server_label: label,
      transport: { type: "http", server_url: draft.serverUrl },
      connection_origin: "service",
      ...(draft.credentialId ? { credential_id: draft.credentialId } : {}),
      ...(allowedTools === undefined ? {} : { allowed_tools: allowedTools }),
      ...(draft.required === undefined ? {} : { required: draft.required }),
    };
    tools.push(mcp);
  }
  if (functionCount > 64) return { error: ta("errors.functionCount"), tools };
  return { tools };
}

export function validateAgentForm(
  values: AgentFormValues,
  intent: AgentFormIntent = "create",
  catalog: VaultCatalog | null = null,
): AgentFormValidation {
  const model = values.model.trim();
  if (isCoreWhitespaceOnly(model)) return { modelError: ta("errors.modelRequired") };

  const name = values.name.trim();
  if ([...name].length > 128) {
    return { nameError: ta("errors.nameTooLong") };
  }

  let parsed: unknown;
  try {
    parsed = values.metadata.trim() ? JSON.parse(values.metadata) : {};
  } catch {
    return { metadataError: ta("errors.metadataJson") };
  }

  if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
    return { metadataError: ta("errors.metadataObject") };
  }

  const metadata = parsed as Record<string, unknown>;
  if (Object.values(metadata).some((value) => typeof value !== "string")) {
    return { metadataError: ta("errors.metadataStrings") };
  }
  const metadataEntries = Object.entries(metadata) as [string, string][];
  if (metadataEntries.length > 16) {
    return { metadataError: ta("errors.metadataCount") };
  }
  if (metadataEntries.some(([key, value]) => [...key].length > 64 || [...value].length > 512)) {
    return { metadataError: ta("errors.metadataLength") };
  }

  if (intent === "create") {
    if (values.reasoningEffort || values.reasoningSummary) {
      return { configurationError: ta("errors.reasoningDefault") };
    }
    if (values.serviceTier !== "auto") {
      return { configurationError: ta("errors.serviceTier") };
    }
    if (values.textFormat.type !== "text") {
      return { configurationError: ta("errors.textFormat") };
    }
    if (values.textVerbosity !== "medium") {
      return { configurationError: ta("errors.verbosity") };
    }
  }

  let serializedTools: ConfigurableAgentToolInput[] | undefined;
  if (intent === "create" ? values.tools.length > 0 : values.toolsModified) {
    const toolResult = serializeAgentToolDrafts(values.tools, catalog);
    if (toolResult.error) return { toolsError: toolResult.error };
    serializedTools = toolResult.tools;
  }

  const input: CreateAgentInput = {
    model,
    name: name || null,
    instructions: values.instructions.trim() || null,
    metadata: metadata as Record<string, string>,
    service_tier: values.serviceTier,
    text: {
      format: values.textFormat,
      verbosity: values.textVerbosity,
    },
    ...(serializedTools ? { tools: serializedTools } : {}),
  };
  if (intent === "create") {
    if (values.harness) input.x_agents_core = { harness: values.harness };
  } else if (values.harnessModified) {
    input.x_agents_core = values.harness ? { harness: values.harness } : null;
  }
  if (intent === "update") {
    input.reasoning = {
      effort: values.reasoningEffort || null,
      summary: values.reasoningSummary || null,
    };
  } else if (values.reasoningEffort || values.reasoningSummary) {
    input.reasoning = {
      ...(values.reasoningEffort ? { effort: values.reasoningEffort } : {}),
      ...(values.reasoningSummary ? { summary: values.reasoningSummary } : {}),
    };
  }

  return { input };
}

export interface RequestGate {
  begin(): number;
  isCurrent(request: number): boolean;
  invalidate(): void;
}

export function createRequestGate(): RequestGate {
  let current = 0;
  return {
    begin() {
      current += 1;
      return current;
    },
    isCurrent(request) {
      return request === current;
    },
    invalidate() {
      current += 1;
    },
  };
}
