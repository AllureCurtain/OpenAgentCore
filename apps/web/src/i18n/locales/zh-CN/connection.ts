import type { connection as english } from "../en/connection";

type TranslationShape<T> = { [K in keyof T]: T[K] extends string ? string : TranslationShape<T[K]> };

export const connection: TranslationShape<typeof english> = {
  title: "连接 Agent Core", apply: "应用连接", mode: "连接模式", localCore: "本地 Parsar Core", localDefault: "默认 · 无需输入", compatibleCore: "其他兼容 Core", advanced: "高级",
  localProxy: "本地 `/v1` 代理", fixedBase: "Agents API 地址由此应用固定，无需填写 URL。", apiBase: "API 地址", authentication: "身份验证", keyDetected: "已检测到服务端管理的密钥", keyMissing: "未检测到服务端管理的密钥",
  proxySecure: "Vite 代理在服务端附加密钥，浏览器 JavaScript 不会接触 bearer 凭据。", proxyConfigure: "配置本地代理令牌文件并重启 Web。本地模式不会要求浏览器令牌。",
  compatibleUrl: "兼容 Core 的基础 URL", urlHelp: "远程 Core 必须使用 HTTPS；只有明确的回环地址可使用 HTTP。Core 必须通过 CORS 允许此 Web 来源、GET/POST、Authorization 与 OpenAI-Beta。", urlError: "请输入不含凭据、查询参数或片段的 HTTPS URL，或 HTTP 回环 URL。",
  bearerToken: "Bearer 令牌", tabOnly: "仅当前标签页", tokenPlaceholder: "兼容 Core 的调用方令牌", tokenHelp: "仅用于直连回退。令牌只保存在当前标签页的 sessionStorage 中，不会进入 localStorage、URL 或服务端代理配置。",
  testTitle: "连接测试", testHelp: "通过一次 GET 检查已认证的 Agents API 访问；不会创建 Agent、Session、Turn 或 Item。", testing: "测试中…", test: "测试连接", testingTitle: "正在测试 Core 连接…", testingDetail: "正在发送一次只读 Agents API GET 请求。", probeBoundary: "聊天使用当前 Agents API 合约。此只读探测不会启动 Turn，也不会验证运行时依赖。",
  operatorSetup: "运维侧配置", operatorHelp: "Web 可以配置并测试现有连接，但不会启动 Docker 或宿主机进程。Core、代理、调用方密钥、daemon、executor 与 provider 请按运维指南配置。", webGuide: "Web 连接指南 · 当前仓库", troubleshooting: "连接故障排查 · 当前仓库", coreSetup: "Parsar Core 配置", finalBoundary: "连接测试仅验证 Agents API 访问。实际请求仍可能因 worker、executor、模型或 provider 不可用而失败。",
  copy: "复制", copied: "已复制", copyLabel: "复制{{label}}", dockerTitle: "启动本地 Docker 后端", dockerIntro: "请在 Docker 主机的终端运行这些命令。Web 只展示已审核命令，不会获得 Docker socket 权限或执行命令。",
  existingTitle: "此电脑已完成配置", existingSubtitle: "启动现有容器", databaseStart: "启动专用数据库", databaseWait: "等待 Docker 健康状态变为 healthy。", coreDaemonStart: "启动 Core API 与 daemon", daemonReconnects: "daemon 会独立重新连接 Core。", verifyHealth: "验证 Core 进程健康", healthBoundary: "健康响应只代表存活；下一步请使用连接测试。",
  firstTime: "首次在此电脑配置", firstTimeSubtitle: "尝试 docker start 前先创建 Core", dockerAssumption: "假定 Docker 已安装。Parsar Core 仍需要专用 PostgreSQL 数据库、私有调用方主体、迁移、Core API 容器和已配置的 daemon profile。Core 未提供安全的零输入引导，因此 Web 不会猜测凭据或容器设置。", buildImage: "构建 Core 镜像", reviewedCheckout: "在已审核的 parsar-core checkout 中运行。", createStack: "创建私有 Core 栈", createStackHelp: "按照仓库容器指南准备专用数据库、调用方密钥文件、迁移和 API 容器。", provisionDaemon: "配置并连接 daemon", provisionDaemonHelp: "签发独立的设备 profile；调用方密钥与 daemon 凭据不能互换。", returnTest: "返回此处测试", returnTestHelp: "配置 Web 服务端调用方密钥文件，重启 Web，然后测试连接。", containerSetup: "Core 容器配置", daemonProvisioning: "Daemon 配置", proxySetup: "Web 代理与调用方密钥配置", dockerUnconfigured: "此 Web 构建未配置 Docker 启动指南。", dockerUnconfiguredHelp: "从 .env.example 设置非敏感的 AGENTS_CORE_WEB_DOCKER_BACKEND_* 值后重启 Web。容器名属于运维配置，浏览器不会猜测。", dockerBoundary: "现有栈命令只会启动已保存的数据库、Core 与 daemon 容器。首次配置会创建持久状态和凭据，仍由运维人员完成。自托管 executor 属于具体 Session，须按该 Session 的 Environment 指引连接。",
  result: {
    authenticatedTitle: "Core API 认证成功", authenticatedDetail: "只读 Agents API 请求返回了有效集合。", invalidTitle: "Core URL 已拦截", invalidDetail: "远程 Core URL 必须使用 HTTPS；只有明确的回环主机可使用 HTTP。", unauthorizedTitle: "认证失败", unauthorizedDetail: "Core 拒绝了调用方密钥（HTTP 401）。请检查服务端管理的调用方密钥或当前标签页令牌。", mismatchTitle: "Agents API 协议不匹配", mismatchDetail: "端点不接受所测试的 /v1/agents GET 合约或 OpenAI-Beta: agents=v1 请求头。", httpTitle: "Core 返回 HTTP {{status}}", httpDetail: "基础 Agents API 读取未成功，未修改任何 Core 资源。", unreachableTitle: "无法连接 Core", unreachableDetail: "浏览器无法连接 Core。请检查本地代理目标、网络或直连模式的 CORS 策略。",
  },
};
