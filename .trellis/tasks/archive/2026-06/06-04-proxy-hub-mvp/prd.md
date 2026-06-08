# proxy-hub MVP — 产品需求（PRD）

> 父任务。仅包含需求、约束与验收标准。
> 技术设计见 `design.md`；里程碑执行计划见 `implement.md`。
>
> 语言约定：本项目所有规划文档与代码注释一律使用中文（见 `AGENTS.md` 的"项目约定 / 语言规范"）。

## 目标

构建 **proxy-hub**：一个单二进制、可自托管的网关，把 OpenAI/Claude 的 **API-Key**
以及第三方 **OpenAI/Claude 兼容 upstream** 统一到 OpenAI- 与 Claude-兼容端点之后，提供
模型映射、完善的用量统计与监控、轻量化部署，并额外提供 Codex/Claude 的 MCP 服务器共享管理。

当前仓库仅有 Trellis 脚手架（`.trellis/`、`AGENTS.md`）与 `references/` 下被 git 忽略的参考
代码检出。proxy-hub 是全新（greenfield）构建。

## 背景与参考

深入分析了四个参考项目（详见父任务 `design.md` 的"参考项目要点"）。简述如下：

- **CLIProxyAPI**（Go，单二进制）：渠道路由、模型映射、多凭证负载均衡、按 HTTP 状态码的
  冷却、轻量的文件/yaml 部署。（其 OAuth 订阅 + utls + 翻译矩阵机制**不采用**——见非目标。）
- **new-api**（Go + React）：渠道→ability 路由索引、按 provider 的 adaptor 接口、链式模型映射、
  双存储统计（明细日志 + 小时滚动汇总）、单二进制内嵌 SQLite + 内存缓存（免 Redis）。**主要
  架构来源**。
- **sub2api**（Go + Vue）：统一的账号/渠道 schema（单表 + JSON 凭证）、通配符模型映射、
  append-only `usage_logs`、有界 worker pool 异步落库。其**硬依赖 PostgreSQL + Redis 被拒绝**。
- **cc-switch**（Tauri/Rust）：唯一实现了 MCP 共享的参考——把统一注册表投影进
  `~/.codex/config.toml` 与 `~/.claude.json`。**MCP 设计来源**。

## 范围内 —— 四大能力

### C1 —— 上游渠道管理 + 模型映射 + 统一端点
- 管理两类上游**渠道**：`api_key`（官方端点 + key）与 `upstream`（自定义 `base_url` + key，
  用于第三方 OpenAI/Claude 兼容中转）。
- 两个上游平台/方言：**OpenAI** 与 **Anthropic（Claude）**。
- 统一入站端点：`/v1/chat/completions`（OpenAI）、`/v1/messages`（Claude）、
  `/v1/responses`（OpenAI Responses）、`/v1/models`。
- **模型映射**：每渠道的重命名/重映射（精确 + 尾部 `*` 最长匹配通配；为空 = 透传），外加可选
  的 `prefix` 命名空间。
- **负载均衡与失败转移**（同一模型由多个渠道服务时）：轮询 + 加权优先级、会话亲和、
  跨渠道重试、按 (渠道, 模型) 的 HTTP 状态感知冷却。
- **同方言路由**是核心路径（OpenAI 客户端 → OpenAI/兼容 upstream；Claude 客户端 →
  Claude/兼容 upstream）。**跨方言翻译**仅限单一类型化转换对——OpenAI-chat ⇄ Claude-messages，
  由一致性测试套件守门。

### C2 —— 统计与监控
- 按 key、按渠道、按模型 的 token / 成本 / 延迟（含 TTFT）/ 错误 追踪。
- append-only 请求事实表 + 预聚合的小时/日滚动汇总，支撑快速仪表盘。
- 配置驱动的模型定价；从 token 数计算成本（因为是 API-Key 渠道，按 token 真实计费）。
- 被动渠道健康（来自实时流量结果）；`/healthz`；可选主动探测。
- 仪表盘读取 API + React 管理后台。

