# 管理员指标：后端需求

状态核对于 2026-09-28：P0 Agent/工具聚合（BE-8）仍是未实现提案。Core 进程指标与节点主机历史已经通过下文链接的独立契约落地。其余 P1/P2 是待讨论设想，不代表已接受的开发清单，也不意味着现有观测能力全部缺失。[English](admin-metrics-backend-requirements.md)

控制台是管理工具，以监控为先：概览、Agent 监控、沙箱监控、Session 日志。它只读取 Web API（`/core/v1/**`，包括 `/core/v1/sandbox/**` 下的沙箱管理路由），从不调用 `/v1`。部分数字来自 Web API 的汇总，其余仍由浏览器对各项目做有上限的读取后自行汇总。本文说明这样做的代价、哪里不完整，以及哪些 Web API 接口可以替代浏览器端的计算。

## Core 的两类接口

1. **面向用户的 Agents API**（`/v1/**`）：只提供锁定版本的 OpenAI Agents API 官方路由；Core 的补充只以 `x_agents_core` 字段出现。指标相关工作不得在这里新增字段、路由或行为。
2. **Web API**（`/core/v1/**`，包括沙箱管理用的 `/core/v1/sandbox/**`）：由控制台服务端和运维脚本用 Core Key 调用。下文提议的所有接口都属于这一类；`services/core-console` 按前缀转发 `/core/v1/*`，新增接口无需改动代理。

## 控制台目前如何计算

| 页面 | 数据来源 | 限制 |
| --- | --- | --- |
| 概览 | `GET /summary`（按项目：资产数量、各状态 Session 数、用量、覆盖率、最近活跃）；`/core/v1/sandbox` 的部署与节点；读取各项目的 Session 列表，用于 24 小时活动图和需要处理的 Session | Session 列表读过 24 小时窗口、并找到汇总所计的全部需要处理的 Session 后即停止，每个项目最多 1,000 个；窗口开始前就不再活跃的项目不读取 |
| Agent 监控 | 用 `GET /summary` 跳过不活跃的项目；读取各项目的 Session 列表，再经项目作用域读取最近活跃 Session 的 Turn 与 Item；按 API 密钥的用量来自 `GET /summary?group_by=key` | 每个项目最多列出 2,000 个 Session；每次最多读取 200 个 Session，每个 Session 最多 10 页 Turn、5 页 Item，每个 Session 限时 15 秒、每次加载 45 秒 |
| 沙箱监控 | `/core/v1/sandbox` 的节点与分配；`GET /core/v1/sandbox/runtime-observations`（全部项目）；经所属项目按 ID 读取每个托管 Session；按托管 Session 读取运行时历史 | 每次刷新最多读取 100 个托管 Session；历史最多覆盖 24 个托管 Session；节点主机观测与历史通过下文的节点详情契约提供 |

控制台在问号提示和告警中说明的后果：

- “请求”指 Agent Turn。HTTP 层面的请求数、状态码和 API 延迟目前都拿不到。
- 模型归属来自 Session 的 Agent 快照，而不是实际生效的执行配置。
- 繁忙项目会超过 Session 读取上限，长时间范围的数据和较早的需要处理的 Session 可能不完整；页面会说明是哪些项目。
- 按 API 密钥的用量，统计的是所选时间范围内新建的 Session，按创建它的 key 归属（#87 写入溯源）；没有记录的 Session 显示为“未知”。

其他限制：浏览器与 Core 的时钟可能不一致（控制台默默容忍 15 分钟偏差，不在页面上提示）；不统计子 Agent 的 Turn 和已删除的 Session。

## 建议新增的接口

所有接口只读、部署级，可选 `project_id` 过滤；像运行时历史一样有边界（`start`、`end`、`step`、最大时间范围与点数）；不可用的值返回 `null`，不返回 0。这些是运维证据，不是计费依据。

### P0：Agent 运行聚合

`GET /core/v1/metrics/agent-runs?start=&end=&step=&group_by=model|agent|harness|project|key&project_id=`

