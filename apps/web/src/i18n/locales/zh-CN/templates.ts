import type { templates as english } from "../en/templates";
type TranslationShape<T> = { [K in keyof T]: T[K] extends string ? string : TranslationShape<T[K]> };
export const templates: TranslationShape<typeof english> = {
  boundary: "管理基础模板的名称和网络访问。启动会话时可选择已保存的模板。保存配置不会启动运行时。",
  refreshFailed: "无法刷新目录。请检查连接后重试。",
  savedRefreshFailed: "更改已保存，但目录刷新失败。请先刷新再进行其他更改。",
  created: "模板已创建，未分配运行时。", updated: "模板已更新。现有会话会保留原配置。", deleted: "模板已删除。",
  loading: "正在加载环境模板…", unavailableTitle: "环境模板不可用", unavailableDescription: "此 Core 未提供模板资源，Web 不会推断目录。",
  loadFailedTitle: "无法加载环境模板", loadFailedDescription: "请检查 Core 连接、访问权限和支持的模板配置；目录当前不可用。", configureConnection: "配置连接",
  filterLabel: "筛选模板", filterPlaceholder: "按名称、ID 或网络筛选", count: "显示 {{visible}} / {{total}} 个模板",
  emptyTitle: "暂无环境模板", emptyDescription: "创建可复用配置后，即可在启动会话时选择。", noMatch: "没有匹配的模板", clearFilter: "清除筛选", listLabel: "环境模板",
  updatedAt: "更新于 {{date}}", network: "网络", allowedDomains: "允许的域名", enabled: "已启用", disabled: "已禁用", edit: "编辑", editLabel: "编辑 {{name}}", delete: "删除", deleteLabel: "删除 {{name}}",
  modal: { delete: "删除模板？", edit: "编辑模板", create: "创建模板" }, deletePrompt: "删除 {{name}}？", deleteWarning: "新会话将无法再选择此模板。现有会话会保留原配置。此操作无法撤销。", deleting: "删除中…", deleteTemplate: "删除模板", unnamed: "未命名模板",
  form: { name: "名称", optional: "可选", namePlaceholder: "例如：受限网络", nameTooLong: "名称最多可包含 256 个字符。", networkAccess: "网络访问", policyLocked: "Web 无法在不替换允许域名的情况下编辑此网络策略。", sessionBoundary: "在使用此模板创建新会话时生效。现有会话会保留原配置。", saving: "保存中…", saveChanges: "保存更改", create: "创建模板" },
  errors: { policyPreserved: "Web 无法编辑此网络策略，必须保留其允许域名。", forbidden: "Core 拒绝了此请求。请检查连接和访问权限。", missing: "此模板已不存在。请刷新目录后重试。", invalid: "Core 拒绝了模板配置。请刷新目录并检查各字段。", uncertain: "未能确认写入结果。再次更改前请刷新并检查 Core 目录；请求未被重放。" },
};
