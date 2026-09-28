import { useQuery } from "@tanstack/react-query";
import type { CoreHarnessKind } from "@agents-core-web/agents-client";
export interface NamedResource {
  id: string;
  name: string;
  revision: number;
}
export interface ModelProfile extends NamedResource {
  model: string;
}
export interface MCPProfile extends NamedResource {
  label: string;
  url: string;
}
export interface RuntimeProfile extends NamedResource {
  environment: "none" | "openai_hosted";
}
export interface AgentTemplate extends NamedResource {
  model_id: string;
  harness: CoreHarnessKind;
  instructions: string;
  skill_ids: string[];
  mcp_ids: string[];
}
export interface AgentInstance extends AgentTemplate {
  template_id: string;
  runtime_id: string;
  core_agent_id: string;
  model: string;
  environment: {
    type: "none" | "openai_hosted";
    environment_template_id?: string;
  };
  tools: unknown[];
}
export async function product<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const response = await fetch(`/app/${path}`, {
    method,
    headers: { "Content-Type": "application/json" },
    ...(body ? { body: JSON.stringify(body) } : {}),
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error?.message || "请求失败");
  return data as T;
}
export const useResources = <T>(kind: string) =>
  useQuery({ queryKey: [kind], queryFn: () => product<T[]>(kind) });
export const useModels = () => useResources<ModelProfile>("models");
export const useTemplates = () => useResources<AgentTemplate>("templates");
export const useRuntimes = () => useResources<RuntimeProfile>("runtimes");
export const useMCPs = () => useResources<MCPProfile>("mcps");
export const useInstances = () => useResources<AgentInstance>("instances");

export const harnessNames = {
  claude_sdk: "Claude Code",
  codex: "Codex",
  mcode: "MiniMax Code",
};
