import type { SDKResultMessage } from "@anthropic-ai/claude-agent-sdk";

export type NativeUsage = Pick<SDKResultMessage, "usage" | "modelUsage" | "total_cost_usd" | "subtype" | "is_error">;

// Keep the SDK scopes and estimate provenance intact. This is not public token accounting.
export function resultUsage(message: SDKResultMessage): NativeUsage {
  return structuredClone({ usage: message.usage, modelUsage: message.modelUsage,
    total_cost_usd: message.total_cost_usd, subtype: message.subtype, is_error: message.is_error });
}

// Streaming-input results mix two scopes. Keep that distinction alongside the
// untouched values so a Turn consumer cannot sum query totals across Turns.
export function executorResultUsage(message: SDKResultMessage): NativeUsage & {
  scopes: { usage: "native_turn_main_loop"; modelUsage: "query_cumulative"; total_cost_usd: "query_cumulative_estimate" };
} {
  return { ...resultUsage(message), scopes: {
    usage: "native_turn_main_loop", modelUsage: "query_cumulative",
    total_cost_usd: "query_cumulative_estimate",
  } };
}
