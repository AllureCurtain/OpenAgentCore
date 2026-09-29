import { describe, expect, it } from "vitest";
import { projectCoreHarnessList, projectHarnessModelConfiguration } from "./admin-projection";

const provider = { object: "core.model_configuration", harness: "codex", model: "test-model", harness_config: { model_reasoning_effort: "high" }, model_provider: { protocol: "responses", base_url: "https://model.example/v1", api_key_configured: true }, updated_at: "2026-09-26T08:00:00Z", last_used_at: null, last_error_code: null, last_error_at: null };
describe("deployment default observations", () => {
  it("preserves null and safe observations", () => {
    expect(projectHarnessModelConfiguration(provider)).toEqual(provider);
    const observed = { ...provider, last_used_at: "2026-09-26T08:01:00Z", last_error_at: "2026-09-26T08:00:30Z", last_error_code: "authentication_error" };
    expect(projectHarnessModelConfiguration(observed)).toEqual(observed);
  });
  it("rejects private state, raw codes and inconsistent pairs", () => {
    for (const extra of [{ revision: "private" }, { recovery_pending: true }, { last_error_code: "raw provider secret", last_error_at: provider.updated_at }, { last_error_code: "context_length_exceeded", last_error_at: provider.updated_at }, { last_error_code: "server_error" }, { last_error_at: provider.updated_at }, { last_used_at: "invalid" }, { last_used_at: undefined }]) {
      expect(() => projectHarnessModelConfiguration({ ...provider, ...extra })).toThrow();
    }
  });
});

describe("adapter model configuration support", () => {
  const support = { protocols: ["anthropic", "responses", "chat_completions"], native_protocols: ["responses", "anthropic"], accepts_harness_config: true, token_limits_required: false };
  const harness = { object: "core.harness", id: "codex", enabled: false, default: false, model_configuration: null, model_configuration_support: support };
  it("preserves build declarations and native default order without inferring readiness", () => {
    const value = { object: "list", data: [harness] };
    expect(projectCoreHarnessList(value)).toEqual(value);
    const requiredLimits = { ...harness, id: "mcode", model_configuration_support: { ...support, native_protocols: ["anthropic"], accepts_harness_config: false, token_limits_required: true } };
    expect(projectCoreHarnessList({ object: "list", data: [requiredLimits] }).data).toEqual([requiredLimits]);
  });
  it("rejects missing, malformed or contradictory declarations", () => {
    for (const value of [undefined, null, {}, { ...support, protocols: [] }, { ...support, protocols: ["unknown"] },
      { ...support, protocols: ["responses", "responses"] }, { ...support, native_protocols: [] },
      { ...support, native_protocols: ["unknown"] }, { ...support, protocols: ["responses"], native_protocols: ["anthropic"] },
      { ...support, native_protocols: ["responses", "responses"] }, { ...support, accepts_harness_config: "true" },
      { ...support, token_limits_required: null }, { ...support, ready: true }]) {
      expect(() => projectCoreHarnessList({ object: "list", data: [{ ...harness, model_configuration_support: value }] })).toThrow();
    }
  });
});
