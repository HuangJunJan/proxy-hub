# Proxy Hub - Technical Design (v1)

> Scope: 实现 PRD v1 的全部 FR/NFR/AC。  
> Reading order: 先看"模块全景图"建立心智模型，再看"请求生命周期"理解关键路径，最后翻"YAML / SQLite Schema"等参考章节。

---

## 1. 模块全景

```
proxy-hub/
├── cmd/proxy-hub/main.go            # 入口：解析 flag，启动 server，自动开浏览器
├── internal/
│   ├── config/                       # YAML 加载、热重载、原子写、watcher
│   ├── store/                        # SQLite repository 抽象 (request_log, channel_stat, schema 迁移)
│   ├── auth/                         # 控制台 session 与下游 API Key 鉴权
│   ├── upstream/
│   │   ├── adapter.go                # Upstream interface (Chat, Models, HealthCheck)
│   │   ├── openai/                   # OpenAI-compatible adapter (baseUrl + key)
│   │   └── chatgpt/                  # ChatGPT OAuth adapter + 刷新逻辑
│   ├── router/                       # 模型 → 渠道集合的解析与选路
│   ├── scheduler/                    # 优先级 + round-robin + 熔断
│   ├── proxy/                        # /v1/chat/completions /v1/models handler，流式转发 + Token 计数
│   ├── monitor/                      # 请求日志写入 + SSE fan-out + 统计聚合
│   ├── admin/                        # 控制台 REST API (channels / keys / logs / stats / setup)
│   ├── web/                          # 内嵌的前端静态资源（//go:embed dist）
│   └── server/                       # gin/chi 路由组装与中间件
├── web/                              # React + Vite 源码
│   ├── src/
│   │   ├── pages/                    # route-level setup / login / dashboard / channels / keys / logs / live / settings
│   │   ├── features/                 # domain UI sections reused by pages
│   │   ├── components/
│   │   │   ├── layout/               # AppShell and cross-route layout
│   │   │   └── ui/                   # shadcn-style source components
│   │   ├── lib/                      # axios client、types、theme、i18n、SSE hooks、app context
│   │   ├── App.tsx                   # boot flow + route table only
│   │   └── main.tsx                  # React root + BrowserRouter
│   └── components.json               # shadcn local component aliases
├── config.example.yaml               # 带注释的参考配置
├── go.mod
└── ...
```

### 1.1 关键边界

| 模块 | 唯一职责 | 不做的事 |
|---|---|---|
| `config` | YAML I/O + 热加载 + 原子写 | 不做业务校验逻辑（业务层取快照后自己 validate） |
| `store` | SQLite CRUD + schema 迁移 | 不存储任何配置数据 |
| `upstream/*` | 各类上游的协议适配 + 健康检查 + OAuth 刷新 | 不做选路 / 不做熔断 |
| `router` | 下游 model → 命中渠道集合 | 不做权重选择 |
| `scheduler` | 命中集合内按 priority + round-robin 选 + 熔断状态机 | 不直连上游 |
| `proxy` | 鉴权 → router → scheduler → upstream → 流式回写 + Token 累计 | 不做统计聚合 |
| `monitor` | 请求日志写库 + SSE 推送 + 每分钟滚动聚合 | 不影响主链路延迟（异步 channel） |
| `admin` | 控制台 REST API | 不直接读 SQLite（透过 store 接口） |

### 1.2 v2 演进口子

- `upstream.Adapter` interface → 加 Claude / Gemini 实现即可。
- `proxy` 接受 `DownstreamProtocol` 参数 → 后续加 Anthropic Messages handler 复用 router/scheduler。
- `store` 是接口 → 加 Postgres 实现替换 SQLite。
- `auth` 用户模型现在固定单 admin → 字段已留 `userId`，扩展多用户只改 auth + admin API。

---

## 2. YAML Schema

### 2.1 完整示例（包含所有字段）

