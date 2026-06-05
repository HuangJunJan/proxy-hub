# M1 —— 基础：执行计划（Implement）

> `06-04-m1-foundations` 的有序执行清单与校验。依据本子任务 `prd.md` + `design.md`。
> 语言约定：代码注释一律中文（见 `AGENTS.md`）。

## 前置

- 确认工具链：`go version`（本机 go1.25.5，使用 Go 1.25）。前端 M1 不需 Node（仅静态 `index.html`）。
- 分支：在 `main` 上先开特性分支，例如 `task/m1-foundations`（`task.py set-branch` 可记录）。

## 有序步骤

1. **初始化模块**
   - `go mod init github.com/huangjunjan/proxy-hub`（路径可调）；`go 1.25`。
   - 加依赖：gin、modernc.org/sqlite、fsnotify、yaml.v3。
2. **buildinfo**：`internal/buildinfo/version.go`，导出 `var Version = "dev"`（ldflags 注入）。
3. **config**：
   - `internal/config/config.go`：`Config` 结构 + `Load(path)`（默认 → yaml → 环境覆盖 → 校验）+
     派生路径方法（`DBPath()`、`AuthsDir()`）+ `EnsureAdminKey()`（为空则随机生成 32 字节 hex 并标记
     "首次生成需打印"）。
   - `internal/config/watcher.go`：`Watch(path, onReload)`，fsnotify + 200ms 防抖，仅热生效非密钥键。
   - `config.example.yaml`：注释完整的示例（中文注释）。
4. **store**：
   - `migrations/0001_init.sql`：建 `meta` 表。
   - `internal/store/migrate.go`：`Run(write)`（建 meta、读版本、VACUUM INTO 备份、事务内按序执行、
     更新版本）+ `rebuildTable(...)` helper。
   - `internal/store/db.go`：`Open(cfg)`（双句柄、pragmas、ping、调 migrate）、`Read()`、`Write()`、
     `HealthCheck(ctx)`、`Close()`。
5. **api**：
   - `internal/api/middleware/{recover,requestid,bodylimit,auth}.go`。
   - `internal/api/server.go`：`NewServer(cfg, store)` 装中间件 + `GET /healthz` + 静态壳回退。
6. **web 壳**：`web/dist/index.html`（占位）+ `web/embed.go`（`//go:embed dist`）。
7. **main**：`cmd/proxy-hub/main.go`：解析 `--config` flag → `config.Load` → `EnsureAdminKey`（必要时
   打印一次）→ `store.Open` → `api.NewServer` → 起 `http.Server` → 监听信号 → `Shutdown` + `store.Close`。
   预留 CLI 子命令骨架（如 `proxy-hub mcp sync` 占位，M4 实现）。
8. **打包**：`Dockerfile`（多阶段）、`docker-compose.yml`（单服务/卷/端口）、`.dockerignore`、
   `.goreleaser.yaml` 雏形。

## 校验命令（完成前全绿）

```bash
gofmt -l .            # 无输出
go vet ./...
CGO_ENABLED=0 go build -o ./bin/proxy-hub ./cmd/proxy-hub
go test ./...         # M1 至少含 config.Load 与 migrate.Run 的单测
./bin/proxy-hub --config ./config.example.yaml &   # 本地起服务
curl -fsS http://127.0.0.1:7777/healthz            # 期望 200 + {"status":"ok"}
# 容器：
docker build -t proxy-hub:dev .
docker run --rm -v "$PWD/data:/data" -p 7777:7777 proxy-hub:dev   # 无其它服务即可起
```

## 单元测试要点

- `config.Load`：默认值、yaml 覆盖、环境变量覆盖、缺失文件容错。
- `migrate.Run`：全新库建表 + 版本=1；重复运行 no-op；模拟坏迁移 → 事务回滚、版本不变、存在 `.bak`。
- `store.HealthCheck`：正常返回 nil；关闭后返回错误。

## 评审门 / 回滚

- 评审门：上述校验全绿 + `trellis-check` 干净 + `docker run` 演示可复现。
- 回滚：本里程碑在特性分支，可整体回退；DB 仅 `meta` 表，无数据迁移风险。

## 完成后

- `trellis-check` → 修复 → 提交（Phase 3.4）。
- 更新 `.trellis/spec/backend/`（Go+SQLite 目录结构、错误处理、日志约定）——这些是 M1 确立的真实约定，
  正好回填 `00-bootstrap-guidelines` 的 backend 规范。
