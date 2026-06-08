# M3 —— 统计与监控：技术设计（Design）

> `06-04-m3-stats-monitoring` 子任务设计。继承父 `design.md`（能力 C2）与 M1/M2 已确立约定
> （`.trellis/spec/backend/`，尤其 `relay-channel.md`、`database-guidelines.md`）。
> 语言：规划文档与代码注释中文；标识符/表名/列名/路径/API 路径/技术名英文；`.sql` 查询文件 ASCII 注释。

## 0. 定位与约束

- **消费 M2**：relay 已发出 `UsageEvent`（有界 channel，非阻塞）+ 写 `channel_model_health`。M3 用真实
  采集器**替换** M2 的 `DrainAndDiscard` 占位消费者。
- **铁律**（父 NFR-2/3）：中转热路径**只做非阻塞发送**，请求路径**无 DB I/O**；金额用 `decimal`、
  **读取时计算**（不预存成本）；**绝不静默丢弃**计费数据；**绝不持久化请求/响应体**。
- 端口：后端 **7777**；前端开发 **8888**（Vite `server.proxy` 把 `/v1`·`/admin`·`/v0`·`/healthz` 代理到 7777）；
  **生产无独立前端端口**——`web/dist` 经 `go:embed` 由后端单端口提供（父 FR-C3-3）。

## 1. 关键取舍（规划期定）

- **OQ-A `UsageEvent`/`Emitter` 归属**：从 `relay` 抽出到中立包 **`internal/usage`**（`Event`、`Emitter`、
  `DrainAndDiscard`）。`relay` 与 `stats` 均依赖 `usage`，**消除 `stats → relay` 耦合与潜在环**。这是
  M3 对 M2 边界的一次合理重整（改动小：移 1 文件 + 调 relay/main 引用 + 更新 spec）。
- **OQ-B 成本存储**：`request_logs`/汇总表**只存 token/延迟**；`model_pricing` 单独表，成本在**查询时**
  按分量 `decimal` 计算 ⇒ 改价可对历史重算。
- **OQ-C 定价种子格式**：`pricing/seed.json` 用**本项目 per-million 模式**（`{model:{input_per_million,
  output_per_million,cache_read_per_million,cache_creation_per_million}}`，均字符串 decimal），手工策展
  常见 OpenAI/Claude 模型。比直接吞 LiteLLM 全量（per-token、巨大）更精简；管理端可覆盖/新增。
- **OQ-D 溢出策略**：默认**非静默丢弃 + 计数**（`stats.overflow` 经 `/admin/stats/overview` 暴露）；
  配置 `stats.sync_fallback_on_full=false`（默认）为 true 时改**同步插入**兜底，绝不丢计费。

## 2. 数据模型（迁移 `0003_stats.sql`）

> 金额相关只在 `model_pricing`；事实/汇总表只存 token/延迟。`channel_model_health` 已在 0002 建。

### request_logs —— append-only 事实表（钻取/追踪）
```
id INTEGER PK AUTOINCREMENT
request_id TEXT NOT NULL
created_at TEXT NOT NULL                 -- RFC3339
api_key_id INTEGER NOT NULL DEFAULT 0
channel_id INTEGER NOT NULL DEFAULT 0
user_id INTEGER NOT NULL DEFAULT 0       -- OQ-4 预留，M3 写 0
group_name TEXT NOT NULL DEFAULT 'default'
requested_model TEXT NOT NULL            -- 客户端面名（含 prefix）
upstream_model TEXT NOT NULL DEFAULT ''
endpoint_format TEXT NOT NULL            -- openai|claude|responses
is_stream INTEGER NOT NULL DEFAULT 0
input_tokens/output_tokens/reasoning_tokens/cache_read_tokens/cache_creation_tokens/total_tokens INTEGER NOT NULL DEFAULT 0
latency_ms INTEGER NOT NULL DEFAULT 0
first_token_ms INTEGER                    -- NULL=未知（TTFT）
status_code INTEGER NOT NULL DEFAULT 0
is_error INTEGER NOT NULL DEFAULT 0
error_type TEXT NOT NULL DEFAULT ''
error_message TEXT NOT NULL DEFAULT ''    -- 截断（<=256B）
session_id TEXT NOT NULL DEFAULT ''
usage_source TEXT NOT NULL DEFAULT ''     -- stream|usage_block|estimated|missing
```
索引：`(created_at)`、`(api_key_id,created_at)`、`(channel_id,created_at)`、`(requested_model,created_at)`、
`(status_code)`、`(session_id)`、`(request_id)`。**无成本列；绝不存请求/响应体**（无 ip/ua，M3 不开）。