```yaml
# 监听
server:
  port: 8787
  # host: "0.0.0.0"               # 默认 "0.0.0.0"，omitempty 不写
  # open-browser: false           # 默认 true

# 控制台管理员（首次启动向导写入）
admin:
  username: admin
  password-hash: $argon2id$v=19$m=65536,t=3,p=2$...

# 下游 API Keys
api-keys:
  - token: sk-proxy-hub-AbCd1234EfGh5678...   # 明文，鉴权用 constant-time 比较
    name: cursor-key-1                         # 可选，便于监控页区分
    # notes: ""                                # 可选
    # disabled: false                          # 默认启用
  - token: sk-proxy-hub-IjKl9012MnOp3456...
    name: claude-code

# 上游：OpenAI 兼容（可挂多 key 凭证池；models 可为空，空时默认透传全部模型）
openai-api:
  - name: openai-official                      # 唯一标识
    base-url: https://api.openai.com
    priority: 10                               # 默认 100，越小越先用
    api-key-entries:                           # 凭证池：多 key 自动 round-robin
      - api-key: sk-xxx
      - api-key: sk-yyy
        # proxy-url: socks5://...              # 可选，按 key 覆盖代理
    models:
      - name: gpt-4o                           # alias 缺省 = name
      - name: gpt-4o-mini

  - name: deepseek
    base-url: https://api.deepseek.com
    priority: 20
    api-key-entries:
      - api-key: sk-xxx
    models:
      - name: deepseek-chat                    # 下游可见 = deepseek-chat（alias 缺省 = name）
      - name: deepseek-chat
        alias: gpt-5.4                         # 同一上游模型多别名：再加一条
      - name: deepseek-reasoner

# 上游：ChatGPT OAuth
chatgpt-oauth:
  - name: my-chatgpt-plus
    # timeout-sec: 180                         # 可选，默认 120
    oauth:
      access-token: eyJh...                    # 后端按需刷新时回写
      refresh-token: rt_xxx
      expires-at: 2026-05-26T11:00:00Z
    models:
      - name: gpt-5-codex
      - name: gpt-4.1

# 请求日志
request-log:
  retention-days: 7
  body-mode: failed_only                       # failed_only | always | none
  # max-body-bytes: 65536                      # 默认 65536

# 调度
scheduler:
  max-retries: 2                               # 默认 2
  circuit-cooldown-sec: 60                     # 默认 60
  circuit-failure-threshold: 3                 # 默认 3

# 跨域（默认关闭）
# cors:
#   allowed-origins: ["https://my-app"]
```

### 2.2 字段约束

- **标识**：所有顶层数组成员的唯一键都是 `name`（`api-keys[]` 的 name 可选，省略时控制台展示掩码 token）。同一 section 内 name 必须唯一。
- **`priority`** 仅 `openai-api` 类生效（越小越先），缺省 100；`chatgpt-oauth` 忽略 priority。
- **`models[]` 语义**：每项 = "一对 (上游真实名, 下游可见名)"。对 `openai-api`，`models[]` 是可选的显式枚举 / 别名映射表；为空时请求的下游模型名默认原样传给上游。对 `chatgpt-oauth`，`models[]` 仍必填。
  - `openai-api` 空 `models[]` → 不枚举显式模型，但所有下游模型名默认原样透传
  - 单条 `{name: gpt-4o}` → alias 缺省为 gpt-4o，把 gpt-4o 加入 `/v1/models` 的显式枚举，转发时仍是 1:1
  - 单条 `{name: deepseek-chat, alias: gpt-5.4}` → 下游 gpt-5.4 路由到上游 deepseek-chat
  - 多条共享 alias（`{name: glm-5, alias: claude-opus-4.66}` + `{name: deepseek-v3.1, alias: claude-opus-4.66}`）→ 该 alias 在此渠道内形成 round-robin 池
  - 同一 name 配多 alias（重复条目，不同 alias）→ 给该上游模型加多个下游可见名
- **`api-key-entries[]`** ≥ 1 条；为空视为渠道不可用。
- **`token`** 字符集 `[A-Za-z0-9_-]+`，长度 ≥ 32。
- **`oauth.expires-at`** ISO8601；刷新时更新；不存或已过期触发首次同步刷新。
- **`omitempty` 应用**：所有可选字段、bool 默认值（false）、enum 默认值在序列化时省略。

