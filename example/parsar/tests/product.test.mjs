import { test } from "node:test";
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { mkdir, mkdtemp, rm } from "node:fs/promises";
import { join } from "node:path";
import { homedir } from "node:os";
import { unzipSync, strFromU8 } from "fflate";
import { openStore, dataPath, AppError } from "../server/store.mjs";
import { productAPI } from "../server/product.mjs";

async function setup(store, core) {
  const api = productAPI(store, core);
  const put = (kind, body, id = randomUUID()) =>
    api("PUT", `/app/${kind}/${id}`, body);
  const provider = await put("providers", { name: "Moonshot" });
  const model = await put("models", {
    name: "Kimi",
    model: "kimi-k2.6",
    provider_id: provider.id,
  });
  const runtime = await put("runtimes", {
    name: "Sandbox",
    environment: "openai_hosted",
  });
  const mcp = await put("mcps", {
    name: "Docs",
    label: "docs",
    url: "https://mcp.example/docs",
  });
  const agent = await put("agents", {
    name: "Reviewer",
    model_id: model.id,
    harness: "claude_sdk",
    instructions: "Review carefully",
    mcp_ids: [mcp.id],
    skill_ids: ["skill_review"],
  });
  return {
    api,
    put,
    model,
    runtime,
    agent,
    input: {
      name: "Review",
      agent_id: agent.id,
      runtime_id: runtime.id,
      input: "Review this",
    },
  };
}
test("one Agent creates independent Sessions; edits do not mutate prior execution; retries survive restart", async (t) => {
  const root = join(homedir(), ".oac", "tests");
  await mkdir(root, { recursive: true });
  const dir = await mkdtemp(join(root, "parsar-store-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, "test.sqlite");
  let store = openStore(path);
  const calls = [],
    receipts = new Map();
  let lose = false;
  const core = async (path, method, body, key) => {
    calls.push({ path, body, key });
    if (!receipts.has(key)) receipts.set(key, { id: randomUUID() });
    if (lose) {
      lose = false;
      throw new AppError(502, "lost response");
    }
    return receipts.get(key);
  };
  const { api, put, agent, model, input } = await setup(store, core);
  assert.equal(calls.length, 0);
  const first = await put("sessions", input);
  assert.deepEqual(calls[0].body.environment.skills, [
    { type: "skill_reference", skill_id: "skill_review" },
  ]);
  assert.equal(calls[0].body.agent.tools.length, 1);
  const zip = unzipSync(
    Buffer.from(calls[0].body.environment.plugins[0].source.data, "base64"),
  );
  assert.equal(
    JSON.parse(strFromU8(zip["agent/.mcp.json"])).mcpServers.docs.url,
    "https://mcp.example/docs",
  );
  await put("agents", { ...agent, instructions: "Updated" }, agent.id);
  const second = await put("sessions", input);
  assert.notEqual(first.core_session_id, second.core_session_id);
  assert.equal(calls[0].body.agent.instructions, "Review carefully");
  assert.equal(calls[1].body.agent.instructions, "Updated");
  await assert.rejects(api("DELETE", `/app/models/${model.id}`), /仍被引用/);
  await assert.rejects(api("DELETE", `/app/agents/${agent.id}`), /仍被引用/);
  const pending = randomUUID();
  lose = true;
  await assert.rejects(put("sessions", input, pending), /lost response/);
  const frozen = calls.at(-1).body;
  store.close();
  store = openStore(path);
  t.after(() => store.close());
  const resumed = await productAPI(store, core)(
    "PUT",
    `/app/sessions/${pending}`,
    {},
  );
  assert.equal(resumed.core_session_id, receipts.get(pending).id);
  assert.deepEqual(calls.at(-1).body, frozen);
  assert.equal(store.get("sessions", pending).request, undefined);
  assert.equal(receipts.size, 3);
});
test("compatibility validation precedes execution, and old configurations migrate without fake Sessions", async (t) => {
  const store = openStore(":memory:");
  t.after(() => store.close());
  const { api, put, agent, input } = await setup(store, () => {
    throw new Error("must not execute");
  });
  const runtime = await put("runtimes", { name: "Text", environment: "none" });
  await assert.rejects(
    put("sessions", { ...input, runtime_id: runtime.id }),
    /Skills 需要/,
  );
  await assert.rejects(
    put("mcps", {
      name: "MCP",
      label: "docs",
      url: "https://secret@example.com/mcp",
    }),
    /不含凭据/,
  );
  const legacy = { ...agent, id: randomUUID(), runtime_id: runtime.id };
  store.put("instances", legacy);
  productAPI(store, () => {});
  assert.equal(store.get("agents", legacy.id).name, agent.name);
  assert.equal(store.get("agents", legacy.id).runtime_id, undefined);
  assert.equal(store.list("sessions").length, 0);
  assert.equal(store.list("instances").length, 0);
  assert.notEqual(
    dataPath({ target: "https://core.example", key: "a" }),
    dataPath({ target: "https://core.example", key: "b" }),
  );
});

test("Providers are explicit groups with many models, guarded deletion and no automatic defaults", async (t) => {
  const store = openStore(":memory:");
  t.after(() => store.close());
  const api = productAPI(store, () => {});
  assert.deepEqual(await api("GET", "/app/providers"), []);
  const id = randomUUID();
  const provider = await api("PUT", `/app/providers/${id}`, { name: "Vendor" });
  for (const model of ["model-a", "model-b"])
    await api("PUT", `/app/models/${randomUUID()}`, {
      name: model,
      model,
      provider_id: id,
    });
  assert.equal((await api("GET", "/app/models")).length, 2);
  await assert.rejects(api("DELETE", `/app/providers/${id}`), /仍被引用/);
  await api("PUT", `/app/providers/${id}`, { ...provider, name: "Renamed" });
  assert.ok(store.list("models").every((m) => m.provider_id === id));
  await assert.rejects(
    api("PUT", `/app/models/${randomUUID()}`, { name: "Missing", model: "x" }),
    /有效的资源/,
  );
});
