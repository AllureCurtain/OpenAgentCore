import { test } from "node:test";
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { mkdir, mkdtemp, rm } from "node:fs/promises";
import { join } from "node:path";
import { homedir } from "node:os";
import { openStore, dataPath } from "../server/store.mjs";
import { productAPI } from "../server/product.mjs";

test("model, template and independent runtime-bound Agent persist without execution state", async (t) => {
  const root = join(homedir(), ".oac", "tests");
  await mkdir(root, { recursive: true });
  const dir = await mkdtemp(join(root, "parsar-store-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, "test.sqlite");
  let store = openStore(path);
  const calls = [];
  const core = async (path, method, body) => {
    calls.push({ path, method, body });
    return { id: randomUUID() };
  };
  const api = productAPI(store, core);
  const modelId = randomUUID(),
    runtimeId = randomUUID(),
    mcpId = randomUUID(),
    templateId = randomUUID(),
    instanceId = randomUUID();
  const put = (kind, id, body) => api("PUT", `/app/${kind}/${id}`, body);
  await put("models", modelId, { name: "Kimi", model: "kimi-k2.6" });
  await put("runtimes", runtimeId, {
    name: "Sandbox",
    environment: "openai_hosted",
  });
  await put("mcps", mcpId, {
    name: "Docs",
    label: "docs",
    url: "https://mcp.example/docs",
  });
  const template = await put("templates", templateId, {
    name: "Reviewer",
    model_id: modelId,
    harness: "claude_sdk",
    instructions: "Review carefully",
    mcp_ids: [mcpId],
    skill_ids: ["skill_review"],
  });
  const instance = await put("instances", instanceId, {
    name: "Alice",
    template_id: templateId,
    runtime_id: runtimeId,
  });
  assert.equal(calls.length, 2);
  assert.deepEqual(calls[0].body.skills, [
    { type: "skill_reference", skill_id: "skill_review" },
  ]);
  assert.equal(calls[1].body.model, "kimi-k2.6");
  assert.equal(
    calls[1].body.tools[1].transport.server_url,
    "https://mcp.example/docs",
  );
  assert.equal(instance.environment.type, "openai_hosted");
  assert.ok(instance.environment.environment_template_id);
  await put("templates", templateId, {
    ...template,
    instructions: "Different template",
  });
  assert.equal(
    (await api("GET", `/app/instances/${instanceId}`)).instructions,
    "Review carefully",
  );
  await assert.rejects(api("DELETE", `/app/models/${modelId}`), /仍被引用/);
  store.close();
  store = openStore(path);
  t.after(() => store.close());
  assert.equal(
    store.get("instances", instanceId).core_agent_id,
    instance.core_agent_id,
  );
  assert.equal(
    calls.some((call) => call.path.includes("/sessions")),
    false,
  );
});
test("runtime compatibility, MCP URLs and store isolation are explicit", async (t) => {
  const store = openStore(":memory:");
  t.after(() => store.close());
  const api = productAPI(store, () => {
    throw new Error("must not call Core");
  });
  const model_id = randomUUID(),
    runtime_id = randomUUID(),
    template_id = randomUUID();
  await api("PUT", `/app/models/${model_id}`, {
    name: "Model",
    model: "model",
  });
  await api("PUT", `/app/runtimes/${runtime_id}`, {
    name: "Text",
    environment: "none",
  });
  await api("PUT", `/app/templates/${template_id}`, {
    name: "Template",
    model_id,
    harness: "claude_sdk",
    instructions: "",
    mcp_ids: [],
    skill_ids: ["skill_review"],
  });
  await assert.rejects(
    api("PUT", `/app/instances/${randomUUID()}`, {
      name: "Agent",
      template_id,
      runtime_id,
    }),
    /Skills 需要/,
  );
  await assert.rejects(
    api("PUT", `/app/mcps/${randomUUID()}`, {
      name: "MCP",
      label: "docs",
      url: "https://secret@example.com/mcp",
    }),
    /不含凭据/,
  );
  assert.notEqual(
    dataPath({ target: "https://core.example", key: "a" }),
    dataPath({ target: "https://core.example", key: "b" }),
  );
});