### C3 —— 轻量化部署
- 一个静态 Go 二进制（`CGO_ENABLED=0`），管理后台 SPA 通过 `go:embed` 内嵌。
- 一个内嵌 SQLite 文件（WAL）、一个 `config.yaml`（可被环境变量覆盖）、密钥按文件存于
  `data/auths/`。
- 一个容器、一个 `/data` 卷、一个对外端口：
  `docker run -v ./data:/data -p 7777:7777 proxy-hub`。镜像目标 ~20 MB。
- 无任何必需的外部服务（无 MySQL/Postgres、无 Redis、无消息队列）。

### C4 —— Codex/Claude MCP 共享管理
- SQLite 中的统一 MCP 服务器注册表（SSOT），带 per-client 启用位图。
- 把定义投影进 **Codex**（`~/.codex/config.toml`，对 `[mcp_servers.<id>]` 做外科手术式编辑）
  与 **Claude Code**（`~/.claude.json`，只替换 `mcpServers` 键，其余字节级保留，含 `projects`
  会话历史）。
- 写入为**显式/可选的同步目标**（绝不隐式改写 `$HOME`），原子写、首次写前备份、并发写安全。
- 单向同步（注册表 → 客户端）+ 只读导入（冲突 ⇒ 翻开关位，绝不覆盖 spec）。

## 范围外 —— 非目标（明确）

| 非目标 | 原因 |
|---|---|
| **OpenAI/Claude OAuth *订阅* 渠道**（Claude Code / ChatGPT Codex 登录） | 2026-06-04 决策。对抗式审查确认：Codex Responses 走 WebSocket 传输且需 reasoning 加密内容签名；Anthropic 订阅出口需 utls 绕 Cloudflare（CI 无法测、猫鼠游戏）；per-request token/成本无法从代理流可靠获得（cc-switch 改读 CLI session 日志）；已注册的 redirect_uri 与单端口/无头部署冲突；且代理消费级订阅很可能违反厂商 ToS。从 proxy-hub 排除。 |
| 完整 NxN 格式翻译矩阵（CLIProxyAPI 的 ~60 个包） | 上游限制修改、体量过大。只支持 OpenAI-chat ⇄ Claude-messages 一对。 |
| Gemini / Vertex / Bedrock / xAI / Kimi 上游 | 两个平台（OpenAI + Anthropic）已覆盖需求。adaptor 接口留口子，但不再额外发布方言。 |
| 多节点集群、分布式会话粘性、Redis 并发槽、外部 Postgres/MySQL | 目标是单节点轻量。v1 不留多方言数据库抽象层（被判定为维护陷阱）。 |
| SaaS 计费面（支付、兑换/订阅码、分销、充值、2FA/passkey、多 OAuth 管理端登录） | new-api/sub2api 的臃肿，与个人/小团队 Hub 无关。 |
| 双向/持续 MCP 自动对账、监听客户端配置文件变更 | MVP 同步为单向 + 显式导入。自动监听有反馈环风险。 |
| Codex + Claude Code 以外的 MCP 客户端 | 能力定义明确只含两个客户端。 |
| 在中转里转发 per-request 内联 `mcp_servers` | 这是独立的中转透传问题，不是配置共享。 |

## 功能需求

**渠道（C1）**
- FR-C1-1：渠道（`api_key` / `upstream`）的 CRUD，含 platform、base_url、models、
  model_mapping、prefix、group、priority、weight、enabled、可选出口代理。
- FR-C1-2："测试渠道"动作：探测 upstream 并记录响应时间。
- FR-C1-3：入站 API-Key 鉴权（OpenAI `Authorization: Bearer` 与 Claude `x-api-key`），
  通过内存缓存对存储的 key 哈希校验。
- FR-C1-4：统一端点中转到所选渠道；同方言 = 透传；跨方言 = 单一类型化 OpenAI⇄Claude 转换器。
- FR-C1-5：模型映射（精确、尾部 `*` 最长匹配、透传；prefix 命名空间），统计时保留客户端面的
  模型名、同时改写上游名。
