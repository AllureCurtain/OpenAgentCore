// Landing page copy. Every product claim here is backed by a page in docs/ or
// contracts/; each section links to the page that owns the detail.

export type Lang = 'en' | 'zh'

export interface LandingCopy {
  hero: {
    eyebrow: string
    title: [string, string]
    lede: string
    primary: string
    secondary: string
    installLabel: string
    copied: string
    copy: string
    hud: [string, string][]
  }
  ticker: string[]
  problem: {
    index: string
    kicker: string
    title: string
    lede: string
    questions: { q: string; a: string }[]
    punch: string
  }
  compose: {
    index: string
    kicker: string
    title: string
    lede: string
    harness: string
    protocol: string
    environment: string
    accepted: string
    rejected: (harness: string, protocols: string) => string
    envs: Record<'openai_hosted' | 'self_hosted', { label: string; note: string }>
    input: string
    footnote: string
    footnoteLink: string
  }
  pillars: { tag: string; title: string; body: string }[]
  architecture: {
    index: string
    kicker: string
    title: string
    lede: string
    tabs: { id: string; label: string; alt: string }[]
    boundariesTitle: string
    boundaries: { from: string; to: string; doc: string; link: string }[]
    more: string
  }
  session: {
    index: string
    kicker: string
    title: string
    lede: string
    rules: { title: string; body: string; art: string }[]
    honest: string
  }
  observe: {
    index: string
    kicker: string
    title: string
    lede: string
    tabs: { id: 'overview' | 'metrics'; label: string; alt: string }[]
    caption: string
  }
  compare: {
    index: string
    kicker: string
    title: string
    head: [string, string, string]
    rows: [string, string, string][]
  }
  tradeoffs: {
    index: string
    kicker: string
    title: string
    motto: [string, string]
    items: { title: string; body: string }[]
  }
  start: {
    index: string
    kicker: string
    title: string
    lede: string
    steps: { title: string; body: string }[]
    cta: string
    trial: string
  }
  roadmap: {
    index: string
    kicker: string
    title: string
    lede: string
    tracks: { layer: string; title: string; body: string }[]
  }
  docs: {
    index: string
    kicker: string
    title: string
    lede: (pages: number) => string
    open: string
  }
  outro: { line: string; primary: string; secondary: string }
}

