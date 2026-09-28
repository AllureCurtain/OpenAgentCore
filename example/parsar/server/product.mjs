import { AppError, text, uuid } from "./store.mjs";

const kinds = new Set(["models", "mcps", "runtimes", "templates", "instances"]);
export function productAPI(store, core) {
  const requireReference = (kind, id) => {
    if (typeof id !== "string" || !store.get(kind, id))
      throw new AppError(400, "请选择有效的资源。");
    return id;
  };
  const references = (kind, ids) => {
    if (!Array.isArray(ids) || ids.length > 8)
      throw new AppError(400, "最多选择 8 个资源。");
    return [...new Set(ids.map((id) => requireReference(kind, id)))];
  };
  return async (method, path, body) => {
    const match = path.match(/^\/app\/([a-z]+)(?:\/([a-f0-9-]{36}))?$/);
    if (!match || !kinds.has(match[1]) || (match[2] && !uuid.test(match[2])))
      throw new AppError(404, "Not found.");
    const [, kind, id] = match;
    if (method === "GET") {
      if (!id) return store.list(kind);
      const value = store.get(kind, id);
      if (!value) throw new AppError(404, "记录不存在。");
      return value;
    }
    if (!id) throw new AppError(405, "Method not allowed.");
    const previous = store.get(kind, id);
    if (method === "DELETE") {
      const used =
        store
          .list("templates")
          .some((row) => row.model_id === id || row.mcp_ids.includes(id)) ||
        store
          .list("instances")
          .some(
            (row) =>
              row.template_id === id ||
              row.runtime_id === id ||
              row.model_id === id ||
              row.mcp_ids.includes(id),
          );
      if (used) throw new AppError(409, "资源仍被引用，请先调整绑定。");
      if (kind === "instances" && previous)
        await core(`/v1/agents/${previous.core_agent_id}`, "DELETE");
      store.remove(kind, id);
      return {};
    }
    if (method !== "PUT") throw new AppError(405, "Method not allowed.");
    if (previous && body.revision !== previous.revision)
      throw new AppError(409, "配置已更新，请重新打开后编辑。");
    const value = {
      id,
      name: text(body.name, "名称", 80, true),
      revision: (previous?.revision || 0) + 1,
    };
    if (kind === "models") value.model = text(body.model, "模型 ID", 200, true);
    if (kind === "mcps") {
      let url;
      try {
        url = new URL(body.url);
      } catch {
        throw new AppError(400, "请填写有效的 MCP URL。");
      }
      if (
        url.protocol !== "https:" ||
        url.username ||
        url.password ||
        url.hash ||
        url.search
      )
        throw new AppError(400, "MCP 使用不含凭据的 HTTPS 地址。");
      value.url = url.href;
      value.label = text(body.label, "MCP 标识", 64, true);
      if (!/^[a-zA-Z0-9_-]+$/.test(value.label))
        throw new AppError(400, "MCP 标识仅支持字母、数字、下划线和连字符。");
    }
    if (kind === "runtimes") {
      if (!["openai_hosted", "none"].includes(body.environment))
        throw new AppError(400, "运行环境无效。");
      value.environment = body.environment;
    }
    if (kind === "templates" || kind === "instances") {
      if (kind === "instances") {
        value.template_id = requireReference("templates", body.template_id);
        if (!body.model_id)
          body = { ...store.get("templates", value.template_id), ...body };
      }
      value.model_id = requireReference("models", body.model_id);
      if (!["codex", "claude_sdk", "mcode"].includes(body.harness))
        throw new AppError(400, "请选择执行引擎。");
      value.harness = body.harness;
      value.instructions = text(body.instructions, "指令", 100000);
      value.mcp_ids = references("mcps", body.mcp_ids);
      if (
        !Array.isArray(body.skill_ids) ||
        body.skill_ids.length > 8 ||
        body.skill_ids.some(
          (id) =>
            typeof id !== "string" ||
            !/^skill_[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(id),
        )
      )
        throw new AppError(400, "技能配置无效。");
      value.skill_ids = [...new Set(body.skill_ids)];
    }
    if (kind === "instances") {
      value.template_id = requireReference("templates", body.template_id);
      value.runtime_id = requireReference("runtimes", body.runtime_id);
      const runtime = store.get("runtimes", value.runtime_id);
      if (runtime.environment === "none" && value.skill_ids.length)
        throw new AppError(400, "Skills 需要托管运行环境。");
      const model = store.get("models", value.model_id);
      const tools = value.mcp_ids.map((id) => {
        const mcp = store.get("mcps", id);
        return {
          type: "mcp",
          server_label: mcp.label,
          transport: { type: "http", server_url: mcp.url },
          connection_origin: "service",
          required: true,
        };
      });
      if (new Set(tools.map((tool) => tool.server_label)).size !== tools.length)
        throw new AppError(400, "绑定的 MCP 服务标识不能重复。");
      const environment = { type: runtime.environment };
      if (value.skill_ids.length) {
        const same =
          previous &&
          previous.runtime_id === value.runtime_id &&
          JSON.stringify(previous.skill_ids) ===
            JSON.stringify(value.skill_ids);
        if (same && previous.environment.environment_template_id)
          environment.environment_template_id =
            previous.environment.environment_template_id;
        else {
          const template = await core(
            "/v1/agents/environments/templates",
            "POST",
            {
              name: value.name,
              skills: value.skill_ids.map((skill_id) => ({
                type: "skill_reference",
                skill_id,
              })),
            },
          );
          if (!uuid.test(template.id))
            throw new AppError(502, "Core 返回了无效的技能模板。");
          environment.environment_template_id = template.id;
        }
      }
      const agent = await core(
        previous ? `/v1/agents/${previous.core_agent_id}` : "/v1/agents",
        "POST",
        {
          name: value.name,
          model: model.model,
          instructions: value.instructions,
          x_agents_core: { harness: value.harness },
          metadata: { application: "parsar-example", instance_id: id },
          multi_agent: { enabled: false },
          text: { verbosity: "medium" },
          tools: [{ type: "web_search", mode: "disabled" }, ...tools],
        },
      );
      if (!uuid.test(agent.id))
        throw new AppError(502, "Core 返回了无效的 Agent。");
      value.core_agent_id = agent.id;
      value.environment = environment;
      value.model = model.model;
      value.tools = tools;
    }
    store.put(kind, value);
    return value;
  };
}
