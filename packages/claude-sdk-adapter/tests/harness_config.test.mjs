import assert from "node:assert/strict";
import test from "node:test";
import { parseRequest } from "../dist/request.js";

const request = harness_config => JSON.stringify({ type: "executor_prepare", model: "fixture", system_prompt: "", cwd: process.cwd(), harness_config });
test("private bridge accepts native model parameters and rejects host configuration", () => {
  const native = { effort: "high", thinking: { type: "enabled", budgetTokens: 1024, display: "omitted" } };
  assert.deepEqual(parseRequest(request(native)).harness_config, native);
  for (const config of [null, [], {env:{ANTHROPIC_API_KEY:"private-sentinel"}}, {thinking:{type:"adaptive",api_key:"private-sentinel"}}, {thinking:{type:"disabled",budgetTokens:1024}}, {effort:{}}, {maxThinkingTokens:1024}]) {
    assert.throws(() => parseRequest(request(config)), {message:"invalid_request"});
  }
});
