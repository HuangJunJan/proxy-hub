# 目录结构（Directory Structure）

> proxy-hub 后端代码组织方式。约定由 M1（基础里程碑）确立，后续里程碑沿用。
> 语言：本规范用中文撰写，代码标识符、包名、路径、技术名保留英文（与项目 `design.md` 一致）。

---

## 概览

- 单 Go module：`github.com/huangjunjan/proxy-hub`，`go 1.25`。
- 标准 `cmd/` + `internal/` 布局。业务代码全部在 `internal/`（不对外暴露为库）。
- 入口 `cmd/proxy-hub/main.go` 为**瘦入口**：仅做装配（加载配置 → 打开 DB → 构建 HTTP 服务 → 优雅停机），不含业务逻辑。
- 前端构建产物经 `go:embed` 内嵌（`web/`），最终为单静态二进制（`CGO_ENABLED=0`）。

---

## 目录布局

M1 已落地（标 ✅）；其余为父 `design.md` §2.2 规划、后续里程碑填充：

```
cmd/proxy-hub/main.go              ✅ 瘦入口：装配 + 信号 + 优雅停机
internal/
  config/{config.go, watcher.go}   ✅ 配置模型 + 加载/校验 + fsnotify 热重载
  store/                           ✅ SQLite 生命周期
    db.go                          ✅ 双句柄（读池 + 单写）+ HealthCheck
    migrate.go                     ✅ 版本化迁移 + 备份 + rebuildTable
    migrations/*.sql               ✅ go:embed（注意：紧邻 migrate.go，相对路径）
  api/                             ✅ Gin 服务器
    server.go                      ✅ NewServer + /healthz + 静态壳回退
    middleware/{recover,requestid,bodylimit,auth}.go  ✅
  buildinfo/version.go             ✅ var Version（ldflags 注入）
  relay/ adaptor/ channel/ selector/   (M2)
  stats/ health/                       (M3)
  mcp/ fileio/                          (M4)
web/{embed.go, dist/}              ✅ go:embed dist（M1 为占位 index.html；M3 换 React 产物）
config.example.yaml               ✅
Dockerfile · docker-compose.yml · .goreleaser.yaml · .dockerignore  ✅
```

---

## 模块组织

- **一个能力一个包**（relay/channel/stats/mcp 等），包内再分 `dao`/`service`/`runtime` 等子职责。
- 包级 doc 注释写在该包"主文件"顶部（如 `config.go`、`store/db.go`、`api/server.go`、`middleware/recover.go`），用中文说明包职责。
- 依赖方向：`api` → 各能力包 → `store`/`config`；`store` 依赖 `config`。**禁止反向依赖**（如 `store` 不得 import `api`）。
- 跨能力共享的底层（`fileio`、`buildinfo`、`config`、`store`）保持无业务依赖。

---

## 命名约定

- 包名：小写单词，无下划线（`buildinfo` 而非 `build_info`）。
- 文件名：小写 + 下划线（`server.go`、`request_logging.go`）；测试文件 `_test.go`。
- 构造器：`New<Type>` 返回 `*Type`（如 `api.NewServer`）；包级生命周期入口用动词（`store.Open`、`config.Load`、`config.Watch`、`migrate.Run`）。
- 导出标识符需有中文 doc 注释（见 `quality-guidelines.md`）。

---

## 示例

- 瘦入口范式：`cmd/proxy-hub/main.go` 的 `run()`（返回 error，`main` 统一处理退出码）。
- 包生命周期范式：`internal/store/db.go` 的 `Open`/`Close`、`internal/config/config.go` 的 `Load`。
