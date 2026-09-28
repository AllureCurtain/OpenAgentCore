import { createServer } from "node:http";
import { randomUUID } from "node:crypto";

let agents = [];
let sessions = [];
let histories = new Map();
let turns = new Map();
let receipts = new Map();
let loseCreation = false;
const now = () => Math.floor(Date.now() / 1000);
const page = (data) => ({
  object: "list",
  data,
  has_more: false,
  first_id: data[0]?.id || null,
  last_id: data.at(-1)?.id || null,
});
const message = (turn, role, text) => ({
  id: randomUUID(),
  turn_id: turn.id,
  type: "message",
  status: "completed",
  role,
  phase: role === "assistant" ? "final_answer" : null,
  content: [{ type: role === "user" ? "input_text" : "output_text", text }],
});

createServer(async (req, res) => {
  const url = new URL(req.url, "http://localhost");
  const chunks = [];
  for await (const chunk of req) chunks.push(chunk);
  const body = chunks.length ? JSON.parse(Buffer.concat(chunks)) : {};
  const reply = (value, status = 200) => {
    res.writeHead(status, { "Content-Type": "application/json" });
    res.end(JSON.stringify(value));
  };
  if (url.pathname === "/health") return reply({ fixture: true });
  if (url.pathname === "/reset") {
    agents = [];
    sessions = [];
    histories = new Map();
    turns = new Map();
    receipts = new Map();
    loseCreation = false;
    return reply({});
  }
  if (url.pathname === "/lose-creation") {
    loseCreation = true;
    return reply({});
  }
  if (url.pathname === "/counts")
    return reply({ agents: agents.length, sessions: sessions.length });
  if (req.headers.authorization !== "Bearer fixture-project-key")
    return reply({ error: { message: "Wrong project key" } }, 401);
  if (url.pathname === "/v1/agents") {
    if (req.method === "GET") return reply(page(agents));
    const agent = {
      id: randomUUID(),
      object: "agent",
      created_at: now(),
      updated_at: now(),
      name: null,
      instructions: null,
      metadata: {},
      multi_agent: { enabled: false, max_concurrent_subagents: null },
      reasoning: {},
      service_tier: "auto",
      text: { format: { type: "text" }, verbosity: "medium" },
      tools: [],
      ...body,
    };
    agent.text = { format: { type: "text" }, verbosity: "medium" };
    agent.multi_agent = { enabled: false, max_concurrent_subagents: null };
    agents.push(agent);
    return reply(agent, 201);
  }
  const agent = agents.find(
    (entry) => url.pathname === `/v1/agents/${entry.id}`,
  );
  if (agent) {
    if (req.method === "POST") Object.assign(agent, body);
    return reply(agent);
  }
  if (url.pathname === "/v1/agents/sessions") {
    if (req.method === "GET") return reply(page([...sessions].reverse()));
    const key = req.headers["idempotency-key"];
    if (receipts.has(key)) return reply(receipts.get(key));
    const saved = agents.find((entry) => entry.id === body.agent_id);
    const { object, created_at, updated_at, metadata, ...snapshot } = saved;
    const session = {
      id: randomUUID(),
      object: "agent.session",
      agent: snapshot,
      environment:
        body.environment.type === "none"
          ? { type: "none" }
          : {
              type: "openai_hosted",
              id: randomUUID(),
              capability_directories: [],
              network: { access: "enabled", allowed_domains: [] },
              packages: { npm: [], python: [], system: [] },
              files: [],
              plugins: [],
              skills: [],
            },
      status: "idle",
      error: null,
      metadata: body.metadata,
      required_actions: [],
      vault_ids: [],
      usage: null,
      created_at: now(),
      last_active_at: now(),
    };
    const turn = {
      id: randomUUID(),
      object: "agent.session.turn",
      session_id: session.id,
      agent_id: saved.id,
      subagent_id: null,
      status: "completed",
      created_at: now(),
      started_at: now(),
      completed_at: now(),
      error: null,
      usage: null,
    };
    histories.set(session.id, [
      message(turn, "user", body.input),
      {
        id: randomUUID(),
        turn_id: turn.id,
        type: "command_execution",
        status: "completed",
        command: "npm test",
        cwd: "/workspace",
        output: "12 tests passed",
        exit_code: 0,
        duration_ms: 512,
      },
      message(
        turn,
        "assistant",
        "已检查登录流程并补充验证。\n\n## 结果\n\n- 修复了会话过期后的跳转。\n- 12 项测试通过。\n\n```ts\nconst session = await restoreSession();\n```\n\n可以继续检查移动端表现。",
      ),
    ]);
    turns.set(session.id, [turn]);
    sessions.push(session);
    receipts.set(key, session);
    if (loseCreation) {
      loseCreation = false;
      return reply(
        { error: { message: "Fixture: creation response lost" } },
        502,
      );
    }
    return reply(session, 201);
  }
  const match = url.pathname.match(
    /^\/v1\/agents\/sessions\/([^/]+)(?:\/(items|turns|events))?$/,
  );
  const session = sessions.find((entry) => entry.id === match?.[1]);
  if (!session) return reply({ error: { message: "Not found" } }, 404);
  if (!match[2]) return reply(session);
  if (match[2] === "items") return reply(page(histories.get(session.id)));
  if (match[2] === "turns")
    return reply(page([...turns.get(session.id)].reverse()));
  const event = body.events[0];
  const key = req.headers["idempotency-key"];
  if (receipts.has(key)) return reply({}, 202);
  receipts.set(key, true);
  if (event.type === "agent.session.input.cancel") {
    session.status = "idle";
    const turn = turns.get(session.id).at(-1);
    turn.status = "cancelled";
    turn.completed_at = now();
  } else {
    session.status = "in_progress";
    const turn = {
      ...turns.get(session.id)[0],
      id: randomUUID(),
      status: "in_progress",
      completed_at: null,
    };
    turns.get(session.id).push(turn);
    histories
      .get(session.id)
      .push(message(turn, "user", event.input[0].content[0].text));
  }
  return reply({}, 202);
}).listen(18181, "127.0.0.1");