### usage_hourly_rollups —— 预聚合（小时）
PK `(bucket_hour, channel_id, api_key_id, requested_model)`；`bucket_hour` 为 RFC3339 截到小时。
### usage_daily_rollups —— 预聚合（日）
PK `(bucket_date, channel_id, requested_model)`；`bucket_date` 为 `YYYY-MM-DD`。
两表共有列：`request_count, success_count, error_count, input_tokens, output_tokens, cache_read_tokens,
cache_creation_tokens, reasoning_tokens, sum_latency_ms, sum_first_token_ms, count_first_token`（均 INTEGER
NOT NULL DEFAULT 0）。**只存 token/延迟聚合，不预存成本**。UPSERT：`... ON CONFLICT(pk) DO UPDATE SET
x=x+excluded.x`。事实表是 SOT，汇总可重建。

### model_pricing —— 配置驱动定价
```
model_id TEXT PK
input_per_million / output_per_million / cache_read_per_million / cache_creation_per_million TEXT NOT NULL DEFAULT '0'
source TEXT NOT NULL DEFAULT 'seed'       -- seed|admin
updated_at TEXT NOT NULL
```
启动从 `pricing/seed.json`（`go:embed`）UPSERT 种入（`source='seed'`，不覆盖 `source='admin'` 的行）；
管理端可覆盖/新增（置 `source='admin'`）。未知模型 ⇒ 成本计算返回 0 + `pricing_missing=true` 标记。

索引：`request_logs` 见上；`usage_hourly_rollups(bucket_hour)`、`usage_daily_rollups(bucket_date)`。

## 3. internal/usage（从 relay 抽出）

```go
type Event struct { /* = 原 relay.UsageEvent 全字段 */ }
type Emitter struct { ch chan Event; dropped atomic.Int64 }
func NewEmitter(buffer int) *Emitter
func (*Emitter) Emit(Event)            // 非阻塞；满则 dropped++（不阻塞热路径）
func (*Emitter) Events() <-chan Event
func (*Emitter) Dropped() int64
func (*Emitter) Close()
func DrainAndDiscard(*Emitter) <-chan struct{}   // 保留：测试/降级用
```
relay 改 import `usage`：`Engine.emitter *usage.Emitter`、`Config.Emitter *usage.Emitter`、`fillUsage` 填
`usage.Event`。`relay.UsageResult`（adaptor 解析用）保持在 adaptor 包不动。

## 4. internal/stats

### 4.1 event.go —— 错误分类 + 桶键
- `ClassifyError(statusCode int, errType string) string`：归一 error_type（`upstream_error`/`rate_limit`/
  `auth`/`not_found`/`client_error`/`timeout`/`connection`...），供仪表盘分组。
- `HourBucket(t) string` / `DayBucket(t) string`：RFC3339 截小时 / `YYYY-MM-DD`（UTC 存储，前端按需转）。

### 4.2 collector.go —— 单消费协程
- `Collector{ dao, emitter, batchN=100, batchInterval=200ms, flushInterval=60s, syncFallback bool, now func() }`。
- `Run(ctx)`：单 goroutine select：
  - 从 `emitter.Events()` 收事件 → 入批缓冲 + 折叠进内存滚动缓冲（hourly/daily map[key]agg）。
  - 批满 100 或 200ms ticker ⇒ `dao.InsertLogsBatch`（单事务多行）。
  - 60s ticker ⇒ `dao.UpsertRollups`（hourly+daily，UPSERT 累加）后清滚动缓冲。
  - `<-ctx.Done()` ⇒ **最终 flush**（批 + 滚动缓冲）后返回；`done` channel 通知 main。