### 2.3 校验

- 启动时 + 每次热加载后跑 `Validate(cfg)`：
  - 唯一性（每个 section 内 `name`、`api-keys[].token`、单渠道内 `models[]` 的 (name, alias) 二元组）
  - 必填字段、URL 合法性、enum
  - 至少 1 个启用渠道（否则告警但不退出）
- 校验失败：启动时退出非零；热加载时**保留旧快照** + 控制台显示错误。
- 启动时若发现 admin 的 password-hash 是明文（不是以 `$argon2id$` 开头）则自动哈希并回写，参考 CLIProxyAPI 的做法。

---

## 3. SQLite Schema

```sql
-- 请求日志（FR-7 单条）
CREATE TABLE request_logs (
  id                  INTEGER PRIMARY KEY AUTOINCREMENT,
  ts                  INTEGER NOT NULL,         -- unix ms
  api_key_token_mask  TEXT NOT NULL,            -- 掩码后的 token（sk-...AbCd），便于展示且不泄露明文
  api_key_name        TEXT,                     -- 来自 api-keys[].name，可空
  channel_name        TEXT,                     -- 命中的最终渠道 name（可空：路由失败）
  channel_type        TEXT,                     -- openai-api | chatgpt-oauth
  downstream_model    TEXT NOT NULL,            -- 下游请求模型名（即 alias）
  upstream_model      TEXT,                     -- 实际上游模型名（alias 解析后的 name）
  upstream_key_index  INTEGER,                  -- 命中的 api-key-entries 索引（多 key 池调试用）
  status_code         INTEGER NOT NULL,
  is_stream           INTEGER NOT NULL,         -- 0/1
  duration_ms         INTEGER NOT NULL,
  prompt_tokens       INTEGER,
  completion_tokens   INTEGER,
  total_tokens        INTEGER,
  error_kind          TEXT,                     -- timeout | upstream_5xx | auth | model_not_found | ...
  error_message       TEXT,
  request_body        BLOB,                     -- 按 body-mode 决定是否填
  response_body       BLOB,
  attempts            INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_request_logs_ts ON request_logs(ts);
CREATE INDEX idx_request_logs_channel_ts ON request_logs(channel_name, ts);
CREATE INDEX idx_request_logs_status_ts ON request_logs(status_code, ts);

-- 每小时聚合（避免对 request_logs 大表 group by）
CREATE TABLE channel_stats_hourly (
  channel_name        TEXT NOT NULL,
  hour_ts             INTEGER NOT NULL,         -- 整点 unix ms
  requests            INTEGER NOT NULL DEFAULT 0,
  successes           INTEGER NOT NULL DEFAULT 0,
  failures            INTEGER NOT NULL DEFAULT 0,
  prompt_tokens       INTEGER NOT NULL DEFAULT 0,
  completion_tokens   INTEGER NOT NULL DEFAULT 0,
  total_tokens        INTEGER NOT NULL DEFAULT 0,
  avg_duration_ms     INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (channel_name, hour_ts)
);

-- 元数据（schema 版本、最后清理时间等）
CREATE TABLE meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
```

> **注意**：`channel_name` 是渠道改名后的"历史快照"。改名 = 删旧 + 建新，历史日志保留旧 name，新统计聚合到新 name。这是用 name 当标识的可接受代价。

### 3.1 写入策略

- 主链路（`proxy`）只往 `monitor` 的内存 channel `chan LogEntry` 投递；不直接写库。
- `monitor` 启一个 goroutine，**批量** 写入 `request_logs`（batch 50 / 100ms 触发）。
- 聚合：每 5 秒 flush 一次内存累加器到 `channel_stats_hourly`（UPSERT）。
- 清理：每小时跑一次 `DELETE FROM request_logs WHERE ts < ?`，按 `retentionDays`；`channel_stats_hourly` 不清理（年级别也不会大）。

### 3.2 索引选择理由

