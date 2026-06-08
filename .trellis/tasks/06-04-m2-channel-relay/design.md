# M2 —— 渠道管理 + API-Key 中转 + 模型映射：技术设计（Design）

> `06-04-m2-channel-relay` 的子任务设计。继承父 `design.md`（能力 C1）与 M1 已确立的约定
> （见 `.trellis/spec/backend/`）。此处细化到可直接编码。
> 语言约定：规划文档与代码注释一律中文（见 `AGENTS.md`）；标识符、表名/列名、API 路径、技术名保留英文。

## 0. 已定关键取舍（规划期确认）

- **OQ-1 多租户**：单运维。`api_keys` 不带 per-user 配额/RPM（OQ-4 延后）；`user_id` 暂不作一等维度
  （`request_logs.user_id` 列由 M3 预留，M2 写 0/空）。
- **OQ-2 DB 访问**：**sqlc 代码生成**。`.sql` 查询 → `sqlc generate` → 提交生成代码（Docker/CI 不需 sqlc 二进制）。
- **OQ-3 跨方言转换器**：放 M2，作为**最后一项**，置于**特性开关** `relay.enable_cross_dialect` 之后；
  一致性套件未过则关开关、仅发布同方言路由（可独立验收/上线）。

## 1. 模块与依赖

### 1.1 新增运行时依赖（保持最小）

- `github.com/tidwall/gjson` + `github.com/tidwall/sjson` —— **仅**用于从请求体提取/改写模型名与透传未知字段
  （承重的跨方言转换用类型化 struct，见 §7）。
- sqlc 生成代码仅依赖标准库 `database/sql`（无新增运行时依赖）。
- 出口代理用标准库 `net/http` + `net/url` + `golang.org/x/net`（已在 M1 间接依赖树）。
- 不在 M2 引入：`shopspring/decimal`（M3）、`pelletier/go-toml`（M4）。

### 1.2 sqlc 工具链

- 仓库根 `sqlc.yaml`：`engine: sqlite`；schema 指向 `internal/store/migrations`（复用迁移 SQL 作为 schema 源）；
  queries 指向 `internal/store/queries`；输出包 `internal/store/dbgen`（package `dbgen`）。
- 生成代码 **提交入库**；`internal/store/queries/*.sql` 变更后本地 `sqlc generate` 再提交。
- `dbgen.New(db dbgen.DBTX)`：`DBTX` 由 `*sql.DB` 与 `*sql.Tx` 同时满足。
  - 读：`dbgen.New(store.Read()).XxxQuery(ctx, …)`。
  - 写：`dbgen.New(store.Write()).XxxExec(ctx, …)`。
  - 事务（多写 + 重建 abilities）：`tx,_:=store.Write().BeginTx(ctx,nil); q:=dbgen.New(tx); …; tx.Commit()`。

### 1.3 目录（本里程碑落地）

```
sqlc.yaml
internal/store/
  migrations/0002_channels.sql         # channels/abilities/api_keys/channel_model_health
  queries/{channels,abilities,api_keys,health}.sql
  dbgen/                               # sqlc 生成（提交）
internal/channel/
  model.go        # Channel/Ability/ChannelRuntime 领域类型 + 映射解析
  dao.go          # 渠道/ability/api_key 持久化（包裹 dbgen + 事务）
  routeindex.go   # 内存 RouteIndex：model -> []*ChannelRuntime，增量重建
  manager.go      # 装配 dao + credstore + routeindex；渠道保存编排（含 ability 重建）
internal/credstore/store.go            # data/auths/<id>.json（0600）+ fsnotify 重载 + 内存缓存
internal/adaptor/
  adaptor.go      # Adaptor 接口 + 注册表
  openai/openai.go  claude/claude.go   # 同方言透传 + DoRequest/DoResponse
  convert/{openai_claude.go, sse.go, suite_test.go}  # 跨方言（开关后）
internal/selector/{selector,roundrobin,weighted,affinity}.go
internal/relay/
  relay.go        # 请求流编排：鉴权→提取→路由→选择→适配→调用→重试→发 UsageEvent
  markresult.go   # 按 (渠道,模型) 冷却状态机 → channel_model_health
  usageevent.go   # UsageEvent 定义（M3 消费；M2 只发出）
internal/api/
  middleware/auth.go                   # 升级：真实入站 key 鉴权（替换 M1 雏形）
  relay_handlers.go                    # /v1/chat/completions、/v1/messages、/v1/responses、/v1/models
  admin_handlers.go                    # 渠道 CRUD + channel-test
  apikey_handlers.go                   # 入站 key CRUD（创建时显示一次明文）
```