- **溢出**：`emitter.Emit` 已非阻塞计数；`syncFallback=true` 时 relay 侧在 channel 满改走同步 `dao.InsertLog`
  （经独立小接口，避免 relay→stats 直依赖：注入 `OnFull func(usage.Event)`）。默认仅计数。
- **写并发**：所有写经 `store.Write()` 单写句柄；批量 + 60s 合并，远低于 SQLite 单写者上限。

### 4.3 pricing.go —— 读取时成本
- `Table`：内存 `map[model]Price`（decimal），启动从 DB 载（DB 已含 seed + admin 覆盖），admin 改价时重载。
- `Cost(model, tokens) (Breakdown, missing bool)`：成本 = Σ(分量 token × per_million / 1e6)，`decimal` 计算。
  **缓存语义**：OpenAI `input_tokens` **含**缓存读 ⇒ 计费输入 = `input - cache_read`（再按 cache_read 单价单算）；
  Claude `input_tokens` 为**纯新** ⇒ 直接计费。endpoint_format/usage_source 决定走哪套（见 §6）。
- 改价对历史重算：成本不落库，仪表盘查询时即时算 ⇒ 改 `model_pricing` 后所有历史视图自动反映。

### 4.4 dao.go —— 批量/汇总/查询/清理
- `InsertLogsBatch([]usage.Event) error`（单事务多 INSERT）；可选 `InsertLog`（同步兜底）。
- `UpsertHourly/UpsertDaily([]Rollup) error`（ON CONFLICT 累加）。
- 仪表盘查询（见 §5）：overview/timeseries/breakdown/logs/health，全走 `store.Read()`，**聚合在 SQL 内**。
- `CleanupRawLogs(before time.Time) (deleted int64, err error)`：`DELETE FROM request_logs WHERE created_at < ?`
  批量（分页 LIMIT 避免长事务）；汇总不删。

## 5. 仪表盘读取 API（`/admin/stats/*`，admin key 守护）

| 端点 | 形状 |
|---|---|
| `GET /admin/stats/overview?range=24h` | 总 token/请求/错误率/成本（读时算）/平均延迟·TTFT + `overflow` 丢弃数 + `pricing_missing` 列表 |
| `GET /admin/stats/timeseries?range=7d&interval=hour\|day` | 时序点数组（从 rollups 读）：bucket、tokens、count、error、latency |
| `GET /admin/stats/breakdown?by=model\|channel\|api_key\|error_type&range=24h` | 分组聚合 + 读时成本 |
| `GET /admin/stats/logs?request_id=\|api_key_id=\|channel_id=\|model=&page=&size=` | 分页钻取 request_logs（倒序 created_at） |
| `GET /admin/stats/health` | `channel_model_health` 全量（is_healthy/cooldown_until/consecutive_failures/last_error） |
| `GET /admin/pricing` · `PUT /admin/pricing/:model` · `DELETE /admin/pricing/:model` | 定价 CRUD（admin 覆盖；改后重载内存表） |

- 成本计算在 handler 层用 `pricing.Table`，DTO 含 `cost`（字符串 decimal）+ `pricing_missing`。
- 时间范围解析 `range`（`24h`/`7d`/`30d`）→ 起止；timeseries 选 hourly/daily 表。

## 6. 缓存 token 语义（成本正确性，父 §3 model_pricing）

| 入站/上游 | input_tokens 含义 | 计费输入 token |
|---|---|---|
| OpenAI（`usage.prompt_tokens` + `prompt_tokens_details.cached_tokens`） | **含**缓存 | `input - cache_read`，cache_read 另按 cache_read 单价 |
| Claude（`message_delta.usage.input_tokens` + `cache_read_input_tokens`/`cache_creation_input_tokens`） | **纯新** | 直接用 `input`；cache_read/creation 各按其单价 |

M2 的 adaptor 已按各自语义填 `UsageEvent` 的 `InputTokens/CacheReadTokens/CacheCreationTokens`（openai：input
为 prompt 总含 cache；claude：input 为纯新）。pricing 据 `endpoint_format` 区分扣减，避免重复计费。