- FR-C1-6：选择 = 最高优先级档 → 加权随机，带会话亲和；可重试上游失败时换一个渠道重试。
- FR-C1-7：按 (渠道, 模型) 的冷却，由上游 HTTP 状态驱动（429 → 指数退避；401/403 → 较长；
  404/不支持 → 很长；5xx → 较短）。

**统计（C2）**
- FR-C2-1：每次完成的请求写一条 append-only 事实行（token 拆分为
  input/output/reasoning/cache-read/cache-creation，延迟、TTFT、状态、错误类别、
  客户端面 vs 上游模型、session id）。不存请求/响应体。
- FR-C2-2：小时 + 日滚动汇总，由内存缓冲定期 flush 维护；汇总存 token/延迟聚合（不预存成本）。
- FR-C2-3：成本由配置驱动的定价表在读取/聚合时计算，定价更正可对历史重算；未知模型 ⇒ 打标，
  token 追踪继续。
- FR-C2-4：仪表盘 API：概览、时序、分组（按模型/渠道/key/错误）、分页请求日志钻取（按
  request_id 查找）、渠道健康。
- FR-C2-5：用量采集绝不阻塞中转热路径，且绝不**静默**丢弃计费数据：溢出要计数并暴露，
  并提供同步回退选项。
- FR-C2-6：`/healthz`（进程 + DB ping）；被动 per-(渠道, 模型) 健康；可选主动心跳探测（默认关）。

**部署（C3）**
- FR-C3-1：单静态二进制；内嵌 SPA；内嵌 SQLite（WAL、busy_timeout、读连接池 + 单写协程）；
  一个 `config.yaml`（可被环境变量覆盖）热重载。
- FR-C3-2：版本化迁移，含预迁移 DB 备份与表重建模式（正视 SQLite ALTER 的局限，而非假装没有）。
- FR-C3-3：单服务容器、单卷、单端口；容器 HEALTHCHECK；~20 MB 镜像。
- FR-C3-4：密钥（`data/auths/*.json`、key 哈希、admin key）`0600`，绝不记录到日志。

**MCP（C4）**
- FR-C4-1：注册表 CRUD + per-client 启用开关；规范的宽松 JSON spec（stdio/http/sse），
  往返保留未知字段；宽松的 per-server 校验。
- FR-C4-2：显式同步目标（运维登记的绝对路径；单用户 HOME 自动探测为可选）。同步绝不创建
  多余目录（父目录缺失则跳过）。
- FR-C4-3：Codex 写入器 = 外科手术式 `[mcp_servers.<id>]` TOML 编辑 + 清理遗留
  `[mcp.servers]`；Claude 写入器 = 整文件往返、只替换 `mcpServers`（保留 `projects` 与其它键），
  Windows 下 `cmd /c` 包装并带 WSL 检测。
- FR-C4-4：原子 temp+rename 写、首次写前 `.bak` 备份、并发写安全（写前立即重读；按目标加锁）。
- FR-C4-5：CRUD/toggle 时单向同步 + 显式全量对账端点 + CLI `proxy-hub mcp sync`；导入为只读
  （冲突 ⇒ 翻开关位，绝不覆盖 spec），并给出"spec differs"警告。

## 非功能需求

- NFR-1（轻量）：默认运行零外部服务；冷启动 < 2s；镜像 ~20 MB；空闲内存适度。
- NFR-2（热路径）：中转开销极小；日志/健康写入异步且不阻塞；路由按请求读内存而非 DB。
- NFR-3（持久化/正确性）：金额用 decimal（以字符串存），绝不用 float；用量数据绝不静默丢弃；
  密钥绝不落到统计 DB 或日志。
- NFR-4（可移植）：`CGO_ENABLED=0` 静态二进制，支持 linux/macos/windows（amd64+arm64）；
  纯 Go SQLite 驱动。工具链 **Go 1.25+**（本机 go1.25.5）。
- NFR-5（客户端配置写入安全）：proxy-hub 绝不破坏用户的 `~/.claude.json`（尤其 `projects`
  历史）或 `~/.codex/config.toml`（无关表）。

## 验收标准（父级、跨子）