## 2. 数据模型（迁移 `0002_channels.sql`）

> 凭证 blob **不入 DB**（存 `data/auths/<id>.json`，0600）。金额/成本不在 M2。

### channels —— 渠道元数据
```
id INTEGER PK AUTOINCREMENT
name TEXT NOT NULL
enabled INTEGER NOT NULL DEFAULT 1
platform TEXT NOT NULL CHECK(platform IN ('openai','anthropic'))
type TEXT NOT NULL CHECK(type IN ('api_key','upstream'))
base_url TEXT NOT NULL DEFAULT ''        -- 空 ⇒ 用 platform 默认端点
group_name TEXT NOT NULL DEFAULT 'default'  -- 'group' 是 SQL 保留字，列名用 group_name
priority INTEGER NOT NULL DEFAULT 50
weight INTEGER NOT NULL DEFAULT 1
models TEXT NOT NULL DEFAULT '[]'        -- JSON 数组：本渠道支持的上游模型名
model_mapping TEXT NOT NULL DEFAULT '{}' -- JSON：alias_model -> upstream_model（支持尾部 *）
prefix TEXT NOT NULL DEFAULT ''          -- 命名空间前缀（客户端面）
proxy_url TEXT NOT NULL DEFAULT ''       -- 出口代理（可空）
status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','error','disabled'))
error_message TEXT NOT NULL DEFAULT ''
created_at TEXT NOT NULL                 -- RFC3339
updated_at TEXT NOT NULL
```

### abilities —— 派生路由索引（new-api 模式；DB 持久镜像，内存另有 RouteIndex）
```
group_name TEXT NOT NULL
alias_model TEXT NOT NULL                -- 客户端面键（含通配 'gpt-4*' 形态）
channel_id INTEGER NOT NULL
upstream_model TEXT NOT NULL             -- 解析出的上游名
priority INTEGER NOT NULL
weight INTEGER NOT NULL
enabled INTEGER NOT NULL
PRIMARY KEY (group_name, alias_model, channel_id)
FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE
```
- 渠道保存时**增量** upsert/delete（绝不 TRUNCATE）；同事务内重建该渠道的行。
- `alias_model` 严格客户端面；`upstream_model` 一并存储（修父设计的命名空间碰撞）。

### api_keys —— 入站平台发放 key
```
id INTEGER PK AUTOINCREMENT
hash TEXT NOT NULL UNIQUE                 -- sha256(明文)，hex
name TEXT NOT NULL DEFAULT ''
group_name TEXT NOT NULL DEFAULT 'default'
enabled INTEGER NOT NULL DEFAULT 1
created_at TEXT NOT NULL
last_used_at TEXT                         -- NULL 允许
```
- 明文仅创建时返回一次；DB 只存 sha256。索引 `hash`（UNIQUE 即索引）。

### channel_model_health —— 被动健康/冷却（per 渠道×模型）
```
channel_id INTEGER NOT NULL
model TEXT NOT NULL                        -- upstream_model
is_healthy INTEGER NOT NULL DEFAULT 1
consecutive_failures INTEGER NOT NULL DEFAULT 0
last_success_at TEXT
last_failure_at TEXT
last_error TEXT NOT NULL DEFAULT ''
cooldown_until TEXT                        -- RFC3339；NULL/过去 ⇒ 未冷却
updated_at TEXT NOT NULL
PRIMARY KEY (channel_id, model)
FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE CASCADE
```

索引：`abilities(group_name, alias_model)`、`channels(enabled)`、`channel_model_health(cooldown_until)`。

## 3. credstore —— 文件级凭证

- 路径 `data/auths/<channel_id>.json`，权限 **0600**；目录 `data/auths` 0700（M1 已建）。
- 形状：`api_key` ⇒ `{"api_key":"..."}`；`upstream` ⇒ `{"base_url":"...","api_key":"..."}`。
- `Store`：启动全量加载 + `fsnotify` 监听 `auths/` 目录变更（增量重载单文件）；内存 `map[int]Cred` + RWMutex。
- 接口：`Get(channelID) (Cred, bool)`、`Put(channelID, Cred)`（原子写：temp+rename，0600）、`Delete(channelID)`。
- **绝不**记录 key 到日志；`Cred` 不进 SQLite、不进 `UsageEvent`。

## 4. RouteIndex —— 内存路由（热路径不碰 DB）

