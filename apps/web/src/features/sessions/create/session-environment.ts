import type {
  AgentEnvironmentInput,
  OpenAIHostedNetworkAccess,
} from "@agents-core-web/agents-client";

export type SessionEnvironmentType = AgentEnvironmentInput["type"];
export type HostedNetworkChoice = "default" | OpenAIHostedNetworkAccess;

export type SessionEnvironmentValidation =
  | { input: AgentEnvironmentInput; error: null }
  | { input: null; error: string };

const absoluteWorkspaceError = "Workspace directory must be an absolute POSIX path, for example /workspace.";
const safeWorkspaceError = "Workspace directory cannot contain NUL, carriage return, line feed, or backslash characters.";

export function validateWorkspaceDirectory(value: string): string | null {
  if (!value || value.startsWith("~") || !value.startsWith("/")) return absoluteWorkspaceError;
  if (/[\0\r\n\\]/u.test(value)) return safeWorkspaceError;
  return null;
}

const canonicalUuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export function sessionEnvironmentInput(
  type: unknown,
  workspaceDirectory: string,
  hostedNetwork: HostedNetworkChoice = "default",
  environmentTemplateId: string | null = null,
): SessionEnvironmentValidation {
  if (type === "none") {
    return { input: { type: "none" }, error: null };
  }

  if (type === "openai_hosted") {
    if (hostedNetwork !== "default" && hostedNetwork !== "enabled" && hostedNetwork !== "disabled") {
      return { input: null, error: "The managed Environment network policy is unsupported." };
    }
    // Omitting the field inherits the Core default, or a referenced Template's
    // policy. Core rejects an explicit null beside a reference, so Web never
    // sends one.
    const network = hostedNetwork === "default" ? {} : { network: { access: hostedNetwork } };
    if (environmentTemplateId === null) {
      return { input: { type: "openai_hosted", ...network }, error: null };
    }
    if (!canonicalUuidPattern.test(environmentTemplateId)) {
      return { input: null, error: "The selected Environment Template identifier is invalid." };
    }
    return {
      input: { type: "openai_hosted", environment_template_id: environmentTemplateId, ...network },
      error: null,
    };
  }

  if (type !== "self_hosted") {
    return { input: null, error: "The selected Environment type is unsupported." };
  }

  const error = validateWorkspaceDirectory(workspaceDirectory);
  if (error) return { input: null, error };

  return {
    input: {
      type: "self_hosted",
      workspace_directory: workspaceDirectory,
      capability_directories: [],
    },
    error: null,
  };
}

function record(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function exactKeys(value: Record<string, unknown>, keys: readonly string[]): boolean {
  return Object.keys(value).length === keys.length && Object.keys(value).every((key) => keys.includes(key));
}

function hasKey(value: Record<string, unknown>, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(value, key);
}

/** Revalidates the finite Web request at the App boundary instead of trusting form types. */
export function normalizeSessionEnvironmentInput(value: unknown): SessionEnvironmentValidation {
  const input = record(value);
  if (!input || typeof input.type !== "string") {
    return { input: null, error: "The Environment input is invalid." };
  }
  if (input.type === "none") {
    return exactKeys(input, ["type"])
      ? sessionEnvironmentInput("none", "")
      : { input: null, error: "The no-Environment request contains unsupported fields." };
  }
  if (input.type === "self_hosted") {
    if (
      !exactKeys(input, ["type", "workspace_directory", "capability_directories"]) ||
      !Array.isArray(input.capability_directories) ||
      input.capability_directories.length !== 0 ||
      typeof input.workspace_directory !== "string"
    ) return { input: null, error: "The self-hosted Environment request is not the supported Web profile." };
    return sessionEnvironmentInput("self_hosted", input.workspace_directory);
  }
  if (input.type === "openai_hosted") {
    const reference = hasKey(input, "environment_template_id");
    const templateId = reference
      ? typeof input.environment_template_id === "string" ? input.environment_template_id : ""
      : null;
    const base = reference ? ["type", "environment_template_id"] : ["type"];
    if (exactKeys(input, base)) return sessionEnvironmentInput("openai_hosted", "", "default", templateId);
    const network = record(input.network);
    if (
      !exactKeys(input, [...base, "network"]) ||
      !network ||
      !exactKeys(network, ["access"]) ||
      (network.access !== "enabled" && network.access !== "disabled")
    ) return { input: null, error: "The managed Environment request is not the supported basic profile." };
    return sessionEnvironmentInput("openai_hosted", "", network.access, templateId);
  }
  return { input: null, error: "The selected Environment type is unsupported." };
}
