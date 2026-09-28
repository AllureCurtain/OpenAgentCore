import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer, request } from "node:http";
import { configuration, createHandler } from "../server.mjs";

test("oversized writes return a definite 413 without contacting Core", async (t) => {
  let called = false;
  const url = await serve(t, () => {
    called = true;
    return Response.json({});
  });
  const response = await fetch(`${url}/v1/agents/sessions`, {
    method: "POST",
    headers: { origin: url },
    body: "x".repeat(1024 * 1024 + 1),
  });
  assert.equal(response.status, 413);
  assert.equal(called, false);
});

async function serve(t, fetchImpl) {
  const config = {
    port: 0,
    key: "server-only-secret",
    target: "https://core.example",
  };
  const server = createServer(createHandler(config, { fetchImpl }));
  await new Promise((done) => server.listen(0, "127.0.0.1", done));
  config.port = server.address().port;
  t.after(() => {
    server.closeAllConnections();
    server.close();
  });
  return `http://127.0.0.1:${config.port}`;
}

test("server injects only the project key and preserves request identity", async (t) => {
  let called = false;
  const url = await serve(t, async (target, init) => {
    called = true;
    assert.equal(target, "https://core.example/v1/agents/sessions");
    assert.equal(init.headers.Authorization, "Bearer server-only-secret");
    assert.equal(init.headers["Idempotency-Key"], "same-operation");
    assert.equal(init.headers["OpenAI-Beta"], "agents=v1");
    assert.equal(init.redirect, "manual");
    assert.equal(init.headers.cookie, undefined);
    return Response.json({ id: "session" }, { status: 201 });
  });
  const result = await fetch(`${url}/v1/agents/sessions`, {
    method: "POST",
    body: "{}",
    headers: {
      origin: url,
      authorization: "Bearer browser-secret",
      cookie: "secret=cookie",
      "idempotency-key": "same-operation",
    },
  });
  assert.equal(result.status, 201);
  assert.equal(called, true);
  assert.equal((await result.text()).includes("secret"), false);
});

test("cross-origin, rebinding, management and encoded paths never reach Core", async (t) => {
  const url = await serve(t, () => {
    throw new Error("must not reach Core");
  });
  for (const [path, headers, method, status] of [
    ["/v1/agents", { origin: "https://evil.example" }, "GET", 403],
    ["/v1/agents", { host: "evil.example" }, "GET", 403],
    ["/v1/agents", { "sec-fetch-site": "cross-site" }, "GET", 403],
    ["/v1/agents", {}, "POST", 403],
    ["/core/v1/projects", {}, "GET", 404],
    ["/api/v1/devices", {}, "GET", 404],
    ["/v1/agents%2fsessions", {}, "GET", 404],
    ["/v1/agents", {}, "DELETE", 404],
  ]) {
    const received = await new Promise((done, reject) => {
      const req = request(`${url}${path}`, { method, headers }, (res) => {
        res.resume();
        done(res.statusCode);
      });
      req.on("error", reject);
      req.end();
    });
    assert.equal(
      received,
      status,
      `${method} ${path} ${JSON.stringify(headers)}`,
    );
  }
});

test("redirects and transport errors do not expose credentials or follow another host", async (t) => {
  for (const reply of [
    () =>
      new Response(null, {
        status: 302,
        headers: { location: "https://evil.example" },
      }),
    () => {
      throw new Error("server-only-secret");
    },
  ]) {
    const url = await serve(t, reply);
    const result = await fetch(`${url}/v1/agents`);
    assert.equal(result.status, 502);
    assert.equal((await result.text()).includes("server-only-secret"), false);
  }
});

test("configuration keeps remote credentials on HTTPS and requires a project key", () => {
  for (const target of [
    "http://remote.example",
    "https://user:pass@core.example",
    "https://core.example/v1",
    "https://core.example/?token=x",
  ]) {
    assert.throws(() =>
      configuration({
        OAC_EXAMPLE_CORE_URL: target,
        OAC_EXAMPLE_PROJECT_KEY: "key",
      }),
    );
  }
  assert.throws(() => configuration({}));
  assert.equal(
    configuration({ OAC_EXAMPLE_PROJECT_KEY: "key" }).target,
    "http://127.0.0.1:8091",
  );
});
