# 日志规范（Logging Guidelines）

> proxy-hub 的日志约定。由 M1 确立。

---

## 概览

- 库：标准库 **`log/slog`**（结构化），零额外依赖。
- 全局默认 logger 在启动时由 `cmd/proxy-hub/main.go` 的 `setupLogger(cfg)` 配置，输出到 `stdout`。
- 格式由配置 `log.format` 决定：`text`（默认，开发友好）或 `json`（生产/采集）。
- 级别由 `log.level` 决定：`debug|info|warn|error`，可经 `config.yaml` 热重载（非密钥键）。

---

## 日志级别

- `debug`：开发期细节（请求路由决策、选择器候选等），生产默认关闭。
- `info`：正常生命周期事件（启动、监听、迁移完成、热重载生效、优雅停机）。
- `warn`：可恢复异常或需关注项（admin_key 自动生成、热重载失败保留旧配置、配置监听出错、用量缓冲溢出回退）。
- `error`：操作失败（panic 兜底、停机超时、写库失败）。

---

## 结构化日志

- 用 **key-value 属性**，不要把变量拼进 message：
  ```go
  slog.Info("已应用迁移", "name", m.name, "version", m.version)   // ✅
  slog.Info(fmt.Sprintf("已应用迁移 %s", m.name))                  // ❌
  ```
- message 用中文短语；属性 key 用英文小写下划线（`request_id`、`from_version`、`data_dir`）。
- HTTP 相关日志尽量带 `request_id`（`middleware.RequestIDFromContext(c)`），串起一次请求的多条日志。

---

## 该记什么

- 进程生命周期：启动（version/addr/data_dir）、监听、迁移（备份/版本迁移/完成）、停机信号、已停止。
- 异常路径：panic、停机超时、配置热重载失败、资源打开失败。
- M2+：渠道选择/失败转移/冷却切换、用量缓冲溢出计数（**非静默**）、MCP 同步结果。

---

## 不该记什么（安全红线）

- ❌ **绝不**记录上游 API key、入站 key 原文、`admin_key`（仅首次自动生成时 `warn` 打印一次，且明确提示"仅此一次"）。
- ❌ **绝不**记录请求体 / 响应体（隐私 + 体量）。错误信息需截断。
- ❌ 不记录 MCP spec 中的 env/headers 等可能含密钥的字段原文。
- ❌ 不把完整 `Authorization`/`x-api-key` 头写进日志。