const en: LandingCopy = {
  hero: {
    eyebrow: 'open source · self-hosted · OpenAI Agents API',
    title: ['One core.', 'Many agents.'],
    lede: 'An open-source, self-hosted implementation of the OpenAI Agents API. Run Codex, Claude Code or MiniMax Code with your own models, on your own machines, through the official OpenAI SDK.',
    primary: 'Run your first Session',
    secondary: 'Read the docs',
    installLabel: 'Linux amd64 · Docker · Python 3.9+',
    copied: 'copied',
    copy: 'copy',
    hud: [
      ['api', '/v1 · OpenAI Agents API'],
      ['harness', 'codex | claude_sdk | mcode'],
      ['protocol', 'responses | anthropic | chat_completions'],
      ['sandbox', 'docker | microsandbox | e2b'],
      ['machine', 'linux | macos | windows'],
    ],
  },
  ticker: ['Codex', 'Claude Code', 'MiniMax Code', 'Responses API', 'Anthropic Messages', 'Chat Completions', 'Docker', 'microsandbox', 'E2B', 'Linux', 'macOS', 'Windows', 'PostgreSQL', 'OpenAI SDK'],
  problem: {
    index: '01',
    kicker: 'the missing layer',
    title: 'Calling a model is solved. Letting an agent do the work is not.',
    lede: 'An agent reads files, runs commands, waits for tests and asks for approval. A single model response is one step of that. Put it inside a product and a new set of questions appears.',
    questions: [
      { q: 'The user closed the tab. What happens to the work already submitted?', a: 'A dropped connection is an observer leaving. The Turn keeps running; read its durable state.' },
      { q: 'The request timed out. Send it again, or look up the original?', a: 'Poll durable state. Supported submission paths are idempotent, so a lost response does not duplicate work.' },
      { q: 'The user pressed Stop. Did the output stop, or the agent and everything it started?', a: 'Cancellation has a receipt: Core confirms the Turn\'s execution and cleanup state.' },
      { q: 'Which model and which machine ran this? Where are the files and the log?', a: 'Sessions, Turns, Items, artifacts and usage are stored by Core and queryable through the API.' },
      { q: 'Switching from Codex to Claude Code, or Docker to E2B. How much product code changes?', a: 'Your code keeps the same API calls. The harness is a Session setting with a matching model provider, and the sandbox is the administrator\'s choice, behind a Sandbox Provider.' },
    ],
    punch: 'Between an agent that works in a terminal and an agent a product can call, there is a whole layer of engineering. OpenAgentCore is that layer.',
  },
  compose: {
    index: '02',
    kicker: 'pick each part',
    title: 'Choose the model, the agent and the machine separately.',
    lede: 'They are three different decisions. Each Session names its harness, its model provider and its Environment; Core validates the combination before anything runs.',
    harness: 'harness',
    protocol: 'model protocol',
    environment: 'environment',
    accepted: 'accepted · Session created',
    rejected: (harness, protocols) => `rejected before the Session is created · ${harness} speaks only ${protocols}`,
    envs: {
      openai_hosted: { label: 'managed sandbox', note: 'Docker · microsandbox · E2B' },
      self_hosted: { label: 'your machine', note: 'Linux · macOS · Windows' },
    },
    input: 'Fix the failing test and explain the change.',
    footnote: 'The supported combinations are declared by each harness, not guessed.',
    footnoteLink: 'Harness capabilities',
  },
  pillars: [
    { tag: 'api', title: 'Same API as OpenAI', body: 'Point the official OpenAI SDK, or plain HTTP, at your installation. No new client to learn.' },
    { tag: 'agent', title: 'Your choice of agent', body: 'Each Session runs a native harness: Codex, Claude Code or MiniMax Code, with the model provider you configure.' },
    { tag: 'machine', title: 'Your choice of machine', body: 'A managed sandbox (Docker, microsandbox or E2B), or your own Linux, macOS or Windows machine.' },
    { tag: 'swap', title: 'Every part is replaceable', body: 'Sandboxes, harnesses and model providers plug in through defined protocols. Swap one without touching Core.' },
  ],
  architecture: {
    index: '03',
    kicker: 'protocols at every boundary',
    title: 'Every part plugs in through a protocol.',
    lede: 'Core keeps durable execution state and schedules work. The Runtime prepares the Environment and starts the native harness. The harness calls the model and tools, and reports back through the Runtime.',
    tabs: [
      { id: 'ecosystem', label: 'ecosystem', alt: 'Applications reach OpenAgentCore through the Agents API; harnesses, models and compute connect through their own boundaries.' },
      { id: 'protocols', label: 'components', alt: 'Agents API and Core API on top of Core; Sandbox Provider, Runtime, Harness and Model Provider connected by protocols.' },
      { id: 'surfaces', label: 'api namespaces', alt: 'Three namespaces with three credentials: Agents API for applications, Core API for operators, Machine API for nodes.' },
    ],
    boundariesTitle: 'one boundary · one protocol · one document',
    boundaries: [
      { from: 'Application', to: 'Core', doc: 'Agents API', link: '/docs/api/public-agent-api' },
      { from: 'Core', to: 'Sandbox Provider', doc: 'Sandbox Provider guide', link: '/docs/sandbox-provider' },
      { from: 'Provider', to: 'Runtime', doc: 'Runtime bootstrap', link: '/docs/runtime-bootstrap' },
      { from: 'Core', to: 'Runtime', doc: 'Core–Runtime protocol', link: '/docs/runtime-protocol' },
      { from: 'Runtime', to: 'Harness', doc: 'Harness onboarding', link: '/contracts/agents-api/harness-onboarding' },
      { from: 'Harness', to: 'Model', doc: 'Model execution', link: '/contracts/agents-api/model-execution' },
    ],
    more: 'Read the architecture',
  },
  session: {
    index: '04',
    kicker: 'execution you can manage',
    title: 'A run becomes a Session you can manage.',
    lede: 'A Session is a continuing piece of agent work; a Turn is one input executed inside it. Both have identities and durable state. Four rules decide what "running", "waiting", "cancelled" and "failed" mean.',
    rules: [
      { title: 'Connection ≠ work', body: 'Closing the stream only removes an observer. Submitted work stays with the execution system.', art: 'client ──────╳  disconnect\nturn   ━━━━━━━━━━━━━━▶ completed' },
      { title: 'Retries have identity', body: 'Supported submission paths carry a stable key, so a lost response never creates the work twice.', art: 'POST  Idempotency-Key: a1f ──▶ turn_01\nPOST  Idempotency-Key: a1f ──▶ turn_01' },
      { title: 'Cancel has a receipt', body: 'Closing output proves nothing. Core confirms the target Turn\'s execution and cleanup.', art: 'cancel ──▶ turn  in_progress\n       ──▶ turn  cancelled  ✓ confirmed' },
      { title: 'Execution ≠ machine', body: 'A Turn ending, the executor closing and the Environment being reclaimed are separate lifecycles.', art: 'turn        ━━━━┫\nexecutor    ━━━━━━━━┫\nenvironment ━━━━━━━━━━━━━┫' },
    ],
    honest: 'Recovery has limits, and they are explicit: an unknown outcome is never reported as success.',
  },
  observe: {
    index: '05',
    kicker: 'see it run',
    title: 'Operations you can actually see.',
    lede: 'Web is the operator console: Sessions, node capacity, work waiting for the caller, errors, latency, tokens and tool calls, per Project.',
    tabs: [
      { id: 'overview', label: 'overview', alt: 'Web console overview: service status, running Sessions, sandbox capacity, fleet and Projects.' },
      { id: 'metrics', label: 'agent metrics', alt: 'Web console agent metrics: requests, errors, latency, tokens and tool calls.' },
    ],
    caption: 'Screenshot values are illustrative. Usage visibility depends on what each native harness reports.',
  },
  compare: {
    index: '06',
    kicker: 'where it fits',
    title: 'What you still own, depending on how you build.',
    head: ['approach', 'fits when', 'you still own'],
    rows: [
      ['Call a model API', 'One-off inference, or you want full control of the agent loop', 'Context, tool execution, the loop and the whole task lifecycle'],
      ['Call a native CLI or SDK', 'Personal automation, or one integration around one harness', 'Session mapping, processes, resources and every engine\'s differences'],
      ['Use a sandbox service', 'You need an isolated computer', 'The agent engine, execution, interaction, records and the product API'],
      ['Use OpenAgentCore', 'Self-hosted, many native harnesses and environments behind one API', 'Business permissions, product experience, team orchestration and operating your installation'],
    ],
  },
  tradeoffs: {
    index: '07',
    kicker: 'trade-offs, stated',
    title: 'The choices behind the design.',
    motto: ['State belongs to the infrastructure.', 'Intelligence belongs to the harness.'],
    items: [
      { title: 'Native harnesses, with their constraints', body: 'The Runtime and the native harness run inside the Environment; Core holds the durable control plane. Native engines are reused as-is, so the Environment, native history and recovery still matter.' },
      { title: 'One protocol, real differences', body: 'A capability one harness supports is not silently granted to another. Core checks each combination before running, rejects what is unsupported and records what is unverified.' },
      { title: 'Simple for the caller', body: 'Callers think in tasks, input, state and results. Machine connections, native executors and cleanup belong behind clear component boundaries. The interface is the product.' },
    ],
  },
  start: {
    index: '08',
    kicker: 'get started',
    title: 'From install to first Session.',
    lede: 'On a Linux amd64 host with Docker and Python 3.9+:',
    steps: [
      { title: 'Sign in to Web', body: 'Use the Core key the installer created, then configure the domain and HTTPS.' },
      { title: 'Set a default model', body: 'Then issue a Project API key for your application.' },
      { title: 'Add execution capacity', body: 'A node, E2B, or your own machine.' },
      { title: 'Run your first Session', body: 'With the official OpenAI SDK, against your own Core.' },
    ],
    cta: 'Installation guide',
    trial: 'Quickstart',
  },
  roadmap: {
    index: '09',
    kicker: 'what comes next',
    title: 'Version 1 is the foundation.',
    lede: 'OpenAgentCore is pre-release; support is qualified per harness, environment and operation. The next layers we are working toward:',
    tracks: [
      { layer: 'harness', title: 'Agent loop, decoupled', body: 'Run the agent loop on the server and tool execution on a local machine or a cloud sandbox, with enterprise authorization at the tool layer.' },
      { layer: 'compute', title: 'Faster, denser compute', body: 'Separate compute from storage, restore state quickly and scale sandbox capacity on demand.' },
      { layer: 'storage', title: 'More storage backends', body: 'S3, OSS, shared file systems and agent-native file systems.' },
      { layer: 'apps', title: 'Applications on the Agents API', body: 'Keep building real products on the public API, the same way any application uses it.' },
    ],
  },
  docs: {
    index: '10',
    kicker: 'documentation',
    title: 'Everything is in the docs.',
    lede: (pages) => `${pages} pages, listed straight from the repository's docs.json. New guides appear here when they are added.`,
    open: 'open',
  },
  outro: { line: 'Products differ in interaction, model and engine. They can share one execution layer.', primary: 'Get started', secondary: 'Star on GitHub' },
}

