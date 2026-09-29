import assert from "node:assert/strict";
import test from "node:test";
import { executorResultUsage, resultUsage } from "../dist/usage.js";

test("preserve separate SDK scopes and price provenance without recalculating", () => {
  const first = { type: "result", subtype: "success", is_error: false,
    usage: { input_tokens: 15, output_tokens: 8, cache_read_input_tokens: 30, cache_creation_input_tokens: 7 },
    modelUsage: { primary: { inputTokens: 15, outputTokens: 8, cacheReadInputTokens: 30, thinkingTokens: 3, costUSD: 0.03, costBasis: "unknown" },
      helper: { inputTokens: 100, outputTokens: 9, costUSD: 0.07, provider: "gateway" } },
    total_cost_usd: 0.1, result: "private response", session_id: "native" };
  const snapshot = resultUsage(first);
  assert.deepEqual(snapshot, { usage: first.usage, modelUsage: first.modelUsage, total_cost_usd: 0.1, subtype: "success", is_error: false });
  first.modelUsage.primary.inputTokens = 900;
  assert.equal(snapshot.modelUsage.primary.inputTokens, 15);
  assert.equal(Object.hasOwn(snapshot.modelUsage.helper, "thinkingTokens"), false);
  const resumed = resultUsage({ ...first, usage: { input_tokens: 2, output_tokens: 1 }, modelUsage: {}, total_cost_usd: 0.01 });
  assert.equal(resumed.usage.input_tokens, 2);
  assert.equal(resumed.total_cost_usd, 0.01);
});

test("reported error usage survives and missing fields are not manufactured", () => {
  const error = { subtype: "error_during_execution", is_error: true, total_cost_usd: 0.02,
    usage: { input_tokens: 12, output_tokens: 4 }, modelUsage: {} };
  assert.deepEqual(resultUsage(error), error);
  const missing = JSON.parse(JSON.stringify(resultUsage({ subtype: "error_during_execution", is_error: true })));
  assert.deepEqual(missing, { subtype: "error_during_execution", is_error: true });
});

test("executor preserves cumulative counter resets without inventing per-Turn accounting", () => {
  const values = [8, 15, 0, 2].map(total => executorResultUsage({
    subtype: "success", is_error: false, usage: { input_tokens: 2 },
    modelUsage: { primary: { inputTokens: total, costUSD: total / 100 } },
    total_cost_usd: total / 100,
  }));
  assert.deepEqual(values.map(value => value.modelUsage.primary.inputTokens), [8, 15, 0, 2]);
  assert.deepEqual(values.map(value => value.usage.input_tokens), [2, 2, 2, 2]);
  for (const value of values) {
    assert.equal(value.scopes.modelUsage, "query_cumulative");
    assert.equal(value.scopes.usage, "native_turn_main_loop");
  }
});