- 结构：`map[groupName]map[aliasModel][]*ChannelRuntime`，外加每渠道通配项的有序列表（用于最长匹配）。
- `ChannelRuntime`：`ChannelID, UpstreamModel, Priority, Weight, Platform, Type, BaseURL, ProxyURL`（不含 key——key 在 credstore）。
- 构建：启动从 `abilities`(enabled) + `channels`(enabled) 装配；渠道 upsert/delete 时**增量**替换该渠道项（写锁），其余不动。
- 查询 `Candidates(group, requestedModel) ([]*ChannelRuntime, resolvedUpstream)`：见 §6 解析顺序。
- 与 cooldown 解耦：RouteIndex 只给候选；冷却过滤在 selector（读 `channel_model_health` 的内存镜像）。

## 5. Adaptor —— 上游适配

- 接口（父设计）：
  ```go
  type Adaptor interface {
      // 同方言：构造发往上游的 *http.Request（透传 body + 改写模型名）。
      BuildRequest(ctx, in *RelayInput, rt *ChannelRuntime, cred Cred) (*http.Request, error)
      // 处理上游响应：流式/非流式回写客户端，解析 usage 填入 UsageEvent。
      HandleResponse(ctx, resp *http.Response, w http.ResponseWriter, ev *UsageEvent) error
  }
  ```
- 实现 `openai/`（OpenAI chat/responses）、`claude/`（messages）。同方言为**透传**：
  - 用 gjson 取/改 `model`；保留其余字段原样（不做类型化往返），按 stream 标志走 SSE 或一次性。
  - usage 解析：OpenAI `usage`（含缓存语义，input 含 cache）/ Claude `message_delta.usage`（input 为纯新）；
    M2 只**填** `UsageEvent` 的 token 字段并发出，**不**算成本（M3）。
- `/v1/responses` 仅由 OpenAI/兼容 upstream 服务（HTTP/SSE）。
- 出口代理：若 `rt.ProxyURL` 非空，HTTP client 用该代理；否则直连。每渠道独立 client（带连接复用 + 超时）。

## 6. 模型映射与路由解析（固定顺序）

客户端请求 model = `M`，group = `G`（来自入站 key 的 group）。流程：
1. **剥离 prefix**：若 `M` 以某渠道 `prefix` 开头，记录候选 prefix 并去前缀得 `M'`；否则 `M'=M`。
2. **剥离 `[1M]` 等长上下文后缀**（保留客户端面原名用于统计）。
3. 解析顺序（在 `abilities`/RouteIndex 内）：
   - **精确别名**：`alias_model == M'` 命中 ⇒ 候选集 + 各自 `upstream_model`。
   - **最长 `*` 通配**：形如 `gpt-4*`，取**最长前缀**匹配者；映射目标可含 `*` 透传剩余段。
   - **原样上游名**：以上未命中 ⇒ 把 `M'` 当上游名，命中 `models` 含 `M'` 的渠道。
4. 候选集交给 selector。**保留客户端面 `M`（含 prefix）** 写入 `UsageEvent.requested_model`；`upstream_model` 另存。
- 保存渠道时校验 `model_mapping`：同 (group, alias) 的多渠道允许（负载均衡），但**单渠道内**别名不得冲突；通配与精确并存时以解析顺序消歧并告警。

## 7. 选择器 + 重试

- `selector.Pick(candidates, sessionID) (*ChannelRuntime, error)`：
  1. **冷却过滤**：`isBlockedForModel(channelID, upstreamModel)` 读 `channel_model_health` 内存镜像，跳过 `cooldown_until > now`。
  2. **会话亲和**：sessionID（取自 header `X-Session-Id` → 请求 metadata → 内容哈希兜底）映射到上次成功渠道；仍健康则优先。
  3. **优先级档**：取最高 `priority` 档；档内 **加权随机**（weight）。
- 重试循环（`relay.go`）：可重试失败（429/5xx/连接错误）⇒ `MarkResult(cooldown)` + 从候选集排除该渠道 + 重选，至多 `relay.max_retries`（默认 2）。不可重试（4xx 非 429）⇒ 翻译错误停止。
- 亲和缓存：内存 `map[sessionID]channelID` + TTL（默认 10min），关机丢弃（无需持久化）。

## 8. MarkResult —— 冷却状态机（per 渠道×模型）