每个时间桶（设置 `group_by` 时再按分组）返回：

- 创建、完成、失败、取消、仍在运行的 Turn 数。
- 从 `started_at` 到 `completed_at` 的耗时：平均、p50、p95。
- 从 `created_at` 到 `started_at` 的排队时间：平均、p95。
- Token：输入、输出、缓存输入、推理。
- 来自执行配置的实际模型与执行引擎。
- 前 N 个分组加一个 `other` 桶，以及分组总数。

它可以替代 Agent 监控页逐 Session 读取 Turn 的做法，并让长时间范围的数据完整。

### P0：工具调用聚合

`GET /core/v1/metrics/tool-calls?start=&end=&step=&group_by=tool|kind|agent&project_id=`

按时间桶和工具（`function` 名称、MCP 的 `server_label` 加名称、Shell 命令、网页搜索、子 Agent）返回调用数、失败数，以及 Item 上报时的耗时。它可以替代逐 Session 读取 Item。

### 已实现：Core 进程指标

`GET /core/v1/metrics?range=1h|6h|24h|7d` 提供 Core 进程 CPU/RSS 与限额、执行队列与槽位、PostgreSQL 和后台任务观测。单位、空值和保留范围以 [Core 指标契约](../../contracts/agents-api/core-metrics.md) 为准。原提议的 `/core/v1/core-status` 路由未采用。整机 CPU 不等于 Core 进程指标；其他主机字段是独立设想，不属于 BE-8 的缺失项。

### P1：Session 活动与需要处理的 Session

`GET /summary` 已经按项目、Agent 和 key 给出各状态的 Session 数。概览仍有两部分需要读取 Session 列表：

- 每个时间桶新建和失败的 Session：`GET /core/v1/metrics/sessions?start=&end=&step=&project_id=`。
- 跨项目需要处理的 Session：`GET /core/v1/sessions?status=failed,requires_action&order=last_active_desc&limit=`（每条标明所属项目），或者在各项目的 Session 列表上支持 `status` 过滤。

### P1：Agents API 请求指标

`GET /core/v1/metrics/api-requests?start=&end=&step=&group_by=route|status_class|project|key`

由 API 路由中间件记录：按路由族（Session、事件流、Turn、Item、文件等）统计请求数、4xx/5xx 数和延迟分位数。这就是托管平台控制台里的“请求数”和“错误率”。建议参照运行时历史，把汇总写入现有 PostgreSQL 并设保留期，可选 OTLP 导出作为第二去向。

### 已实现：节点主机观测与历史

`GET /core/v1/sandbox/nodes/{node_id}?range=1h|6h|24h` 提供节点主机历史。字段、空值、新鲜度和聚合规则以 [节点主机历史契约](../../contracts/agents-api/node-host-history.md) 为准。原提议的 `/nodes/{id}/history` 路由未采用。进一步扩展列表字段或分配数量的时间序列，需要单独确定范围，不是 BE-8 的前置条件。

### P2：托管运行时连同 Session 信息

`GET /core/v1/sandbox/runtime-observations` 为每条观测标明所属项目，但不含 Session 的标题、Agent、状态和用量，所以沙箱监控要按 ID 逐个读取托管 Session（每次刷新最多 100 个）。增加 `expand=session` 选项返回这些字段，就可以省掉这些读取。

### P2：密钥

- API 密钥的 `last_used_at` 和按密钥统计的请求数（随上面的 API 请求指标一起提供）。

## 待讨论问题

1. 聚合是读取时从现有表计算（简单，但大部署会慢），还是周期汇总（参照运行时历史）？
2. 每类指标的保留期和最大时间范围分别是多少？控制台用 7 天是否足够？
3. 各类指标统一使用哪种分位数算法和时间桶对齐方式？
4. API 请求指标是否要排除控制台自身的轮询流量？
5. Core 自身状态是否要包括执行器网关和模型端点的可达性，还是只限于进程和主机？
