# Proxy Hub - Implementation Plan (v1)

> 执行顺序按"依赖反向构建"：底层 → 上层 → 端到端。每个 Phase 内的步骤必须按序，跨 Phase 也按序。每步附**验证命令**和**完成标志**。

---

## 总览

```
P0 Bootstrap         → 项目骨架 + 依赖选型
P1 Config Layer      → YAML 加载/热加载/原子写
P2 Storage Layer     → SQLite + repository
P3 Auth + Setup      → 鉴权 + 首次启动向导
P4 Upstream Adapters → openai-api / chatgpt-oauth
P5 Router + Scheduler→ alias 索引 + 优先级 RR + 熔断
P6 Proxy /v1/*       → chat/completions + models
P7 Monitor           → 日志写库 + SSE + 聚合 + 清理
P8 Admin API         → channels/keys/logs/stats CRUD
P9 Frontend          → React UI + i18n + theme
P10 Embed + Build    → go:embed + Windows exe
P11 Acceptance       → 跑完 AC-1..AC-18
```

> 单子任务，全部由一个开发者按序推进（PRD 已选不拆）。

---

## P0 Bootstrap（半天）

### Steps

1. **建立目录结构**（见 design.md §1）。
2. **`go mod init proxy-hub`**，定 Go 1.22+（用 `iter.Seq`）。
3. **选型依赖**（写入 go.mod，不引入运行时但占位 import 以确认可用）：
   - HTTP：`github.com/gin-gonic/gin`
   - YAML：`gopkg.in/yaml.v3`
   - SQLite：`modernc.org/sqlite`（纯 Go，免 CGO 简化 Windows 交叉编译）
   - 文件锁：`github.com/gofrs/flock`
   - 文件监控：`github.com/fsnotify/fsnotify`
   - argon2：`golang.org/x/crypto/argon2`
   - singleflight：`golang.org/x/sync/singleflight`
   - tiktoken：`github.com/pkoukk/tiktoken-go`
   - 日志：`log/slog`（标准库 JSON handler）
4. **`cmd/proxy-hub/main.go`** 最小骨架：解析 `--config` flag，起 gin server，监听 `:8787`，挂一个 `/healthz` 返回 200。
5. **前端骨架**：`web/` 下 `pnpm create vite` (react-ts)，装 `tailwindcss`、`shadcn-ui` init、`react-router-dom`、`react-i18next`、`axios`、`recharts`。
6. **`config.example.yaml`** 落地（含所有字段注释）。

### Validation

```bash
go run ./cmd/proxy-hub --config ./config.example.yaml &
curl -s http://localhost:8787/healthz   # → "ok"
cd web && pnpm dev                       # 打开浏览器看到默认 Vite 页
```

### Done 标志

- `go build ./...` 通过
- `pnpm build` 通过
- README 列出依赖与启动步骤

---

## P1 Config Layer（1-1.5 天）

### Steps

1. **`internal/config/types.go`** 定义全部 YAML struct（design.md §2 完整字段），所有可选字段标 `yaml:",omitempty"`、bool 用 `bool` 即可（默认 false 走 omitempty）。
2. **Marshal/Unmarshal 测试**：fixture 完整 YAML → struct → 重新 Marshal → 比对，保证 omitempty 真的生效（默认值不写回）。
3. **`Validate(cfg) error`**：
   - 每 section 内 `name` 唯一
   - `api-keys[].token` 唯一 + 字符集 + 长度 ≥ 32
   - 单渠道内 `(models[].name, alias)` 唯一
   - `base-url` URL 合法、`oauth.expires-at` 解析
   - 至少 1 个启用渠道（warning，不阻塞）
4. **`Manager`** 实现（design.md §6）：
   - `atomic.Pointer[Config]` 持快照
   - `Load(path)`：读 → validate → 存快照
   - `Save(mutator func(*Config))`：clone → mutate → validate → flock → tmp+rename → atomic.Store → 广播 OnChange
   - `Watch()`：fsnotify + debounce 200ms → reload