写 `channel_model_health`（经单写句柄；高频但量可控，M2 同步写即可，M3 不依赖它落库）：
- `429` ⇒ 指数退避（base 1s，×2，cap 5min），`consecutive_failures++`。
- `401/403` ⇒ 冷却 30min（凭证问题）。
- `404/不支持` ⇒ 冷却 12h。
- `5xx/连接错误` ⇒ 冷却 1min。
- **成功** ⇒ `consecutive_failures=0`、`cooldown_until=NULL`、`is_healthy=1`、`last_success_at=now`。
- 内存镜像：`channel_model_health` 维护在内存 map（写时同步落库 + 更新镜像），selector 读镜像（热路径不查 DB）。

## 9. 统一端点 + 入站鉴权

- 路由（在 M1 的 protected 组之外，relay 单独组，走入站 key 鉴权而非 admin key）：
  - `POST /v1/chat/completions`（OpenAI chat）
  - `POST /v1/messages`（Claude messages）
  - `POST /v1/responses`（OpenAI Responses）
  - `GET /v1/models`（列表形状按入站 UA/路径在 OpenAI vs Claude 间切换）
- **入站鉴权中间件**（升级 M1 雏形）：从 `Authorization: Bearer` 或 `x-api-key` 取明文 → sha256 → 查内存 key 缓存（含负缓存）→ 命中且 enabled 放行，注入 `api_key_id`/`group`；否则 401。
  - 缓存：`map[hash]apiKeyMeta` + 负缓存集；`api_keys` 变更时失效。比较哈希用 `crypto/subtle.ConstantTimeCompare`（同时修 M1 admin auth 的非常量时间比较）。
- **管理端**（admin key，沿用 M1 auth）：`/admin/channels` CRUD、`/admin/channels/:id/test`、`/admin/api-keys` CRUD。

## 10. 跨方言转换器（开关后，最后做）

- `convert/openai_claude.go`：单一类型化对 —— OpenAI chat ⇄ Claude messages。覆盖：
  - 消息角色/系统提示、`tools`/`tool_calls`↔`tool_use`/`tool_result`、`stop`/`max_tokens` 等参数映射。
  - 流式：`sse.go` 把两边的 SSE chunk 形状互转（OpenAI `chat.completion.chunk` ⇄ Claude `content_block_delta` 等）。
- `relay` 中：候选渠道 platform 与入站方言不一致时，经转换器；一致则透传。
- **特性开关** `relay.enable_cross_dialect`（默认 false，套件绿后置 true）。
- `suite_test.go`：一致性套件（请求/响应、流式、工具）；未过 ⇒ 开关保持 false，同方言路由不受影响、可独立上线。

## 11. UsageEvent（M2 只发出，M3 消费）

`usageevent.go` 定义结构（token 拆分 input/output/reasoning/cache_read/cache_creation、latency、TTFT、status、error_type、requested_model/upstream_model、endpoint_format、is_stream、session_id、api_key_id、channel_id、group、usage_source）。M2 经一个**有界 channel** 非阻塞发出（消费者是 M3 采集器；M2 阶段可挂一个 drain-and-discard 的占位消费者，避免阻塞）。**绝不**含密钥/请求体。

## 12. 安全

| 关注点 | 做法 |
|---|---|
| 上游 key | credstore `data/auths/<id>.json` 0600；不入 DB、不记日志、不进 UsageEvent。 |
| 入站 key | sha256 存 `api_keys.hash`；明文仅创建时返回一次；内存缓存 + 负缓存；比较用 `subtle.ConstantTimeCompare`。 |
| 管理端 | admin key（M1）守护 `/admin/*`；relay 端点走入站 key。 |
| 请求/响应体 | 绝不持久化；`error_message` 截断；透传不落盘。 |
| 出口代理 | per 渠道；凭证经 client 注入，不写日志。 |

## 13. 验收映射

§2/§4/§6 ⇒ FR-C1-1/4/5；§9 ⇒ FR-C1-3 + 统一端点；§7 ⇒ FR-C1-6；§8 ⇒ FR-C1-7；§5/§10 ⇒ FR-C1-4（同方言/跨方言）；§11 ⇒ FR-C2-1（发出）/FR-C2-6（健康）。对应 M2 `prd.md` 验收清单逐条可演示。转换器经开关 ⇒ 即使套件未过也可发布同方言路由（回滚形态）。

## 14. 回滚 / 上线形态

- 特性分支；DB 迁移 0002 前向式 + 预迁移备份（M1 框架自带）。回滚 = 恢复备份 + 旧二进制。
- 跨方言转换器由 `relay.enable_cross_dialect` 守门：套件未过则关开关，仅发布同方言路由（核心路径），不阻塞 M2 验收的其余项。
- credstore 文件写为原子 temp+rename + 0600；误写可删文件 + 渠道置 error。
