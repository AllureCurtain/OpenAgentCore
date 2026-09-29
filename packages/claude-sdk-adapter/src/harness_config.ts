import type { Options } from "@anthropic-ai/claude-agent-sdk";

export type HarnessConfig = Pick<Options, "effort" | "thinking">;

// This private bridge guard mirrors the adapter-owned admission declaration.
// Only model parameters reach Options; credentials and execution policy never do.
export function parseHarnessConfig(value: unknown): HarnessConfig | undefined {
  if (value === undefined) return undefined;
  const fail = (): never => { throw new Error("invalid_request"); };
  if (!value || typeof value !== "object" || Array.isArray(value)) return fail();
  const config = value as Record<string, unknown>;
  if (Buffer.byteLength(JSON.stringify(config), "utf8") > 16 * 1024) return fail();
  for (const [key, option] of Object.entries(config)) {
    if (key === "effort") {
      if (typeof option !== "string" || !["low", "medium", "high", "xhigh", "max"].includes(option)) return fail();
    } else if (key === "thinking") {
      if (!option || typeof option !== "object" || Array.isArray(option)) return fail();
      const thinking = option as Record<string, unknown>;
      if (!["adaptive", "enabled", "disabled"].includes(thinking.type as string)) return fail();
      for (const [field, setting] of Object.entries(thinking)) {
        if (field === "type") continue;
        if (field === "budgetTokens") {
          if (thinking.type !== "enabled" || !Number.isSafeInteger(setting) || (setting as number) < 1) return fail();
        } else if (field === "display") {
          if (thinking.type === "disabled" || !["summarized", "omitted"].includes(setting as string)) return fail();
        } else return fail();
      }
    } else return fail();
  }
  return config as HarnessConfig;
}
