# 错误处理（Error Handling）

> proxy-hub 的错误处理约定。由 M1 确立。

---

## 概览

- Go 惯用法：**返回 error，不 panic**。库/包函数把失败作为 `error` 返回值上抛。
- 包装传播：用 `fmt.Errorf("上下文: %w", err)` 包裹并保留链，用中文描述上下文；判定用 `errors.Is`/`errors.As`。
- 多错误合并用 `errors.Join`（见 `store.Close` 同时关读写句柄）。
- panic 只应来自真正不可恢复的程序错误；HTTP 路径由 recover 中间件兜底（见下）。

---

## 错误类型

- M1 暂不定义自定义 error 类型，统一用 `fmt.Errorf` + 哨兵判定（`errors.Is(err, os.ErrNotExist)`、`sql.ErrNoRows`）。
- 需要分类（如 M2 上游 HTTP 状态驱动冷却、M3 错误分类器）时，再引入带 `Code`/`Kind` 的领域 error 类型，并在边界翻译。

---

## 错误处理范式

- **启动期错误**：层层 `%w` 上抛到 `cmd/proxy-hub/main.go` 的 `run()`，由 `main` 打到 stderr 并 `os.Exit(1)`：
  ```go
  if err := run(); err != nil {
      fmt.Fprintf(os.Stderr, "proxy-hub 启动失败: %v\n", err)
      os.Exit(1)
  }
  ```
- **资源清理**：`defer func() { _ = x.Close() }()`；清理错误若无需上抛可显式忽略（`_ =`），不要静默裸调用。
- **非致命降级**：能继续就记日志后继续（如 `config.Watch` 失败 → `slog.Warn` 后不阻塞启动）。
- **HTTP panic 兜底**：`middleware.Recover()` 捕获 handler panic → `slog.Error`（带 request_id）→ 返回 500，绝不让进程崩溃。

---

## API 错误响应

- 统一 JSON 形状：`gin.H{"error": "<message>"}`，配合恰当 HTTP 状态码。M1 既有：
  - body 超限 → `413` `{"error":"request body too large"}`（`middleware.BodyLimit`）。
  - 鉴权失败 → `401` `{"error":"unauthorized"}`（`middleware.Auth` 雏形）。
  - panic → `500` `{"error":"internal server error"}`（`middleware.Recover`）。
  - `/healthz` DB 异常 → `503`，正常 `200`（`{"status","version","db"}`）。
- 每个响应都带 `X-Request-Id`（`middleware.RequestID`），便于按 `request_id` 追踪日志/请求。
- M2 接入 OpenAI/Claude 兼容端点时，错误体需翻译为对应方言的错误形状（届时在 adaptor 层处理）。

---

## 常见错误

- ❌ 吞掉 error（`x, _ := f()` 后当成功用）。除非确为可忽略的清理错误。
- ❌ 丢失错误链（`fmt.Errorf("...: %v", err)` 用 `%v` 而非 `%w`）→ 用 `%w`，除非有意切断链。
- ❌ 在 handler 里 `panic` 表达普通业务错误 → 返回状态码 + JSON。
- ❌ 把内部错误细节（路径、SQL、密钥）原样回给客户端 → 客户端只给概要，细节进日志。
