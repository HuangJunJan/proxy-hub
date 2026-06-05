# M1 —— 基础：技术设计（Design）

> `06-04-m1-foundations` 的子任务设计。继承父 `design.md`，此处只细化 M1 范围内的技术细节，
> 细到可直接编码。语言约定：规划文档与代码注释一律中文（见 `AGENTS.md`）。

## 1. 模块与依赖

- 模块路径（提案，可调）：`github.com/huangjunjan/proxy-hub`；`go 1.25`（本机 go1.25.5；因不移植 CLIProxyAPI 代码，无需 1.26）。
- 依赖（保持最小）：
  - `github.com/gin-gonic/gin` —— HTTP 框架。
  - `modernc.org/sqlite` —— 纯 Go SQLite 驱动（`CGO_ENABLED=0`）。注册的驱动名为 `sqlite`。
  - `github.com/fsnotify/fsnotify` —— 配置热重载。
  - `gopkg.in/yaml.v3` —— 解析 `config.yaml`。
  - 标准库 `log/slog`（结构化日志）、`database/sql`、`embed`。
- M1 不引入：sqlc（M2 起按 OQ-2 决定）、shopspring/decimal、pelletier/go-toml（各自里程碑再引）。

## 2. 目录（本里程碑落地的部分）

```
cmd/proxy-hub/main.go
internal/
  config/{config.go, watcher.go}
  store/{db.go, migrate.go}
  api/{server.go, middleware/{requestid.go, recover.go, bodylimit.go, auth.go}}
  buildinfo/version.go            # 版本号（ldflags 注入）
web/embed.go + web/dist/index.html  # go:embed 空壳
migrations/0001_init.sql          # 仅建 meta 表，确立迁移框架
config.example.yaml
Dockerfile
docker-compose.yml
.goreleaser.yaml
.dockerignore
```

## 3. Store —— SQLite 并发模型（核心）

SQLite 的并发模型是"多读单写"。M1 用**两个 `*sql.DB` 句柄指向同一文件**来落地父设计的
"读连接池 + 单写者"，无需手写 writer 协程（批量写是 M3 采集器的关注点，届时在 `write` 句柄上加批量）：

- `read *sql.DB`：默认连接池（多读）。
- `write *sql.DB`：`SetMaxOpenConns(1)` —— 所有 `INSERT/UPDATE/DELETE/DDL` 经此单连接串行化，
  天然契合 SQLite 单写者模型，避免 `database is locked`。

DSN（modernc 用 `_pragma=` 查询参数）：
```
file:<dataDir>/proxy-hub.db?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)
```

`store.Open(cfg) (*Store, error)`：
1. 确保 `dataDir` 存在（`0700`）。
2. 打开 `read` 与 `write` 两个句柄（同 DSN）；`write.SetMaxOpenConns(1)`；`read.SetMaxOpenConns(N)`
   （默认 `max(4, GOMAXPROCS)`）。
3. 各 `PingContext` 校验。
4. 调 `migrate.Run(write)` 应用迁移。
5. 返回 `*Store{read, write}`，暴露 `Read() *sql.DB`、`Write() *sql.DB`、`HealthCheck(ctx)`、`Close()`。

> WAL 模式建议 `synchronous=NORMAL`（WAL 下安全且更快）；`busy_timeout=30000` 兜底偶发锁等待。

## 4. Migrate —— 版本化迁移 + 备份 + 表重建

- 迁移文件以 `go:embed migrations/*.sql` 内嵌；按文件名数字前缀（`0001_`、`0002_`…）排序。
- `meta(key TEXT PRIMARY KEY, value TEXT)` 存 `schema_version`（字符串整数）。
- `Run(write *sql.DB)`：
  1. 建 `meta`（`CREATE TABLE IF NOT EXISTS`）；读 `schema_version`（缺省 0）。
  2. 若有待应用迁移（version > 当前），**先做预迁移备份**：用 SQLite `VACUUM INTO '<db>.bak-<schema_version>'`
     生成一致性快照（modernc 支持；优于直接拷文件，避免 WAL 半态）。
  3. 在单个事务内依次执行每个待应用迁移的 SQL；成功后把 `schema_version` 更新为该版本号；事务提交。
     任一失败 → 回滚事务并返回错误（调用方据此可恢复备份）。
- **表重建模式**（供后续里程碑改表用，M1 先实现 helper）：`CREATE 新表 → INSERT ... SELECT 拷数据 →
  DROP 旧表 → ALTER 新表 RENAME`，整体在迁移事务内，规避 SQLite `ALTER` 局限。