5. **OnChange 订阅器**：简单 `[]func(*Config)`，注册回调。
6. **首次启动逻辑**：检测 `path` 不存在 → 进入"setup mode"标志（不写 YAML，等 P3 setup endpoint 写入）；存在但缺 `admin` 或 `api-keys` 也进 setup mode。
7. **明文密码自动哈希**：load 后若 `admin.password-hash` 不是 argon2id 格式 → 哈希并 `Save`。

### Validation

```bash
go test ./internal/config/...
# 单元覆盖：
#   - omitempty roundtrip
#   - validate 全部分支
#   - Save 原子性（手动 kill -9 中途，文件应为旧版完整态）
#   - fsnotify 编辑触发 reload
```

### Done 标志

- `go test ./internal/config -race -count=1` 全绿
- 手测：编辑 config.yaml 改 priority，进程日志显示 reload，新 priority 生效

---

## P2 Storage Layer（半天）

### Steps

1. **`internal/store/sqlite.go`** 打开 SQLite，`PRAGMA journal_mode=WAL; synchronous=NORMAL; busy_timeout=5000`。
2. **schema 迁移**：`internal/store/migrations/0001_init.sql`（design.md §3 所有 DDL），迁移 runner 用 `meta` 表记录 `schema_version`。
3. **Repository 接口**：
   ```go
   type RequestLogRepo interface {
       BatchInsert(ctx, []LogEntry) error
       Query(ctx, QueryFilter) ([]LogEntry, error)
       DeleteBefore(ctx, ts int64) (int64, error)
   }
   type StatsRepo interface {
       UpsertHourly(ctx, []HourlyDelta) error
       QueryChannelSummary(ctx, window) ([]ChannelSummary, error)
       QuerySeries(ctx, channelName, metric, window) ([]Point, error)
   }
   ```
4. **modernc.org/sqlite 实现**。
5. **失败恢复**：启动时 `PRAGMA integrity_check`，失败则 rename `db.broken-<ts>` + 重建空库 + 日志告警。

### Validation

```bash
go test ./internal/store/... -race
# 覆盖：迁移幂等、批量插入、查询过滤、清理 DeleteBefore
```

---

## P3 Auth + Setup（半天）

### Steps

1. **`internal/auth/password.go`**：argon2id `t=3 m=64MiB p=2`，hash + verify。
2. **`internal/auth/bearer.go`**：从 `api-keys[]` 构建 token→entry 索引（reload 时重建）；`constant-time` 比较。
3. **`internal/auth/session.go`**：HMAC-SHA256 签名，secret 启动时 `crypto/rand` 生成 32 字节；cookie `HttpOnly`、`SameSite=Lax`、`Path=/`、过期 30 天。
4. **`internal/admin/setup.go`**：
   - `GET /api/admin/setup/status` → `{needed: bool}`
   - `POST /api/admin/setup` body `{username, password}` → 生成 token、Save YAML（写 admin + first api-key）、返 token 明文
5. **中间件**：
   - `requireSession` 校验 cookie；setup 模式下放行 `/api/admin/setup/*`、`/setup` 静态资源、`/_next/*` 之类静态，其他全部 302 到 `/setup`
   - 正常模式：`/api/admin/*` 要求 session（除 login/setup-status）；`/v1/*` 要求 bearer key

### Validation

```bash
# 启动一个空白 config 目录
rm -rf ./testenv && mkdir ./testenv
go run ./cmd/proxy-hub --config ./testenv/config.yaml &
curl -s http://localhost:8787/api/admin/setup/status      # → {"needed":true}
curl -sX POST http://localhost:8787/api/admin/setup \
    -H 'content-type: application/json' \
    -d '{"username":"admin","password":"hunter2"}'        # → {"token":"sk-proxy-hub-..."}
cat ./testenv/config.yaml                                  # 应含 admin + 1 个 api-key
```

---

## P4 Upstream Adapters（1.5 天，含 R-1/R-2 spike）