- [ ] `docker run -v ./data:/data -p 7777:7777 proxy-hub` 在无其它服务下启动；`/healthz`
      返回 200；首次运行创建 DB + 迁移。
- [ ] 可通过管理 API/UI 创建一个 OpenAI API-Key 渠道与一个 Claude API-Key 渠道，测试并经统一
      端点服务真实流量。
- [ ] 模型映射（重命名 + 通配 + prefix）路由正确；可重试上游错误时发生渠道间失败转移；多轮会话
      粘在同一渠道。
- [ ] OpenAI 格式请求可经单一跨方言转换器由 Claude upstream 服务（反之亦然），通过一致性套件
      （流式 + 工具）。
- [ ] 仪表盘展示按 key/渠道/模型 的 token、成本、延迟、TTFT、错误率，含时序趋势与按 request_id
      钻取；定价变更可对历史重算。
- [ ] 用量采集绝不阻塞中转；人为制造的溢出被计数并暴露（而非静默丢弃）。
- [ ] 定义一个 MCP 服务器、启用 Codex + Claude、执行"同步"：它正确出现在 `~/.codex/config.toml`
      与 `~/.claude.json` 中，且所有无关内容（含 Claude `projects`）字节级保留；再次导入能读回而
      不覆盖 spec。
- [ ] 跨平台发布二进制（linux/macos/windows）可由单一配置文件运行。
- [ ] lint、类型检查、一致性/集成测试通过；`.trellis/spec` 已更新。

## 已决决策（2026-06-04 确定）

1. **范围排序**：去风险化分期——先做 API-Key 渠道 + 统计 + MCP + 部署；不做订阅阶段。
2. **订阅/OAuth 渠道**：**不构建**（见非目标）。proxy-hub 不内置逆向出的厂商 ClientID；
   不代理消费级订阅。
3. **Trellis**：本任务树（父 + 5 个里程碑子任务）即为既定计划。

## 开放问题（规划期/子任务启动前确认）

- OQ-1 多租户深度：`user_id` 是否一等维度（多终端用户、各自 key/配额），还是单运维
  （user_id = 0、列预留）？*默认：单运维。*
- OQ-2 DB 访问：`sqlc`（类型化代码生成）vs 手写 `database/sql`。*默认：sqlc。*
- OQ-3 跨方言转换器：放 M2，还是拆为后续？*默认：放 M2，作为最后一项。*
- OQ-4 入站按 key 限速/配额：上线时需要，还是 MVP 后？*默认：MVP 后。*
- OQ-5 单用户主机的 MCP HOME 自动探测：可选默认关，还是始终显式？*默认：可选、默认关。*
- OQ-6 共享 origin 上的 管理端/SPA 鉴权模型（bearer + origin 校验 vs cookie/CSRF）。
  *默认：SPA 用 bearer token + 对改文件端点做 origin 校验。*

## 任务图

| 子任务 | 交付物 |
|---|---|
| `06-04-m1-foundations` | Store（SQLite WAL + 写协程）、config + 热重载、Gin 骨架 + `/healthz`、打包、内嵌 SPA 壳 |
| `06-04-m2-channel-relay` | 渠道 + 路由索引 + API-Key 中转 + adaptor + 选择器 + 冷却 + 模型映射 + 跨方言转换器 |
| `06-04-m3-stats-monitoring` | 事实表 + 滚动汇总 + 定价 + 异步采集器 + 仪表盘 API + React 仪表盘 |
| `06-04-m4-mcp-sharing` | MCP 注册表 + 同步目标 + Claude/Codex 写入器 + fileio 安全 + API/CLI + UI |
| `06-04-m5-release-hardening` | 主动健康、FillFirst、定价 UI、管理端鉴权/CSRF、goreleaser、文档、e2e |

依赖关系记录在各子任务的 `prd.md` / `implement.md` 中，而非由树位置隐含。M1 阻塞全部；M2 依赖
M1；M3 与 M4 依赖 M1（并依赖 M2 提供实时数据 / fileio 助手）且可并行；M5 为最终打磨。
