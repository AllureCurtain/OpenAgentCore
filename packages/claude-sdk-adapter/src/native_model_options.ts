import type { Options } from "@anthropic-ai/claude-agent-sdk";

export type NativeModelOptions = Pick<Options, "effort" | "thinking">;

// Go compiles these private options after validating the shared Harness contract.
// This boundary only checks structure; native enums, budgets and combinations
// have one semantic validator in the Go adapter declaration.
export function parseNativeModelOptions(value: unknown): NativeModelOptions | undefined {
  if (value === undefined) return undefined;
  const fail = (): never => { throw new Error("invalid_request"); };
  if (!value || typeof value !== "object" || Array.isArray(value)) return fail();
  const options = value as Record<string, unknown>;
  if (Buffer.byteLength(JSON.stringify(options), "utf8") > 16 * 1024) return fail();
  for (const [key, option] of Object.entries(options)) {
    if (key === "effort") {
      if (typeof option !== "string") return fail();
    } else if (key === "thinking") {
      if (!option || typeof option !== "object" || Array.isArray(option)) return fail();
      const thinking = option as Record<string, unknown>;
      if (typeof thinking.type !== "string") return fail();
      for (const [field, setting] of Object.entries(thinking)) {
        if (field === "type" || field === "display") {
          if (typeof setting !== "string") return fail();
        } else if (field === "budgetTokens") {
          if (typeof setting !== "number" || !Number.isFinite(setting)) return fail();
        } else return fail();
      }
    } else return fail();
  }
  return options as NativeModelOptions;
}