### Steps

1. **`internal/upstream/types.go`** 接口：
   ```go
   type Adapter interface {
       Chat(ctx, ChatReq) (ChatResp | StreamResp, error)
       Models(ctx) ([]string, error)         // 仅 openai-api 实现
       HealthCheck(ctx) error
   }
   type ChatReq struct {
       UpstreamModelName string
       OriginalBody      []byte    // 透传，需替换其中 "model" 字段
       Stream            bool
       APIKey            string    // openai-api 用；oauth 由 adapter 内部 ensureFresh
       BaseURL           string
       Headers           map[string]string
       Timeout           time.Duration
   }
   ```
2. **`internal/upstream/openai/adapter.go`**：
   - 替换 body.model → `UpstreamModelName`（用 `sjson`/或自定义 JSON 浅替换）
   - `POST {BaseURL}/v1/chat/completions`，stream/非 stream 分支
   - 返回 `io.ReadCloser` for stream，proxy 层负责转发
3. **R-1 / R-2 Spike**：参考 CLIProxyAPI `internal/auth/codex/` 与 `internal/translator/codex/`，确定：
   - ChatGPT OAuth token refresh endpoint + 参数
   - Codex 上游 endpoint + request/response 协议
   - 转换成 OpenAI chat/completions 兼容输出的 translator 写法
   - 落地为 `internal/upstream/chatgpt/README.md` 备忘
4. **`internal/upstream/chatgpt/oauth.go`**：refresh + singleflight。
5. **`internal/upstream/chatgpt/adapter.go`**：实现 Chat（含协议转换）+ HealthCheck（用 list models 等轻请求）。
6. **`internal/upstream/probe.go`**：通用"探活/拉取模型"工具，用于控制台 probe-models 按钮（仅 openai-api 调用上游 /v1/models）。

### Validation

```bash
# 用 httptest 起 mock 上游
go test ./internal/upstream/openai/... -race
go test ./internal/upstream/chatgpt/... -race
# spike 产物：internal/upstream/chatgpt/README.md 落定 endpoint
```

### Risk

- **R-2 协议转换可能很重**。若发现 chatgpt OAuth 协议改动频繁/不稳定，**可以选择 v1 临时降级为只做 openai-api，把 chatgpt-oauth 推到 v1.1**。改动：PRD AC-12 / FR-2.5 标 deferred，更新 design 第 1 章。**这个 fallback 在 P4 末尾做 go/no-go review**。

---

## P5 Router + Scheduler（半天）

### Steps

1. **`internal/router/index.go`**：监听 config OnChange，重建 `aliasToHits` map（design.md §5.1）。alias 小写规范化。
2. **`internal/scheduler/scheduler.go`**：
   - `ChannelRuntime` 同步加 / 删（reload 时与新 config diff，保留熔断状态）
   - `Pick(hits) iter.Seq[HitEntry]`：filterAlive → groupByPriority → bucket-RR
   - 熔断状态机（closed/open/half-open），thread-safe
3. **`internal/scheduler/keypool.go`**：单渠道内的 `api-key-entries` round-robin 游标，封装 `NextKey()`。

### Validation

```bash
go test ./internal/router/... ./internal/scheduler/... -race
# 关键用例：
# - 同 alias 多渠道按 priority 排序
# - 同优先级 RR
# - 熔断打开后跳过；half-open 试探
# - reload 后熔断状态保留（同名渠道）
```

---

## P6 Proxy /v1/* Handler（1 天）

### Steps

1. **`internal/proxy/handler.go`**：
   - 解析 body 拿 `model`、`stream`
   - `router.Resolve(model)` → hits
   - `for hit := range scheduler.Pick(hits)` 重试循环
   - openai-api：从 `hit.Channel.KeyPool.NextKey()` 拿凭证 → `adapter.Chat`
   - chatgpt-oauth：`adapter.Chat`（内部 ensureFresh）
   - 成功：流式逐 chunk `c.Writer.Write` + `Flush`；非流式 `c.JSON`
