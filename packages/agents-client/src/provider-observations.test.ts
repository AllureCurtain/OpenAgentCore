import { describe, expect, it } from "vitest";
import { projectHarnessModelConfiguration } from "./admin-projection";

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
