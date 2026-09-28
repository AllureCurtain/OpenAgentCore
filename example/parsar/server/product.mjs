import { sessionAPI } from "./sessions.mjs";
import { AppError, text, uuid } from "./store.mjs";

const kinds = new Set([
  "providers",
  "models",
  "mcps",
  "runtimes",
  "agents",
  "sessions",
]);
export function productAPI(store, core) {
  // Preserve earlier example configurations without fabricating execution records.
  for (const kind of ["templates", "instances"]) {
    for (const row of store.list(kind)) {
      if (!store.get("agents", row.id)) {
        const {
          id,
          name,
          revision,
          model_id,
          harness,
          instructions,
          skill_ids,
          mcp_ids,
        } = row;
        store.put("agents", {
          id,
          name,
          revision,
          model_id,
          harness,
          instructions,
          skill_ids,
          mcp_ids,
        });
      }
      store.remove(kind, row.id);
    }
  }
  const sessions = sessionAPI(store, core);
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
    if (kind === "sessions") {
      if (method !== "PUT") throw new AppError(405, "Method not allowed.");
      return sessions(id, body);
    }
    const previous = store.get(kind, id);
    if (method === "DELETE") {
      const used =
        store.list("models").some((row) => row.provider_id === id) ||
        store
          .list("agents")
          .some((row) => row.model_id === id || row.mcp_ids.includes(id)) ||
        store
          .list("sessions")
          .some((row) => row.agent_id === id || row.runtime_id === id);
      if (used) throw new AppError(409, "资源仍被引用，请先调整绑定。");
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
    if (kind === "models") {
      value.model = text(body.model, "模型 ID", 200, true);
      value.provider_id = requireReference("providers", body.provider_id);
    }
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
    if (kind === "agents") {
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
    store.put(kind, value);
    return value;
  };
}