2. **Token 统计**：
   - 上游返 `usage` 时直接用
   - 否则 tiktoken 估算 prompt（解析 messages）+ 累计 completion bytes 折算
3. **错误归一**：实现 design.md §4.2 的 mapping。
4. **`GET /v1/models`**：从 router 的 alias 索引返 union。
5. **`monitor.Submit(LogEntry)`** defer 调用。

### Validation

```bash
# 跑 httptest mock 上游
go test ./internal/proxy/... -race
# E2E：
go run ./cmd/proxy-hub --config ./testdata/two-channels.yaml &
curl -s http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer sk-proxy-hub-..." \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}'
# stream:
curl -N -H "..." -d '{...,"stream":true}'
```

---

## P7 Monitor（半天）

### Steps

1. **`internal/monitor/buffer.go`**：`chan LogEntry`（buffer 1024），溢出时 drop + 计数告警。
2. **`internal/monitor/writer.go`**：goroutine 消费 channel，batch 50 / 100ms 触发 → `RequestLogRepo.BatchInsert`。
3. **`internal/monitor/aggregator.go`**：内存累加器（map[channelName]HourlyAccum），每 5 秒 flush 到 `channel_stats_hourly`（UPSERT）。
4. **`internal/monitor/sse.go`**：fan-out hub，每个订阅者一个 `chan LogEntry`（小 buffer，慢消费者直接掉）。
5. **`internal/monitor/cleanup.go`**：每小时 `RequestLogRepo.DeleteBefore(now - retention)`。
6. **优雅关闭**：`Shutdown(ctx)` 排空 channel + 最后一次 flush。

### Validation

```bash
go test ./internal/monitor/... -race
# 模拟 1000 条/秒持续 10 秒，检查批写 + 聚合无丢失
```

---

## P8 Admin API（1 天）

### Steps

按 design.md §8.4 表实现，每个 endpoint：
1. handler 函数 + 输入校验 DTO
2. 调 `config.Manager.Save` 改 YAML（channels/keys CRUD）
3. 调 store / router / scheduler 读运行时数据（logs/stats/health）

**关键 endpoint 细节**：
- `POST /api/admin/channels/probe-models`：传入 `{base-url, api-key, proxy-url?}`，调用 `upstream.Probe.ListModels`，返 string 数组。
- `POST /api/admin/channels/:type/:name/health`：调对应 adapter `HealthCheck`，返延迟 / 错误。
- `GET /api/admin/logs/stream`：SSE，复用 monitor.sse hub。

### Validation

```bash
go test ./internal/admin/... -race
# E2E 走一遍 curl：建渠道、改优先级、拉模型、删 key
```

---

## P9 Frontend（2-3 天）

### Steps

1. **基础设施**：
   - Vite + Tailwind + Shadcn 装好基础组件（button、input、table、dialog、sheet、tabs、toast、card、badge）
   - `lib/api.ts`：axios + 401 重定向 + i18n error toast
   - `lib/i18n.ts`：zh/en，`react-i18next` 初始化，语言切换持久化 localStorage
   - `lib/theme.tsx`：light/dark/system，CSS variables，持久化 localStorage，挂 `<html class="dark">`
   - `lib/sse.ts`：`useEventStream(path)` hook
   - `lib/auth.tsx`：`<RequireSetup>` + `<RequireAuth>` 路由守卫

2. **页面**（按依赖顺序）：
   - `/setup`：单页表单（username + password + confirm），调 `POST /api/admin/setup`，成功后 dialog 显示首个 token（明文一次）让用户复制
   - `/login`
   - `<AppShell>`：侧栏 + 顶栏（主题/语言/退出）
   - `/dashboard`：渠道汇总卡片 + 24h 趋势图（recharts）
   - `/channels`：列表 + 新建/编辑 Sheet，含"拉取模型列表"按钮（弹出可勾选清单）、alias 编辑、健康检查按钮
   - `/keys`：列表 + 新建 dialog（创建后明文 token 仅显示一次）
   - `/logs`：表格 + 过滤（渠道/状态/时间范围）+ 分页
   - `/live`：虚拟列表 + SSE，最多保留 500 条
   - `/settings`：主题 / 语言 / 改密 / 改 YAML 路径（只读显示）

