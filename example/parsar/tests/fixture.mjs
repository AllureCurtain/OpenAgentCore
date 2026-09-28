import { createServer } from "node:http";
import { randomUUID } from "node:crypto";
let agents = [],
  templates = [],
  skills = [],
  versions = new Map();
const now = () => Math.floor(Date.now() / 1000);
const page = (data) => ({
  object: "list",
  data,
  has_more: false,
  first_id: data[0]?.id || null,
  last_id: data.at(-1)?.id || null,
});
createServer(async (req, res) => {
  const url = new URL(req.url, "http://localhost");
  const chunks = [];
  for await (const chunk of req) chunks.push(chunk);
  const bytes = Buffer.concat(chunks);
  const reply = (value, status = 200) => {
    res.writeHead(status, { "Content-Type": "application/json" });
    res.end(JSON.stringify(value));
  };
  if (url.pathname === "/health") return reply({ fixture: true });
  if (url.pathname === "/reset") {
    agents = [];
    templates = [];
    skills = [];
    versions = new Map();
    return reply({});
  }
  if (url.pathname === "/counts")
    return reply({ agents, templates, skills, sessions: 0 });
  if (req.headers.authorization !== "Bearer fixture-project-key")
    return reply({ error: { message: "Wrong project key" } }, 401);
  let body = {};
  if (
    bytes.length &&
    req.headers["content-type"]?.startsWith("application/json")
  )
    body = JSON.parse(bytes);
  if (url.pathname === "/v1/agents" && req.method === "POST") {
    const agent = {
      id: randomUUID(),
      object: "agent",
      created_at: now(),
      updated_at: now(),
      ...body,
    };
    agents.push(agent);
    return reply(agent, 201);
  }
  const agent = agents.find((a) => url.pathname === `/v1/agents/${a.id}`);
  if (agent) {
    if (req.method === "DELETE") {
      agents = agents.filter((row) => row !== agent);
      return reply({ id: agent.id, object: "agent.deleted", deleted: true });
    }
    if (req.method === "POST") Object.assign(agent, body);
    return reply(agent);
  }
  if (
    url.pathname === "/v1/agents/environments/templates" &&
    req.method === "POST"
  ) {
    const template = { id: randomUUID(), ...body };
    templates.push(template);
    return reply(template, 201);
  }
  if (url.pathname === "/v1/skills" && req.method === "GET")
    return reply(page(skills));
  const versionMatch = url.pathname.match(/^\/v1\/skills\/([^/]+)\/versions$/);
  const skillMatch = url.pathname.match(/^\/v1\/skills\/([^/]+)$/);
  if (
    req.method === "POST" &&
    (url.pathname === "/v1/skills" || versionMatch)
  ) {
    if (req.headers["openai-beta"])
      return reply(
        { error: { message: "Skills must not use Beta header" } },
        400,
      );
    const form = await new Request("http://fixture", {
      method: "POST",
      headers: { "Content-Type": req.headers["content-type"] },
      body: bytes,
    }).formData();
    const file = form.get("files[]") || form.get("files");
    if (!file) return reply({ error: { message: "Missing Skill file" } }, 400);
    const content = await file.text();
    const name = content.match(/name: "([^"]+)"/)?.[1] || "review-skill";
    const description =
      content.match(/description: "([^"]+)"/)?.[1] || "Review code";
    let skill = skills.find((s) => s.id === versionMatch?.[1]);
    if (!skill) {
      skill = {
        id: `skill_${randomUUID().replaceAll("-", "")}`,
        object: "skill",
        created_at: now(),
        name,
        description,
        default_version: "1",
        latest_version: "1",
      };
      skills.push(skill);
      versions.set(skill.id, []);
    }
    const number = String(versions.get(skill.id).length + 1);
    const version = {
      id: `skillver_${randomUUID().replaceAll("-", "")}`,
      object: "skill.version",
      skill_id: skill.id,
      version: number,
      created_at: now(),
      name,
      description,
    };
    versions.get(skill.id).push(version);
    skill.latest_version = number;
    if (form.get("default") === "true") {
      skill.default_version = number;
      skill.name = name;
      skill.description = description;
    }
    return reply(versionMatch ? version : skill);
  }
  if (versionMatch) return reply(page(versions.get(versionMatch[1]) || []));
  const skill = skills.find((s) => s.id === skillMatch?.[1]);
  if (skill) {
    if (req.method === "DELETE") {
      skills = skills.filter((s) => s !== skill);
      return reply({ id: skill.id, object: "skill.deleted", deleted: true });
    }
    if (req.method === "POST") {
      skill.default_version = body.default_version;
    }
    return reply(skill);
  }
  return reply({ error: { message: "Not found" } }, 404);
}).listen(18181, "127.0.0.1");