## 7. 前端（web/，React + TS + Vite + Tailwind + shadcn/ui + TanStack Query）

- 工程：`web/`（Vite）；构建产物 `web/dist`（M1 已有 `go:embed` 壳，M3 由真实构建覆盖）。
- 开发：`vite.config.ts` `server.port=8888` + `server.proxy` 把 `/v1`·`/admin`·`/v0`·`/healthz` → `http://127.0.0.1:7777`。
- 鉴权：admin key 存浏览器（输入一次，存 localStorage），`fetch`/TanStack Query 注入 `Authorization: Bearer <admin_key>`。
- 页面：①概览（卡片：token/请求/错误率/成本/延迟/TTFT/溢出）②趋势（timeseries 折线）③分组表（model/channel/key/error_type 切换）④请求日志钻取（分页 + request_id 搜索）⑤渠道健康（health 表）⑥渠道/Key/定价管理（复用 M2 admin API）。
- 状态：TanStack Query 拉取 + 缓存；shadcn/ui 组件；Tailwind 样式。
- **生产单端口**：`go build` 前 `npm run build` 产出 `web/dist` → `go:embed`（M1 的 `registerStatic` 已就绪，SPA history 回退已实现）。

## 8. 装配（main）

- 用 `usage.NewEmitter(cfg.Stats.UsageBuffer)` 取代原 relay 内联 emitter；同一 emitter 注入 `relay.NewEngine`
  与 `stats.NewCollector`。
- `collector.Run(ctx)` 取代 `relay.DrainAndDiscard`；停机时 `emitter.Close()` → collector 收尾 flush → 等 `done`。
- 启动：`pricing` 种子 UPSERT（embed）→ 载内存定价表；保留期清理跑一次 + 起每日 ticker。
- 路由：`api/stats_handlers.go` 注册到 `/admin/stats/*` + `/admin/pricing/*`（admin key 组）。

## 9. 配置（config.StatsConfig）

```
stats:
  retention_days: 30            # 原始日志保留；汇总永久
  usage_buffer: 16384           # 有界 channel 容量（沿用 relay.usage_buffer，迁移到 stats）
  batch_size: 100
  batch_interval_ms: 200
  flush_interval_s: 60
  sync_fallback_on_full: false  # true=channel 满改同步插入（绝不丢计费），默认 false=计数丢弃
```
环境 `PROXY_HUB_STATS_*` 覆盖；校验非负。`relay.usage_buffer` 迁到 `stats.usage_buffer`（保留兼容读取）。

## 10. 测试

- `stats/event`：错误分类、桶键（小时/日边界）。
- `stats/pricing`：decimal 成本、OpenAI 扣缓存 vs Claude 纯新、pricing_missing。
- `stats/collector`：合成 `usage.Event` 流 → 批量插入计数、滚动缓冲折叠、flush、**溢出计数非静默**、关机 flush（注入 `now` + 手动 tick）。
- `stats/dao`：InsertLogsBatch、UpsertRollups 累加、各仪表盘查询形状、CleanupRawLogs（删原始留汇总）。
- 前端：组件渲染 + API mock（vitest/RTL，按 `frontend/` 规范）；`npm run build` 干净。
- 端到端：合成流量 → 概览/趋势/钻取可见；改价历史重算；制造溢出仪表盘可见。

## 11. 验收映射（对应 prd 验收）

§4.2 非阻塞采集 ⇒ 热路径无 DB I/O；§5 仪表盘 ⇒ token/成本/延迟/TTFT/错误 + 钻取；§3.3+§6 读时成本 ⇒ 改价重算；
§4.2 溢出计数 ⇒ 非静默；§4.4 CleanupRawLogs ⇒ 保留期。`gofmt`/`vet`/`test` + web lint/build 干净 + `trellis-check`。

## 12. 回滚

特性分支；迁移 0003 前向式 + 预迁移备份（M1 框架）。前端纯增量（`web/dist` 覆盖）。`internal/usage` 抽取若
有问题可回退到 relay 内联（但会重新引入耦合）。定价种子仅 UPSERT seed 行，不破坏 admin 覆盖。