- 实时监控页按 `ts DESC` 翻页 → `idx_request_logs_ts`
- 渠道详情按 `(channel, ts)` 翻页 → `idx_request_logs_channel_ts`
- 失败列表按 `status_code` 过滤 → `idx_request_logs_status_ts`

---

## 4. 请求生命周期（关键路径）

```
HTTP POST /v1/chat/completions
        │
        ▼
[middleware] auth.BearerKey                    # 401 invalid_api_key
        │
        ▼
[proxy.Handler]
   ├─ 解析 body：取 model (= alias), stream
   ├─ router.Resolve(alias)                    # 先查显式 alias；未命中则 openai-api 默认透传
   │     ├─ 无显式命中且无启用 openai-api → 404 model_not_found
   │     └─ 返回 []Hit{ Channel, UpstreamModelName }（透传时 UpstreamModelName = alias）
   ├─ scheduler.Pick(集合)                     # 按 priority + round-robin + 熔断
   │     └─ 循环 max-retries+1:
   │           ├─ 选下一个未熔断的 hit
   │           ├─ adapter.Chat(ctx, hit)
   │           │     ├─ openai-api:
   │           │     │     - 从渠道 api-key-entries 按 round-robin 选 key
   │           │     │     - 替换 body.model = hit.UpstreamModelName
   │           │     │     - 拼 base-url + Authorization
   │           │     └─ chatgpt-oauth: ensureFresh(token) → 上游
   │           ├─ 成功 → break
   │           └─ 失败 → 标记失败 / 熔断 / 下一个
   ├─ stream=true: 边读边 SSE 转发 + Token 统计
   ├─ stream=false: 一次性 JSON
   ├─ defer monitor.Submit(LogEntry{
   │       channel_name, channel_type,
   │       downstream_model = alias,
   │       upstream_model = hit.UpstreamModelName,
   │       upstream_key_index, ...})
   └─ return
```

### 4.1 流式 Token 统计

- OpenAI Chat 流式协议在最后一个 chunk（`finish_reason != null`）附带 `usage`，按此累计。
- 上游不返 `usage` 时（部分兼容服务商）：
  - prompt_tokens 用 tiktoken 估算（`pkoukk/tiktoken-go`）
  - completion_tokens 按 chunk 累加估算
- 断流（client 关连接）：把"已传 bytes"折算 token，标 `error_kind = client_canceled`

### 4.2 错误归一

| 上游表现 | error_kind | 返回状态 | OpenAI body shape |
|---|---|---|---|
| 401/403 | `upstream_auth` | 500（不外露上游凭证错） | `{error:{type:"upstream_error",code:"auth_failed"}}` |
| 429 | `upstream_rate_limited` | 429 | 标准 rate_limit_exceeded |
| 5xx | `upstream_5xx` | 502 | bad_gateway |
| timeout | `upstream_timeout` | 504 | gateway_timeout |
| 全部熔断 | `no_available_channel` | 503 | service_unavailable |
| 未找到模型 | `model_not_found` | 404 | model_not_found |

---

## 5. 调度算法

### 5.1 索引结构（启动 / reload 时构建）

```go
// alias -> 显式候选渠道清单（含 alias 在该渠道下对应的上游模型名）
type HitEntry struct {
    Channel          *ChannelRuntime
    UpstreamModelName string   // alias 在该渠道里对应的 name
}
type Index struct {
    // 一个 alias 在同一渠道内可能对应多个上游 name（pool），所以 value 是 []HitEntry
    aliasToHits map[string][]HitEntry
    openAIPassthrough []HitEntry // enabled openai-api channels; Resolve 未命中显式 alias 时使用
}
```

构建规则：
- 遍历 enabled `openai-api` 渠道，先加入 `openAIPassthrough`；遍历其 `models[]` 项：
  - `effectiveAlias = item.alias ?? item.name`
  - 把 `HitEntry{channel, item.name}` 追加到 `aliasToHits[effectiveAlias]`