3. **i18n key 覆盖**：每个页面新增 key 时同时填 zh / en；用 `i18next-parser` 或 grep 校验无遗漏。

### Validation

- 手测每个页面在 light/dark + zh/en 下显示正常
- 手测 setup → login → 建渠道 → 发请求 → 监控页看到记录

---

## P10 Embed + Build（半天）

### Steps

1. **`internal/web/embed.go`**：`//go:embed all:dist`，serve SPA + fallback 到 index.html。
2. **build 脚本** `scripts/build.ps1`：
   ```
   cd web && pnpm build && cd ..
   cp -r web/dist internal/web/dist
   GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/proxy-hub.exe ./cmd/proxy-hub
   ```
3. **自动开浏览器**：main.go 启动后 `exec.Command("rundll32", "url.dll,FileProtocolHandler", url)`，CLI flag `--no-browser` 关掉。
4. **图标 + 版本资源**：用 `goversioninfo` 给 exe 嵌入 icon + 版本信息（可选，时间紧可略）。

### Validation

- 双击 `dist/proxy-hub.exe`：
  - 浏览器自动开到 `http://localhost:8787`
  - 看到 setup 向导
  - 全流程跑通

---

## P11 Acceptance（半天）

逐条对照 PRD AC-1 ~ AC-18 跑一遍，记录在 `.trellis/tasks/05-25-proxy-hub/acceptance.md`。

---

## 风险点与回滚

| 风险 | 触发条件 | 回滚方案 |
|---|---|---|
| ChatGPT OAuth 协议太复杂 | P4 spike 发现 > 2 天工作量 | v1 删掉 chatgpt-oauth；标 v1.1；PRD/Design 移除相关 FR/AC |
| modernc.org/sqlite 性能不够 | P7 压测 < 100 写/秒 | 切 `mattn/go-sqlite3`（CGO，需 gcc 交叉编译；用 Zig CC） |
| YAML 热加载竞态 | P1 race detector 告警 | 退回"重启才生效"模式，控制台改完显示"需重启" |
| Shadcn UI 与 React 19 不兼容 | P9 实际遇到 peer dep 报错 | 降到 React 18.3 |
| tiktoken-go 体积过大 | embed 后 exe > 50MB | 仅在 prompt token 估算时按需 lazy-init；或改用 `dlclark/regexp2` 手写简易切词 |

---

## 全局约束

- **Race detector** 全程 `go test -race`
- **每个 Phase 结束** commit + 跑 lint (`golangci-lint run`)
- **不写 TODO**：发现要做的事直接做或追加到本文件
- **不留死代码**：未启用的实验性逻辑不入仓
- **Windows path**：所有 path 操作用 `filepath`，不用 `path`
- **slog** 全程 JSON handler，关键路径打 request_id + channel_name

---

## Pre-Start Checklist（task.py start 前确认）

- [ ] PRD review pass（用户已确认 v1 范围）
- [ ] Design review pass（用户已确认 schema / 模块切分）
- [ ] Implement review pass（用户已确认本文件）
- [ ] 仓库为空（除 `.trellis/` 与 `.claude/`）→ 可以直接开始 P0
- [ ] Go 1.22+ 已装；pnpm 已装；MinGW / Zig CC（如果走 mattn/go-sqlite3 fallback）

---

## 不在 v1 但已经在 design 里留口的事

- Postgres 实现（`store.RequestLogRepo` 第二实现）
- 更多 upstream 协议（Anthropic Messages / Gemini 原生）
- 多用户 / RBAC（auth 包字段已留 `userId`）
- Prometheus `/metrics`
- 限流 middleware

——这些不在本 implement.md 范围，但在写代码时注意接口边界不要把自己堵死。