const zh: LandingCopy = {
  hero: {
    eyebrow: '开源 · 可自部署 · OpenAI Agents API',
    title: ['One core.', 'Many agents.'],
    lede: 'OpenAI Agents API 的开源实现。用官方 OpenAI SDK，在你自己的模型和机器上运行 Codex、Claude Code 或 MiniMax Code。',
    primary: '运行第一个 Session',
    secondary: '阅读文档',
    installLabel: 'Linux amd64 · Docker · Python 3.9+',
    copied: '已复制',
    copy: '复制',
    hud: [
      ['api', '/v1 · OpenAI Agents API'],
      ['harness', 'codex | claude_sdk | mcode'],
      ['protocol', 'responses | anthropic | chat_completions'],
      ['sandbox', 'docker | microsandbox | e2b'],
      ['machine', 'linux | macos | windows'],
    ],
  },
  ticker: en.ticker,
  problem: {
    index: '01',
    kicker: '缺失的一层',
    title: '调用模型已经标准化，让 Agent 干活还没有。',
    lede: 'Agent 要读文件、执行命令、等待测试，中途还可能需要用户确认。模型的一次返回，只是其中一步。把它接进产品，又会冒出另一组问题。',
    questions: [
      { q: '用户关掉网页，已经提交的工作怎么办？', a: '连接与任务分开。连接断开只是观察者离开，Turn 继续执行，随时查询持久状态。' },
      { q: '请求超时了，应该再发一次，还是先查原来的任务？', a: '查询持久状态。已支持的提交路径带幂等标识，响应丢失也不会重复创建工作。' },
      { q: '点击“停止”，停的是输出，还是 Agent 和它启动的工作？', a: '取消要有回执：Core 确认目标 Turn 的执行和清理状态。' },
      { q: '这次用了哪个模型、哪台机器？输出文件和执行记录在哪？', a: 'Session、Turn、Items、产物和用量由 Core 保存，通过 API 查询。' },
      { q: '从 Codex 换到 Claude Code，从 Docker 换到 E2B，产品要改多少代码？', a: 'API 调用不变。Harness 是 Session 上的设置，配上相应协议的模型服务；沙箱由管理员选择，藏在 Sandbox Provider 后面。' },
    ],
    punch: '一个 Agent 在终端里好用，和它能被产品稳定调用，中间还有一整层工程。OpenAgentCore 就是这一层。',
  },
  compose: {
    index: '02',
    kicker: '自由组合',
    title: '选模型、选 Agent、选机器，是三个独立的决定。',
    lede: '每个 Session 指定自己的 Harness、模型服务和执行环境。Core 在执行前校验组合，不支持的直接拒绝。',
    harness: 'harness',
    protocol: '模型协议',
    environment: '执行环境',
    accepted: '通过校验 · Session 已创建',
    rejected: (harness, protocols) => `创建前即被拒绝 · ${harness} 只支持 ${protocols}`,
    envs: {
      openai_hosted: { label: '托管沙箱', note: 'Docker · microsandbox · E2B' },
      self_hosted: { label: '你自己的机器', note: 'Linux · macOS · Windows' },
    },
    input: '修复失败的测试，并说明改动。',
    footnote: '支持哪些组合，由每个 Harness 明确声明，而不是猜测。',
    footnoteLink: 'Harness 能力表',
  },
  pillars: [
    { tag: 'api', title: '与 OpenAI 相同的 API', body: '用官方 OpenAI SDK 或 HTTP，连接你自己的 Core 地址，不用学新客户端。' },
    { tag: 'agent', title: '自选 Agent', body: '每个 Session 运行一个原生 Harness：Codex、Claude Code 或 MiniMax Code，模型服务由你配置。' },
    { tag: 'machine', title: '自选机器', body: '托管沙箱（Docker、microsandbox、E2B），或你自己的 Linux、macOS、Windows 机器。' },
    { tag: 'swap', title: '部件可替换', body: '沙箱、Harness 和模型服务都通过明确的协议接入，替换任何一个都不动 Core。' },
  ],
  architecture: {
    index: '03',
    kicker: '每条边界都是协议',
    title: '每个部件，都通过协议接入。',
    lede: 'Core 保存持久执行状态并调度工作；Runtime 在 Environment 中准备文件和能力，启动原生 Harness；Harness 调用模型和工具，经 Runtime 把事件回传给 Core。',
    tabs: [
      { id: 'ecosystem', label: '生态', alt: '上层应用通过 Agents API 接入，Harness、模型服务和计算资源通过各自边界连接。' },
      { id: 'protocols', label: '组件', alt: 'Core 之上是 Agents API 与 Core API；Sandbox Provider、Runtime、Harness、Model Provider 通过协议连接。' },
      { id: 'surfaces', label: 'API 命名空间', alt: '三个命名空间、三种凭证：应用用 Agents API，管理员用 Core API，节点用 Machine API。' },
    ],
    boundariesTitle: '一条边界 · 一个协议 · 一份文档',
    boundaries: en.architecture.boundaries.map((b) => ({ ...b })),
    more: '阅读架构文档',
  },
  session: {
    index: '04',
    kicker: '可管理的执行',
    title: '把一次执行，变成可以管理的 Session。',
    lede: 'Session 表示一段持续的 Agent 工作，Turn 是其中一次输入的执行。它们都有标识和持久状态。四条约定，决定了“执行中”“等待处理”“已取消”“失败”分别意味着什么。',
    rules: [
      { title: '连接 ≠ 任务', body: '关闭输出流只是观察者离开，已提交的工作仍由执行系统管理。', art: en.session.rules[0].art },
      { title: '重试有身份', body: '已支持的提交路径带稳定标识，响应丢失也不会重复创建工作。', art: en.session.rules[1].art },
      { title: '取消要有回执', body: '关闭输出流证明不了什么，Core 确认目标 Turn 的执行和清理状态。', art: en.session.rules[2].art },
      { title: '执行 ≠ 机器', body: '一轮任务结束、Executor 关闭、Environment 回收，是三个不同的生命周期。', art: en.session.rules[3].art },
    ],
    honest: '故障恢复有边界，而且是明确的：未知结果永远不会被当成成功。',
  },
  observe: {
    index: '05',
    kicker: '看见运行',
    title: '运行情况，看得见。',
    lede: 'Web 是管理员控制台：按 Project 查看 Session、节点容量、等待调用方处理的工作，以及错误、耗时、Token 和工具调用。',
    tabs: [
      { id: 'overview', label: '部署概览', alt: 'Web 控制台概览：服务状态、运行中的 Session、沙箱容量、节点与 Project。' },
      { id: 'metrics', label: 'Agent 监控', alt: 'Web 控制台 Agent 监控：请求、错误、耗时、Token 与工具调用。' },
    ],
    caption: '截图数值仅用于展示界面。用量可见性取决于原生 Harness 提供的数据。',
  },
  compare: {
    index: '06',
    kicker: '适用场景',
    title: '不同接法，产品还要自己负责什么。',
    head: ['接法', '适合什么', '还要自己负责'],
    rows: [
      ['直接调用模型 API', '单次推理，或希望完全控制 Agent 循环', '上下文、工具执行、循环推进，以及完整的任务生命周期'],
      ['直接调用原生 CLI / SDK', '个人自动化，或围绕一种 Harness 构建集成', '会话映射、执行进程、资源管理和不同引擎的差异'],
      ['使用沙箱服务', '需要隔离的计算环境', '接入 Agent 引擎，管理执行、交互、记录和产品接口'],
      ['使用 OpenAgentCore', '自部署，通过统一 API 使用多个原生 Harness 和执行环境', '业务权限、产品体验、团队编排，以及自身部署的运维'],
    ],
  },
  tradeoffs: {
    index: '07',
    kicker: '关键取舍',
    title: '设计背后的选择。',
    motto: ['状态归基础设施，', '智能归 Harness。'],
    items: [
      { title: '保留原生 Harness，也接受它的约束', body: 'Runtime 和原生 Harness 运行在执行环境中，持久控制面放在 Core。原生引擎得以直接复用，运行环境、原生历史和恢复能力也因此仍然重要。' },
      { title: '共享协议，保留真实差异', body: '某个 Harness 支持的能力，不会被默认赋予其他 Harness。Core 在执行前检查组合，不支持的明确拒绝，待验证的如实记录。' },
      { title: '把简单留给调用者', body: '调用者只需要理解任务、输入、状态和结果。机器连接、原生执行器、资源清理，由清楚的组件边界承接。接口本身就是产品。' },
    ],
  },
  start: {
    index: '08',
    kicker: '开始使用',
    title: '从安装到第一个 Session。',
    lede: '在装有 Docker 和 Python 3.9+ 的 Linux amd64 主机上：',
    steps: [
      { title: '登录 Web', body: '用安装器生成的 Core key 登录，配置域名和 HTTPS。' },
      { title: '设置默认模型', body: '再为你的应用创建 Project API key。' },
      { title: '添加执行资源', body: '节点、E2B，或你自己的机器。' },
      { title: '运行第一个 Session', body: '用官方 OpenAI SDK，连接你自己的 Core。' },
    ],
    cta: '安装指南',
    trial: '快速开始',
  },
  roadmap: {
    index: '09',
    kicker: '下一步',
    title: '1.0 只是地基。',
    lede: 'OpenAgentCore 仍处于 pre-release 阶段，支持范围以具体的 Harness、环境和操作组合为单位验收。接下来的方向：',
    tracks: [
      { layer: 'harness', title: 'Agent loop 与工具解耦', body: 'Loop 运行在服务端，工具在用户本地或云端沙箱执行，工具层实现企业级鉴权。' },
      { layer: 'compute', title: '更快、更密的计算', body: '存算分离、快速恢复状态，按需扩容沙箱容量。' },
      { layer: 'storage', title: '更多存储介质', body: 'S3、OSS、共享文件系统，以及面向 Agent 的文件系统。' },
      { layer: 'apps', title: '基于 Agents API 的应用', body: '继续在公开 API 上打磨真实产品，和任何应用的接入方式完全一样。' },
    ],
  },
  docs: {
    index: '10',
    kicker: '文档',
    title: '一切细节，都在文档里。',
    lede: (pages) => `共 ${pages} 页，直接读取仓库的 docs.json 生成。新增文档会自动出现在这里。文档正文为英文。`,
    open: '打开',
  },
  outro: { line: '产品可以有不同的交互、模型和执行引擎，共用同一层执行基础设施。', primary: '开始使用', secondary: '在 GitHub 上 Star' },
}

export const copy: Record<Lang, LandingCopy> = { en, zh }

/** Native protocols per harness, default first (contracts/agents-api/model-execution.md). */
export const harnesses = [
  { id: 'codex', label: 'Codex', protocols: ['responses'] },
  { id: 'claude_sdk', label: 'Claude Code', protocols: ['anthropic'] },
  { id: 'mcode', label: 'MiniMax Code', protocols: ['anthropic', 'responses', 'chat_completions'] },
] as const

export const protocols = ['responses', 'anthropic', 'chat_completions'] as const

export const installCommand = 'curl -fsSL https://github.com/MiniMax-AI/OpenAgentCore/releases/latest/download/install.sh | bash'

export const repoUrl = 'https://github.com/MiniMax-AI/OpenAgentCore'
