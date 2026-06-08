# M4 —— Codex/Claude MCP 共享管理

> `06-04-proxy-hub-mvp` 的子任务。共享设计见父 `design.md`（能力 C4）。逻辑从 cc-switch
> 移植（Rust → Go）。这是一个独立模块：只与 proxy-hub 其余部分共享内嵌 DB 与 `fileio` 原子写助手。
>
> 语言约定：本项目所有规划文档与代码注释一律使用中文（见 `AGENTS.md`）。

## 目标

让运维在 proxy-hub 中只定义一次 MCP 服务器，便能（带格式转换）把它投影进 Codex
（`~/.codex/config.toml`）与 Claude Code（`~/.claude.json`）的客户端配置，且绝不破坏无关内容
（尤其 Claude 的 `projects` 历史）、绝不隐式改写 `$HOME`。

## 范围

**范围内**
- 表：`mcp_servers`（SSOT：id、name、规范 `spec_json`、per-client 启用位图、元数据）+
  `mcp_sync_targets`（运维登记的绝对配置路径）。
- `internal/mcp/store`（DAO）、`internal/mcp/service`（SSOT 编排：upsert / toggle / delete /
  sync-target / sync-all，按前后位图 diff → 各客户端 写入/移除）、`internal/mcp/validation`
  （宽松：对象 + type∈{stdio,http,sse} + 必填 command/url；保留未知字段）。
- `internal/mcp/clients`：
  - **Claude**（`claude.go`）：读取整个 `~/.claude.json`，只替换 `mcpServers`，**字节级保留其余
    所有顶层键含 `projects`**；剥离 UI 辅助字段；Windows 下对 `npx/npm/node/...` 做 `cmd /c`
    包装并带 WSL-UNC-path 检测；原子写。
  - **Codex**（`codex.go`）：`pelletier/go-toml/v2` 仅对 `[mcp_servers.<id>]` 外科手术式编辑；
    清理遗留 `[mcp.servers]`；stdio→command/args/env/cwd 与 http/sse→url/http_headers；扩展字段
    透传。
- `internal/fileio`：原子 temp+rename、首次写前 `.bak`、拷贝源权限、父目录缺失则跳过、**按目标
  加锁 + 写前立即重读**（并发写安全）。
- API `/v0/mcp/*`：注册表 CRUD、per-client toggle、目标 CRUD、显式 `POST /v0/mcp/sync`
  （+ 按目标）全量对账、只读 `POST /v0/mcp/import`（冲突 ⇒ 翻开关位，绝不覆盖 spec；"spec
  differs"告警）、`POST /v0/mcp/import-bundle`（朴素 `{mcpServers}` + apps CSV）。
- CLI `proxy-hub mcp sync`；管理 UI：server 列表 + per-client 开关 + "立即同步" + 目标管理。
- 可选的单用户 HOME 自动探测 `~/.codex` / `~/.claude.json`（默认关）。

**范围外**
- 双向/持续自动对账、监听客户端配置变更。Codex + Claude 以外的客户端。多租户 per-user 目标 /
  RBAC。`ccswitch://` deeplink 格式（用朴素 `import-bundle`）。密钥保险库。转发 per-request 内联
  `mcp_servers`。

## 需求（来自父 `prd.md`）

FR-C4-1 … FR-C4-5；NFR-5（绝不破坏客户端配置）。

## 依赖

**依赖 M1**（DB + `fileio` 底座——若 M1 未含，`fileio` 助手可在此构建）。其余与 M2/M3 无关。
**可与 M3 并行。**

## 交付清单

- [ ] `mcp_servers` + `mcp_sync_targets` 迁移；`internal/mcp/store` DAO。
- [ ] `internal/mcp/service` SSOT 编排（位图 diff → 同步/移除）。
- [ ] `internal/mcp/validation`（宽松、保留字段）。
- [ ] `internal/mcp/clients/claude.go`（整文件保留 + Windows cmd/c + WSL）。
- [ ] `internal/mcp/clients/codex.go`（toml 外科手术式编辑 + 遗留清理 + 转换）。
- [ ] `internal/fileio/atomic.go`（原子 + .bak + 锁 + 写前重读 + 跳过缺失）。
- [ ] `/v0/mcp/*` API + `proxy-hub mcp sync` CLI + 管理 UI（列表/开关/同步/目标/导入）。

## 验收标准

- [ ] 定义一个 MCP server，启用 Codex + Claude，"立即同步"：它作为 `[mcp_servers.<id>]` 出现在
      `~/.codex/config.toml`，作为 `mcpServers` 下的项出现在 `~/.claude.json`。
- [ ] **所有无关内容字节级保留**：Claude `projects` 历史及其它顶层键、Codex `[mcp_servers]` 之外的
      表/注释完好（用填充过的 `~/.claude.json` 与带注释的 `config.toml` 做往返测试验证）。
- [ ] 禁用某客户端只从该客户端文件移除该 server。
- [ ] 导入把现有客户端配置读入注册表；id 冲突时翻开关位并告警"spec differs"——绝不覆盖已存 spec。
- [ ] 模拟读-写之间的外部并发编辑不被覆盖（锁 + 写前重读）；每个目标首次写后存在 `.bak`。
- [ ] Windows stdio server 被 `cmd /c` 包装（WSL 目标跳过）。
- [ ] `gofmt`/`go vet`/`go test ./...`（含往返/保留测试）+ `web` 检查 干净；`trellis-check` 干净。

## 备注

参考（只读）cc-switch 文件：`src-tauri/src/services/mcp.rs`、`src-tauri/src/mcp/codex.rs`、
`src-tauri/src/claude_mcp.rs`、`src-tauri/src/config.rs`、`src-tauri/src/database/dao/mcp.rs`。
子任务 `design.md`/`implement.md` 在子任务启动时撰写。