- 遍历 enabled `chatgpt-oauth` 渠道时只读取 `models[]`，不加入透传候选
- alias 大小写规范化：lowercase 比较（避免客户端大小写不一致）
- `GET /v1/models` 只返回 `aliasToHits` 的显式 union；默认透传能力不能也不应该被展开成无限模型列表

### 5.2 ChannelRuntime

```go
type ChannelRuntime struct {
    Name          string         // 唯一标识
    Type          string         // openai-api | chatgpt-oauth
    Priority      int            // openai-api only, 默认 100
    Adapter       upstream.Adapter

    // 多 key 池
    KeyEntries    []KeyEntry     // openai-api
    keyCursor     atomic.Uint64

    // 熔断状态
    State         atomic.Int32   // 0=closed, 1=open
    OpenedAt      atomic.Int64
    ConsecFailures atomic.Int32

    // round-robin 游标（同优先级桶共用，由 scheduler 持有，不在此处）
}
```

### 5.3 Pick 算法（伪码）

```go
func Pick(hits []HitEntry) iter.Seq[HitEntry] {
    // 1. 过滤：渠道 enabled && 未熔断（或熔断已过冷却 → half-open 让 1 个试探）
    alive := filterAlive(hits)
    // 2. 分桶：openai-api 按 channel.priority 升序；chatgpt-oauth 全部一个低优先级桶
    buckets := groupByPriority(alive)
    // 3. 升序遍历桶；桶内按 rrCursor++ 取
    return func(yield func(HitEntry) bool) {
        for _, b := range buckets {
            for _, hit := range rotate(b, b.cursor.Add(1)) {
                if !yield(hit) { return }
            }
        }
    }
}
```

调用方 `for hit := range Pick(hits)` 最多迭代 `max-retries + 1` 次。

> 注意：同一渠道内"alias 对应多 name"形成的 pool 在构建 Index 时已展开成多个 HitEntry，所以这里 Pick 直接平铺 round-robin，不需要嵌套两层。

### 5.3 熔断

- closed → 连续 3 次失败（可配）→ open
- open + 冷却到期 → half-open，允许 1 次试探
- half-open 成功 → closed；失败 → open + 重置冷却

---

## 6. YAML 热加载与原子写

### 6.1 真相源 + 内存快照

```go
type Config struct{ ... }            // 不可变快照（指针 + atomic.Value）

type Manager struct {
    path     string
    snapshot atomic.Pointer[Config]
    fileLock *flock.Flock            // 跨 goroutine + 跨进程
    saveMu   sync.Mutex              // 写入串行
}
```

- 读：`m.snapshot.Load()` 拿当前快照，调用方在请求生命周期内复用。
- 写（来自 admin API 或 OAuth refresh）：
  1. `saveMu.Lock`
  2. 克隆当前 snapshot → 修改 → `Validate`
  3. `fileLock.Lock` → 临时文件 `config.yaml.tmp` → `yaml.Marshal` → `os.Rename` → `fileLock.Unlock`
  4. `m.snapshot.Store(newCfg)`
  5. 广播 `OnChange` 事件（router / scheduler / monitor 重建索引）

### 6.2 外部编辑

- `fsnotify` 监听 `config.yaml`：检测 `WRITE | RENAME` 事件
- debounce 200ms（避免编辑器多次保存）
- 加 `fileLock.RLock` 读、Validate、replace snapshot
- 校验失败：log + 控制台 SSE 错误通知，保留旧快照

### 6.3 原子性保证

- 临时文件 + rename 是 POSIX 与 Windows NTFS 都保证原子的 inode 替换。
- `flock` 在 Windows 走 `LockFileEx`（gofrs/flock 跨平台）。
- 不保留注释 → 直接 `yaml.Marshal(snapshot)`，简单可靠。

---

## 7. OAuth 刷新流程

```
chatgpt-oauth adapter.Chat(ctx, req):
    tok = ensureFresh(channelName):
        cfg = configMgr.Snapshot()
        oauth = cfg.findChannel(channelName).OAuth
        if oauth.expiresAt - now < 60s:
            // singleflight 防止并发刷新
            tok, err = sf.Do(channelName, doRefresh)
            if err: return err
            configMgr.UpdateOAuth(channelName, tok)  // 触发 6.1 的写流程
        return oauth.accessToken
    
    httpReq.Header.Set("Authorization", "Bearer " + tok)
    do upstream call
```