- M1 的 `0001_init.sql`：仅建 `meta` 表（其余业务表由各里程碑的迁移新增）。

## 5. Config —— YAML + 环境覆盖 + 热重载

`Config` 结构（字段示例，YAML 标签小写下划线，环境变量前缀 `PROXY_HUB_`）：
```
server:   { addr: ":7777", read_timeout, write_timeout }
data_dir: "./data"            # 派生 db_path = data_dir/proxy-hub.db, auths_dir = data_dir/auths
admin_key: ""                 # 为空则首次运行生成并打印一次（M1 仅生成+打印，鉴权在 M2 接管）
log:      { level: "info", format: "text" }
retention_days: 30
```
- `config.Load(path)`：读默认值 → 若文件存在则 yaml 覆盖 → 环境变量覆盖（`PROXY_HUB_SERVER_ADDR`
  等，下划线映射嵌套键）→ 校验（addr 非空、data_dir 可创建）。
- `watcher.Watch(path, onReload func(*Config))`：fsnotify 监听文件；防抖（200ms）后重新 `Load`，
  对**非密钥**键热生效（server 超时、log level、retention）；变更记录到日志。密钥类（admin_key）
  不热重载（避免半态），变更需重启。

## 6. API —— Gin 服务器 + 中间件 + healthz + 优雅停机

- `api.NewServer(cfg, store)`：构建 `*gin.Engine`，`gin.New()` + 自定义中间件（不用 `gin.Default()`，
  避免默认 logger 噪音）。
- 中间件链（顺序）：`recover`（panic → 500 + slog 记录）→ `requestid`（读 `X-Request-Id` 或生成，
  注入 ctx 与响应头）→ `bodylimit`（默认 32MB，可配）→ `auth`（M1 为雏形：仅放行 `/healthz`，
  其余 `/admin/*`、`/v0/*` 暂返回 501 占位或放行——见下）。
- 路由：`GET /healthz` → 调 `store.HealthCheck`（DB ping）+ 进程存活 → 200 `{"status":"ok",
  "version":..,"db":"ok"}`；DB 异常 → 503。
- `auth` 雏形：M1 不做真实鉴权（M2 接管）。实现为可插拔中间件：若 `cfg.AdminKey` 已设，对
  `/admin/*`、`/v0/*` 校验 `Authorization: Bearer <key>`；M1 这些路由尚未注册，故等价于仅 `/healthz`
  可用。保留中间件骨架供 M2 扩展。
- 优雅停机：`main` 监听 SIGINT/SIGTERM → `http.Server.Shutdown(ctx, 10s)` → `store.Close()`
  （后续里程碑在此 hook flush 统计缓冲）。

## 7. 前端壳 + 静态服务

- `web/dist/index.html` 一个最小占位页；`web/embed.go`：`//go:embed dist` 暴露 `embed.FS`。
- 服务：`router.StaticFS` / `NoRoute` 回退到 `index.html`（SPA history 模式），API 路由优先匹配。
- M1 不引入 Node 构建；`web/dist` 直接放一个静态 `index.html`（M3 才接入真正的 React 构建，
  届时 Dockerfile 增加前端构建阶段）。

## 8. 打包

- 多阶段 `Dockerfile`：
  - stage build：`golang:1.25-alpine`，`CGO_ENABLED=0 go build -ldflags="-s -w -X
    .../buildinfo.Version=$VERSION" -o /out/proxy-hub ./cmd/proxy-hub`。
  - stage final：`alpine:3.x` + `ca-certificates tzdata`，`COPY` 二进制，`USER nonroot`（可选），
    `EXPOSE 7777`，`VOLUME /data`，`HEALTHCHECK CMD wget -qO- http://127.0.0.1:7777/healthz || exit 1`，
    `ENTRYPOINT ["/proxy-hub"]`。
- `docker-compose.yml`：单服务 `proxy-hub`，`ports: 7777:7777`，`volumes: ./data:/data`，
  无 `depends_on`，无其它服务。
- `.goreleaser.yaml`：`builds`（linux/darwin/windows × amd64/arm64，`CGO_ENABLED=0`，注入 Version），
  `archives`，`checksum`。M1 先放可用雏形。

## 9. 验收映射

§3-§4 ⇒ FR-C3-1/FR-C3-2；§5 ⇒ FR-C3-1（配置）；§6 ⇒ /healthz；§8 ⇒ FR-C3-3；密钥 §5 ⇒ FR-C3-4。
对应 `prd.md` 验收清单逐条可演示。
