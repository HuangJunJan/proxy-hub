# proxy-hub MVP — 技术设计（Design）

> 所有子任务继承的共享技术设计：边界、契约、数据流、取舍。需求见 `prd.md`；里程碑计划见
> `implement.md`。
>
> 语言约定：本项目所有规划文档与代码注释一律使用中文（见 `AGENTS.md`）。代码标识符、表名/列名、
> 文件路径、API 路径、技术名（Go、Gin、SQLite 等）保留原文。

## 1. 技术栈（已定）

| 层 | 选择 | 理由 |
|---|---|---|
| 语言/运行时 | **Go 1.25+**，Gin | 静态 `CGO_ENABLED=0` 交叉编译；~20 MB 镜像；复用 new-api 的路由/adaptor 模式。（本机 go1.25.5；不移植 CLIProxyAPI 代码，故无需 1.26。） |
| 持久化 | **内嵌 SQLite，纯 Go `modernc.org/sqlite`**，WAL + `busy_timeout=30000` + `synchronous=NORMAL` + `foreign_keys=ON` | 唯一能做真实 SQL 聚合统计的内嵌引擎；纯 Go 保持 CGO 关闭。**读连接池 + 单写协程**（修掉 cc-switch 的单 `Mutex<Connection>` 争用）。 |
| DB 访问 | **`sqlc` 生成类型化查询**（默认；OQ-2） | schema 小而显式；避免 new-api 的跨库 AutoMigrate 之痛。不用 GORM。 |
| 金额 | **`shopspring/decimal`**，以 TEXT 存储 | 无浮点漂移；跨库安全。 |
| JSON | 跨方言转换器用类型化 struct；**`gjson`/`sjson`** 仅用于透传/未知字段 | 承重转换要类型安全；string surgery 只留给透传。 |
| TOML（MCP/Codex） | **`pelletier/go-toml/v2`**，限定子树编辑 | Go 最接近 Rust `toml_edit` 的方案；被重写的 `[mcp_servers]` 表内注释保留为尽力而为，靠首次写前 `.bak` 兜底。 |
| 文件监听 | `fsnotify` | 热重载 `config.yaml` + `auths/`。 |
| 日志 | `log/slog`（stdlib，结构化） | 零额外依赖。 |
| 前端 | **React + TS + Vite + Tailwind + shadcn/ui + TanStack Query**，构建到 `web/dist` 并 `go:embed` | 与仓库既有 shadcn 提交及 Trellis `frontend/` 规范一致；单产物。 |
| 打包 | 多阶段 Dockerfile → `alpine:3.x`（+ca-certificates、tzdata）；`goreleaser` | ~20 MB 镜像；跨平台二进制。 |
| 配置 | 一个 `config.yaml`，每个键可被环境变量覆盖，热重载 | 干净运维 + 12-factor。 |

**不采用（相对参考项目）**：OAuth/PKCE 流程、`utls`、WebSocket executor、token 刷新循环
（无过期凭证）、NxN 翻译矩阵、Postgres/MySQL/Redis、SaaS/支付面、Tauri 桌面外壳。这些都属于
被砍掉的能力。

## 2. 架构

### 2.1 分层

```
AI 客户端（OpenAI/Claude SDK、指向 proxy-hub 的 Claude Code/Codex）──HTTP(单端口)──┐
                                                                                  │
 internal/api (Gin)：relay 路由 · admin 路由 · /v0/mcp/* · 中间件(auth/reqid/recover)   │
   │            │                 │                    │
 relay (C1)   stats (C2)        channel              mcp (C4，独立)
 select→map→  collector/        DAO + RouteIndex     store/service/clients
 adapt→call→  pricing/rollups   (内存)               (claude,codex)
 retry            │                 │                    │
   └──────── store (SQLite：读连接池 + 单写协程) ───────────────────────────┘
 adaptor (openai/claude/responses + convert)   selector (RR/加权/亲和 + per-model 冷却)
 credstore (data/auths/*.json —— API key)      fileio (原子写 + .bak + 锁)
```

**贯穿原则**
- **SSOT 分裂**：密钥在 `data/auths/*.json`；可查询状态在 SQLite；MCP 定义在 SQLite；客户端
  配置文件（`config.toml`、`.claude.json`）是*下游投影*。
- **热路径每请求不碰 DB**：路由读内存 `RouteIndex`（从 `abilities` 镜像）；用量写入经有界
  channel → 单写协程。
- **健康由 relay 路径拥有**（`MarkResult`）；统计*只读*健康。
- **MCP 是完全独立模块**：只共享 DB 与 `fileio` 助手。

### 2.2 目录布局（提案；目前均不存在）

