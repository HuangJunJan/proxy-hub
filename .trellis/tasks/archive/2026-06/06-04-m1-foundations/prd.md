# M1 —— 基础（Foundations）

> `06-04-proxy-hub-mvp` 的子任务。共享的架构/数据模型/技术栈见父任务 `design.md`。这是阻塞所有
> 其它里程碑的基础里程碑。
>
> 语言约定：本项目所有规划文档与代码注释一律使用中文（见 `AGENTS.md`）。

## 目标

立起可运行的骨架：一个 Go 二进制，打开内嵌 SQLite、加载配置、用 Gin 提供带 `/healthz` 的 HTTP
服务、内嵌（空的）管理后台 SPA、并以一个小容器交付——让后续每个里程碑都有插入点。

## 范围

**范围内**
- `go.mod`（Go 1.25+）、`cmd/proxy-hub/main.go` 瘦入口（载配置 → 开 DB → 初始化管理器 → 起
  Gin + 后台循环 + CLI 子命令雏形）。
- `internal/store`：经纯 Go `modernc.org/sqlite` 打开 SQLite，启用 WAL、`busy_timeout`、
  `synchronous=NORMAL`、`foreign_keys=ON`；**读连接池 + 单写协程**；版本化迁移器
  （`meta.schema_version`），含**预迁移 `.db` 备份**与**表重建迁移模式**（不只是
  `IF NOT EXISTS`/`ALTER`）。
- `internal/config`：`config.yaml` 模型、每个键的环境变量覆盖、对非密钥键的 `fsnotify` 热重载。
- `internal/api`：Gin 服务器、中间件（auth 雏形、request-id、recover、body-limit）、`/healthz`
  （进程 + DB ping）、优雅停机（flush 写者）。
- 打包：多阶段 Dockerfile → `alpine:3.x`（+ ca-certificates、tzdata），单服务 / 单 `/data` 卷 /
  单对外端口，容器 `HEALTHCHECK`；`go:embed` 一个空 `web/dist` 壳；`.goreleaser.yaml` 雏形；
  `config.example.yaml`。

**范围外**
- 任何渠道/中转/统计/MCP 逻辑（后续里程碑）。此处 auth 中间件是雏形（真实 key 鉴权在 M2 落地）。
  暂无真实 SPA 页面。

## 需求（来自父 `prd.md`）

FR-C3-1、FR-C3-2、FR-C3-3（部署）；FR-C2-* 与所有 DAO 依赖的 store/迁移底座；NFR-1、NFR-4。

## 依赖

无。**阻塞 M2、M3、M4、M5。**

## 交付清单

- [ ] `go.mod` + 模块布局（按父 `design.md` §2.2）。
- [ ] `internal/store/db.go`（打开 + pragmas + 读连接池 + 写协程）。
- [ ] `internal/store/migrate.go`（版本化迁移器 + 备份 + 表重建模式 + `meta`）。
- [ ] `internal/config/{config.go,watcher.go}`（+ `config.example.yaml`）。
- [ ] `internal/api/server.go` + 中间件 + `/healthz` + 优雅停机。
- [ ] `go:embed web/dist` 壳；Dockerfile；`docker-compose.yml`（单服务/卷/端口）；
      `.goreleaser.yaml` 雏形。

## 验收标准

- [ ] `CGO_ENABLED=0 go build ./...` 成功；`gofmt`/`go vet` 干净。
- [ ] `docker run -v ./data:/data -p 7777:7777 proxy-hub` 在无其它服务下启动。
- [ ] `/healthz` 返回 200（进程 + DB ping）；容器 HEALTHCHECK 通过。
- [ ] 首次运行创建 `data/proxy-hub.db`（WAL）并应用迁移；二次运行为 no-op；模拟一次坏迁移能从
      预迁移 `.db` 备份恢复。
- [ ] 编辑 `config.yaml` 热重载非密钥键（有日志）；环境变量覆盖文件键。

## 备注

本子任务的 `design.md`（store/迁移/并发细节）与 `implement.md`（有序步骤 + 校验）在子任务启动时
撰写，从父 `design.md` 推导。
