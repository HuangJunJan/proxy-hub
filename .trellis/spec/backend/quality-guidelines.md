# 质量规范（Quality Guidelines）

> proxy-hub 后端代码质量标准。由 M1 确立；`trellis-check` 与提交前校验据此把关。

---

## 概览

- 提交前必须全绿：
  ```bash
  gofmt -l cmd internal web     # 无输出（已格式化）
  go vet ./...                  # 无告警
  CGO_ENABLED=0 go build -o ./bin/proxy-hub ./cmd/proxy-hub
  go test ./...
  ```
- 二进制必须 `CGO_ENABLED=0` 可构建（纯 Go，跨平台）。
- 依赖保持最小；新增依赖需在该里程碑 design 中说明理由。

---

## 必守模式（Required）

- **代码注释一律中文**（见 `AGENTS.md` 语言规范）；所有导出标识符（函数/类型/常量/包）有中文 doc 注释。
- 包主文件顶部有 `// Package xxx ...` 中文包注释。
- 构造器 `New*` / 生命周期 `Open/Close/Load/Run`；返回 `error` 而非 panic（见 `error-handling.md`）。
- 时间/超时用具名常量（`pingTimeout`、`shutdownTimeout`、`debounceInterval`、`healthCheckTimeout`），不要散落魔数。
- 配置优先级固定：默认值 → yaml → 环境变量（`PROXY_HUB_*`）→ 校验（见 `config.Load`）。
- 改任何常量/配置/端口前**先全局搜索**所有引用再改（见 `guides/`：Pre-Modification Rule）。本项目端口约定见记忆与父 `prd.md`（后端 7777 / 前端开发 8888 / 生产单端口）。

---

## 禁用模式（Forbidden）

- ❌ float 存金额（用 decimal TEXT）。
- ❌ GORM / AutoMigrate（用显式迁移 + 手写 SQL / 后续 sqlc）。
- ❌ 密钥入 DB 或日志；持久化请求/响应体。
- ❌ 在热路径（中转）做同步 DB I/O（路由读内存 RouteIndex；用量异步落库）。
- ❌ 读句柄写数据 / 写句柄开大并发（破坏单写者，见 `database-guidelines.md`）。
- ❌ `gin.Default()`（带噪音 logger）——用 `gin.New()` + 自定义中间件，日志走 slog。
- ❌ 吞错误 / 用 `%v` 丢失错误链。

---

## 测试要求

- 用标准 `testing`；表驱动优先；临时资源用 `t.TempDir()`、环境变量用 `t.Setenv`（自动隔离/还原）。
- 每个有逻辑的包带单测。M1 基线覆盖：
  - `config`：默认值、yaml 覆盖、env 覆盖、缺失文件容错、校验错误、派生路径、`EnsureAdminKey`。
  - `store`：全新库迁移版本=1、重复 no-op、预迁移备份生成、坏迁移事务回滚（版本不变 + 备份在）、`rebuildTable`、`HealthCheck`（正常 nil / 关闭后报错）、`Open` 建库+auths 目录。
- 涉及外部 I/O（HTTP 上游、文件写）的测试用本地 stub / 临时目录，不依赖网络与真实 HOME。
- M2 跨方言转换器、M4 fileio 往返必须有一致性/往返测试守门。

---

## 代码评审清单（trellis-check 关注点）

- [ ] `gofmt`/`go vet`/`go build`(CGO 关)/`go test` 全绿。
- [ ] 新增/修改代码注释为中文；导出符号有 doc 注释。
- [ ] 无密钥落 DB/日志；无请求/响应体持久化。
- [ ] 读写句柄使用正确；热路径无同步 DB I/O。
- [ ] 错误用 `%w` 包装、边界翻译；HTTP 错误返回 JSON + 合适状态码 + `X-Request-Id`。
- [ ] 改了常量/端口/配置 → 全仓搜索过所有引用并同步（含 Dockerfile/compose/docs）。
- [ ] 迁移：版本连续、放 `internal/store/migrations/`、改表用 `rebuildTable`。