```
cmd/proxy-hub/main.go            # 瘦入口：载配置 → 开 DB → 初始化管理器 → 起 Gin + 后台循环 + CLI 子命令
internal/
  api/{server,middleware/{auth,requestid,recover,bodylimit},relay_handlers,admin_handlers,stats_handlers,mcp_handlers}
  relay/{relay.go, markresult.go}
  adaptor/{adaptor.go, openai/, claude/, convert/}      # convert/ = 单一 OpenAI<->Claude 类型化对
  channel/{model,dao,runtime,routeindex}
  selector/{selector,roundrobin,weighted,affinity}
  credstore/store.go                                    # data/auths/*.json（API key），fsnotify 重载
  stats/{event,collector,pricing,dao}
  health/monitor.go
  mcp/{store/dao, service/service, clients/{client,claude,codex}, validation}
  store/{db, migrate}
  fileio/atomic.go                                      # 原子写 + .bak + 拷权限 + 跳过缺失 + 锁
  config/{config, watcher}
  version/version.go
web/                              # React SPA，dist/ 被 go:embed
migrations/*.sql                  # go:embed
pricing/seed.json                 # go:embed（LiteLLM 格式）
Dockerfile · docker-compose.yml（单服务/单卷/单端口）· .goreleaser.yaml · config.example.yaml
```

## 3. 数据模型（SQLite；JSON 入 TEXT；金额用 decimal TEXT）

> 审查驱动的修正标 **[FIX]**。凭证存于 `data/auths/<id>.json`，**不入** DB。

### channels —— 上游凭证元数据
`id PK, name, enabled, platform(openai|anthropic), type(api_key|upstream), base_url
(为空⇒provider 默认), group, priority(默认 50), weight, models(csv/json),
model_mapping(json), prefix, proxy_url, status(active|error|disabled), error_message,
created_at, updated_at`。
凭证 blob 存于 `data/auths/<id>.json`：`api_key` ⇒ `{api_key}`；`upstream` ⇒
`{base_url, api_key}`。

### abilities —— 派生路由索引（new-api 模式）
PK `(group, alias_model, channel_id)`；列 `upstream_model, priority, weight, enabled`。
**[FIX 命名空间碰撞]** `alias_model` 严格为**客户端面**键；解析出的 `upstream_model` 一并存储。
请求时固定解析顺序：**精确别名 → 最长 `*` 通配别名 → 原样上游名**。渠道变更时**增量**重建
（upsert/delete，绝不 TRUNCATE）；镜像进内存 `RouteIndex`。映射冲突在渠道保存时校验。

### request_logs —— append-only 事实表（钻取/追踪）
`id PK, request_id(idx), created_at(idx), api_key_id(idx), channel_id(idx), user_id(idx),
group, requested_model, upstream_model, model_mapping_chain, endpoint_format
(openai|claude|responses), is_stream, input_tokens, output_tokens, reasoning_tokens,
cache_read_tokens, cache_creation_tokens, total_tokens, latency_ms, first_token_ms(NULL),
status_code, is_error, error_type, error_message(截断), ip(NULL,可选), user_agent(NULL,可选),
session_id(idx), usage_source(stream|usage_block|estimated|missing)`。
索引：`(created_at)`、`(api_key_id,created_at)`、`(channel_id,created_at)`、
`(requested_model,created_at)`、`(status_code)`、`(session_id)`、`(request_id)`。
**[FIX 重算]** 这里无成本列——只存 token。**[FIX 隐私]** 绝不存请求/响应体。

### usage_hourly_rollups / usage_daily_rollups —— 预聚合时序
小时 PK `(bucket_hour, channel_id, api_key_id, requested_model)`；日 PK
`(date, channel_id, requested_model)`。列：`request_count, success_count, error_count,
input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, reasoning_tokens,
sum_latency_ms, sum_first_token_ms, count_first_token`。**[FIX]** 只存 token/延迟聚合——
**不预存成本**；成本读取时由 `model_pricing` 计算。内存缓冲每 60s 经 UPSERT flush
（`... ON CONFLICT DO UPDATE SET x=x+excluded.x`）+ 关机 flush；事实表是 SOT，汇总可重建。

### model_pricing —— 配置驱动
`model_id PK, input_per_million, output_per_million, cache_read_per_million,
cache_creation_per_million`（均 TEXT）。启动从内置 `pricing/seed.json`（LiteLLM 格式）载入，
管理端可覆盖。未知模型 ⇒ `pricing_missing` 标记；token 追踪仍工作。成本 = Σ(分量 token ×
单价/1e6)；遵守 Claude（input 为纯新）vs OpenAI（input 含缓存 → 需减去）的缓存 token 语义。