- `golang.org/x/sync/singleflight` 以渠道 name 为 key，保证同一渠道并发请求只触发一次刷新。
- 刷新调用 ChatGPT OAuth 的 token endpoint（具体 endpoint / 参数见研究项 R-1）。
- 刷新失败 → 渠道失败计数 + 熔断（避免反复打挂的 endpoint）。

---

## 8. 前端架构

### 8.1 技术栈与版本

- React 19
- Vite 7
- React Router 7
- Shadcn UI source-component pattern (`components.json` + `src/components/ui`)
- Tailwind CSS 4 + Radix primitives for dialog / sheet / tabs
- Axios + 自封装 client（自动注入 session cookie / 401 重定向到 /login）
- `react-i18next`（中/英）
- 主题：CSS variables + `prefers-color-scheme` + 手动覆盖（next-themes 思路自写）
- SSE：`EventSource` + 自封装 `useEventStream` hook

### 8.2 路由

```
/setup            # 首次启动向导（未完成时所有路由 redirect 至此）
/login            # 登录页
/                 # → /dashboard
/dashboard        # 仪表盘
/channels         # 渠道管理
/channels/:id     # 渠道详情（含拉取模型按钮）
/keys             # 下游 API Key
/logs             # 历史请求查询
/live             # 实时监控（SSE）
/settings         # 主题 / 语言 / 改密
```

### 8.3 关键组件

- `AppShell`：左侧导航 + 顶部主题/语言切换 + 用户菜单
- `ChannelForm`：含"拉取模型列表"按钮，调 `POST /api/admin/channels/probe-models`，返回模型 chip 列表供勾选
- `LiveLogTable`：虚拟列表 + SSE，最多保留 500 条
- `StatsCard`：渠道维度卡片
- `LineChart`（recharts）：24h / 7d / 30d 趋势

### 8.4 API 约定（admin REST）

> 渠道与 Key 都用 URL 安全编码的 `name` 作为路径参数。

| Method | Path | Body / Query | Resp |
|---|---|---|---|
| POST | `/api/admin/setup` | `{username, password}` | `{token: "sk-proxy-hub-..."}` |
| POST | `/api/admin/login` | `{username, password}` | sets session cookie |
| POST | `/api/admin/logout` | | |
| GET | `/api/admin/channels` | | `{openai-api: [...], chatgpt-oauth: [...]}` |
| POST | `/api/admin/channels/:type` | `ChannelDTO`（type ∈ openai-api / chatgpt-oauth） | 201 |
| PUT | `/api/admin/channels/:type/:name` | 完整 ChannelDTO（替换） | 200 |
| DELETE | `/api/admin/channels/:type/:name` | | 204 |
| POST | `/api/admin/channels/probe-models` | `{base-url, api-key, proxy-url?}` | `{models: [{id, ...}]}` |
| POST | `/api/admin/channels/:type/:name/health` | | `{ok, latency-ms, err?}` |
| GET | `/api/admin/keys` | | `[{name?, token-mask, disabled?, usage}]` |
| POST | `/api/admin/keys` | `{name?, notes?}` | `{token: "sk-proxy-hub-..."}`（仅此刻返回明文一次） |
| PATCH | `/api/admin/keys/:token-mask-or-name` | `{disabled?, notes?, name?}` | |
| DELETE | `/api/admin/keys/:token-mask-or-name` | | 204 |
| GET | `/api/admin/logs` | `?channel&status&from&to&page` | 分页 |
| GET | `/api/admin/logs/stream` | (SSE) | `event: log\ndata: {...}` |
| GET | `/api/admin/stats/channels` | `?window=24h\|7d\|30d` | 聚合 |
| GET | `/api/admin/stats/series` | `?channel&metric&window` | 时间序列 |