### channel_model_health —— 被动健康/冷却
**[FIX per-model 粒度]** PK `(channel_id, model)`（**不是**每渠道 1:1）。列 `is_healthy,
consecutive_failures, last_success_at, last_failure_at, last_error, cooldown_until,
updated_at`。由 `MarkResult` 写入；selector + 仪表盘读取。

### health_check_logs —— 主动探测（M5，可选）
`id, channel_id, model, success, http_status, response_time_ms, message, checked_at`。

### api_keys —— 入站平台发放的 key
`id PK, hash(sha256, idx), name, group, enabled, created_at, last_used_at`。（配额/RPM 字段
后置——OQ-4。）

### mcp_servers —— MCP SSOT（C4）
`id(=配置键) PK, name, spec_json(规范的宽松 spec，原样存), description, homepage, docs,
tags_json, enabled_codex, enabled_claude, created_at, updated_at`。spec：stdio
`{type,command,args,env,cwd}` / http|sse `{type,url,headers}`；`type` 省略 ⇒ stdio；往返保留
未知字段。

### mcp_sync_targets —— 显式可写客户端配置
`id PK, client(codex|claude), config_path(绝对，运维登记), label, enabled, last_synced_at,
last_sync_status`。单用户 HOME 自动探测为可选（OQ-5）。

### meta —— `key PK, value`（存 `schema_version`）。

## 4. 各能力设计

### C1 —— 渠道、映射、中转
- **统一端点**：`/v1/chat/completions`、`/v1/messages`、`/v1/responses`、`/v1/models`
  （列表形状按 UA 在 OpenAI vs Claude 间切换）。
- **请求流**：鉴权（内存 key 缓存）→ 提取模型（gjson）→ 剥离 `prefix` 与任何 `[1M]` 后缀 →
  `RouteIndex.candidates(group, alias_model)` → `selector.Pick`（亲和 → 加权优先级；经
  `isBlockedForModel` 跳过冷却中的）→ 解析 `upstream_model` → adaptor（同方言透传，否则
  OpenAI⇄Claude 转换器）→ 调用 upstream（用存储的 key + base_url，可选出口代理）→ 流式/非流式
  回传 → 发出 `UsageEvent` → `MarkResult(success)`。可重试失败（429/5xx/连接）⇒
  `MarkResult(cooldown)` + 换渠道，至多 `max_retries`。不可重试（4xx）⇒ 翻译错误并停止。
- **Adaptor 接口**：`ConvertOpenAIRequest / ConvertClaudeRequest / DoRequest / DoResponse`。
  实现：`openai/`、`claude/`；`convert/` 持有单一类型化 OpenAI-chat ⇄ Claude-messages 对
  （tools/tool_calls、流式 SSE chunk 形状），由一致性套件守门。`/v1/responses` 由
  OpenAI/兼容 upstream 服务（仅 HTTP/SSE）。
- **冷却状态机**（`MarkResult`）：按 (渠道, 模型) —— 429 → 指数，401/403 → 30min，
  404/不支持 → 12h，5xx → 1min，成功 → 重置。

### C2 —— 统计与监控
- **采集**：relay 构建 `UsageEvent`（token 从 OpenAI `usage` 块 / Claude
  `message_delta.usage` 解析，延迟、TTFT、状态、错误类别），对有界 channel（~16k）做**非阻塞
  发送**。**单消费协程**批量插入事实行（100 行 / 200ms）并把事件折叠进内存滚动缓冲；60s ticker
  UPSERT 汇总；关机时 flush。
- **[FIX 不静默丢弃]**：溢出时递增 `dropped_events` 计数并在仪表盘暴露，并（按配置）回退为同步
  插入，而非丢弃计费数据。
- **成本**：读取/聚合时由 `model_pricing` 计算（定价更正可对历史重算）。decimal 计算、按分量、
  正确的缓存 token 语义。
- **健康**：被动 `MarkResult` 写 `channel_model_health`；`/healthz` = 进程 + DB ping；
  主动探测可选（M5）。
- **仪表盘**：为快读汇总；事实表仅供钻取；保留期清理（默认 30 天原始日志，汇总保留）在启动 +
  每日，批量 DELETE。

### C3 —— 轻量化部署
- 单静态二进制；`go:embed` SPA + 迁移 + 定价种子。SQLite WAL + 读连接池 + 单写协程（SQLite
  只允许一个写者）。**[FIX 迁移]** 版本化步骤，含**表重建模式**（CREATE new → copy → drop →
  rename）在事务内 + 预迁移 `.db` 备份；不只是 `IF NOT EXISTS + ALTER`。
- `data/`：`proxy-hub.db`（WAL）· `auths/*.json`（0600）· `config.yaml`（热重载）· `logs/`。
  单容器、单 `/data` 卷、单端口；不需要 OAuth 式回调。

### C4 —— MCP 共享管理
- **SSOT → 投影**。`McpService` 编排：`UpsertServer / ToggleClient / DeleteServer /
  SyncTarget / SyncAll`；编辑时对前后启用位图做 diff → 写入新启用/仍启用的客户端，从禁用的移除。
- **Claude 写入器**（`clients/claude.go`）：**[FIX claude.json 保留]** 读取整个
  `~/.claude.json`，只替换 `mcpServers` 对象，**字节级保留其余每个顶层键（含 `projects`
  历史）**；剥离每个 server 的 UI 辅助字段；Windows 下对 `npx/npm/node/...` 做 `cmd /c` 包装并
  带 WSL-UNC-path 检测；随后原子写。
- **Codex 写入器**（`clients/codex.go`）：`pelletier/go-toml/v2` 仅对 `[mcp_servers.<id>]` 做
  外科手术式编辑，清理遗留 `[mcp.servers]`，stdio→command/args/env/cwd 与
  http/sse→url/http_headers 转换，扩展字段透传。
- **[FIX 并发写]** `fileio` 做原子 temp+rename、首次写前 `.bak`、拷贝源权限、父目录缺失则跳过，
  并对每个目标加锁 + 写前立即重读，使活跃客户端在读-写之间的改动不被覆盖。
- **目标**为显式（`mcp_sync_targets`）；HOME 自动探测为可选。同步单向；导入只读（冲突 ⇒
  翻开关位，绝不覆盖 spec；"spec differs"告警）。

## 5. 安全

| 关注点 | 做法 |
|---|---|
| 上游 API key | `data/auths/<id>.json`，`0600`，绝不入 SQLite、绝不记日志。 |
| 入站 key | sha256 哈希存 `api_keys.hash`；原文仅创建时显示一次；内存缓存带负缓存。 |
| 管理端鉴权 | 单一 admin key（环境 `ADMIN_KEY` 或首次运行生成并打印一次）；守护 `/admin/*` + `/v0/mcp/*`。**[FIX]** SPA 用 bearer token；对改文件端点做 origin 校验（OQ-6）。 |
| MCP spec（env/headers 可能含密钥） | DB + 写出文件继承受限权限；文档说明；env 变量间接化后置。 |
| 敏感请求数据 | IP/UA 可选开启；`error_message` 截断；绝不持久化请求/响应体。 |
| 客户端配置安全 | 整文件保留 + 原子写 + `.bak` + 锁；绝不破坏 `projects`/无关 TOML。 |

## 6. 参考项目要点（该借鉴什么，按文件）

- **new-api** —— `model/ability.go`（路由索引）、`relay/channel/adapter.go` +
  `relay/relay_adaptor.go`（adaptor 注册表）、`relay/helper/model_mapped.go`（链式映射 + 环
  检测）、`model/log.go` + `model/usedata.go`（双存储统计 + 内存缓冲 flush）、
  `model/channel_cache.go`（内存路由，免 Redis）、`model/main.go`（内嵌 SQLite）。**剔除**所有
  支付/订阅/媒体臃肿。
- **CLIProxyAPI** —— `sdk/cliproxy/auth/selector.go`（RR/加权/亲和）、`conductor.go`
  `MarkResult`（状态感知冷却）、`sdk/cliproxy/usage/manager.go`（Record + 异步队列形状）。
  **跳过** OAuth/utls/翻译矩阵/redisqueue。
- **sub2api** —— 统一渠道 schema（单表 + JSON 凭证）、通配符模型映射、
  `usage_record_worker_pool.go`（异步溢出理念——简化为单协程）。**拒绝** PG+Redis、Ent、
  ~400 文件 SaaS 面。
- **cc-switch** —— `services/mcp.rs`（SSOT 编排）、`mcp/codex.rs`（TOML 外科手术式编辑）、
  `claude_mcp.rs`（整文件 `mcpServers` 合并 + Windows `cmd /c`）、`config.rs`（原子写）、
  `database/{mod,schema}.rs`（内嵌 SQLite 生命周期、定价/汇总 schema）。**丢弃** Tauri 外壳；
  **修复**单 `Mutex<Connection>`（改用 WAL + 连接池）。

## 7. 上线 / 回滚形态

- 全新项目：每个里程碑落在自己的子任务、各自特性分支上，`trellis-check` 通过后并入 `main`。
  无生产数据需迁移。
- DB 迁移为前向式 + 预迁移 `.db` 备份；回滚 = 恢复备份文件 + 重新部署旧二进制。
- MCP 写入器是唯一在 proxy-hub `data/` 之外改文件的特性；其 `.bak` 备份即坏同步的回滚。