> Key 的路径定位策略：优先用 `name`；name 缺省时用 token-mask（如 `sk-proxy-hub-...AbCd`）；都重名时拒绝（YAML validate 阻断）。

---

## 9. 中间件链

```
[recover] → [requestID] → [cors? if enabled] → router split:
    /v1/*           → [downstreamAuth] → proxy
    /api/admin/*    → [adminAuth except /setup,/login] → admin
    /api/admin/.../stream → [adminAuth] → SSE (no compression)
    /              → embedded static (SPA fallback to index.html)
```

---

## 10. 关键 Trade-off

| 决策 | 选择 | 替代 | 选择原因 |
|---|---|---|---|
| 配置源 | YAML 唯一 | DB 主存 | 易迁移、PRD 已定 |
| 唯一标识 | `name`（无 id） | ULID | 参考 CLIProxyAPI；YAML 简洁、API 路径友好 |
| 写 YAML 风格 | omitempty + 不保留注释 | yaml.v3 Node API 保留 | 默认值/关闭项不落地，文件极简 |
| 凭证池 | 单渠道挂多 key | 1 渠道 1 key | 同账号叠加额度，参考 CLIProxyAPI |
| 别名表达 | 重复 model 项 `{name, alias}` | `name + aliases[]` | 与 CLIProxyAPI 一致，alias-pool 语义自然 |
| 请求日志 | 异步批量写 | 同步写 | 不阻塞主链路 |
| 聚合 | 内存累加 + 5s flush | 实时 group by | 大幅减少 SQLite 写压 |
| 实时推送 | SSE | WS | 单向够用，复用流式基建 |
| 调度 | priority + RR | 加权随机 / 最少失败 | 用户已选优先级；最直观 |
| OAuth 刷新 | on-demand + singleflight | 后台轮询 | 节省请求、避免并发风暴 |
| 模型解析 | 启动 / reload 时构建索引 map | 每次请求遍历 | O(1) lookup |
| 数据层抽象 | repository interface | 直接 sqlx | 为 Postgres 留口 |

---

## 11. 运维与回滚

- **退出**：`SIGINT` → 主 server 优雅关闭 → 等待 monitor flush → 关 SQLite
- **YAML 写失败**：临时文件 + rename 失败 → API 返回 5xx + 保留旧快照 + 错误推到控制台
- **SQLite 损坏**：启动时检测 → 备份原文件为 `.broken-<ts>` → 重建空库 + 控制台告警（不影响代理转发）
- **OAuth 刷新挂掉**：渠道熔断，控制台显示原因；用户重新填 refresh token 后热加载恢复

---

## 12. 安全要点

- API Key：constant-time 比较（`crypto/subtle.ConstantTimeCompare`）
- 管理员密码：argon2id，参数 `t=3, m=64MiB, p=2`
- session：HttpOnly + SameSite=Lax cookie，签名密钥每次启动随机（重启失效，单用户可接受）
- SSE：复用 session cookie 鉴权；CORS 默认关，浏览器 EventSource 同源
- 日志正文：bodyMode=always 时控制台明显警示"敏感"

---

## 13. 测试策略（concept）

- 单元：router 命中、scheduler RR / 熔断、config validate、yaml roundtrip、oauth 刷新 singleflight
- 集成：起内嵌 server + httptest 上游 mock → 走 /v1/chat/completions 全链路（流式 & 非流式 & 失败转移）
- 前端：vitest + RTL 组件级；E2E 暂缓

---

## 14. 未决待研究（标记到 implement 阶段处理）

- **R-1** ChatGPT OAuth 的实际刷新 endpoint / 参数（access_token / refresh_token grant 形态），需在 implement 阶段先做 spike
- **R-2** ChatGPT OAuth 调用的具体 path 与请求/响应 schema（codex 风格 `/backend-api/codex/responses` vs ChatGPT 风格 `/backend-api/conversation`），要决定如何转成 OpenAI chat/completions 兼容输出
- **R-3** tiktoken 对非 OpenAI 模型的 token 估算误差范围（DeepSeek 等），决定是否要按服务商分 encoder
