# M4 —— Codex/Claude MCP 共享管理：执行计划（Implement）

> 依据本子任务 `prd.md` + `design.md` + 父设计。注释中文；`.sql` 查询 ASCII 注释。

## 前置
- 分支 `task/m4-mcp-sharing`（从含 M1-M3 的 HEAD）。新依赖 `github.com/pelletier/go-toml/v2`（Codex 解析）。
- 参考（只读）cc-switch：`claude_mcp.rs`、`mcp/codex.rs`、`services/mcp.rs`、`config.rs`。

## 有序步骤（每步可独立编译 + 测试）
1. **迁移 0004 + queries + sqlc**：`mcp_servers` + `mcp_sync_targets`（design §2）+ 索引。queries（ASCII）：servers CRUD + SetEnabled、targets CRUD + SetTargetStatus。`sqlc generate` → 提交 dbgen；`go build ./internal/store/...`。
2. **internal/fileio/atomic.go**：`UpdateFile(path, transform)`（锁 + 父目录缺失跳过 + 读当前 + 首次 .bak + 原子 temp+rename + 拷权限）。单测：原子写、.bak 仅首次、并发锁、缺父目录跳过、写前重读。
3. **internal/mcp/validation.go**：`Validate(spec)` 宽松 + 保留未知字段。单测。
4. **internal/mcp/store/dao.go**：dbgen↔领域（Server/Target）；CRUD + SetEnabled + SetTargetStatus。
5. **internal/mcp/clients/claude.go**：`Apply`（sjson 设 mcpServers，剥 UI 字段，Windows cmd/c + WSL 跳过）+ `Read`（解析 mcpServers）。单测：填充 json 往返保留 projects、cmd/c、移除。
6. **internal/mcp/clients/codex.go**：`Apply`（文本段手术删旧 mcp_servers/遗留 mcp.servers + 追加新段，TOML 转义、env/headers 子表）+ `Read`（go-toml/v2 解析）。单测：带注释/无关表往返保留、遗留清理、stdio/http 序列化、扩展透传。
7. **internal/mcp/service/service.go**：UpsertServer/ToggleClient/DeleteServer/SyncAll/SyncTarget/Import/ImportBundle（位图 diff → 各 client 目标 Apply/移除 + 写 target 状态）。单测：位图 diff、删除移除、导入冲突翻位不覆盖。
8. **api/mcp_handlers.go** + server.go 注册 `/v0/mcp/*`（admin key 组）；main 装配 mcp service。
9. **CLI `proxy-hub mcp sync`**：main 的 dispatchSubcommand 接 mcp sync → 装配 DB + service → SyncAll。
10. **前端**：MCP 管理页（server 列表 + per-client 开关 + 立即同步 + 目标 CRUD + 导入）；加到 App 标签页。
11. **验证 + 端到端**：建 server→启用双端→登记 target(临时文件)→sync→断言 toml/json 内容 + 无关内容保留。

## 校验命令（完成前全绿）
```bash
sqlc generate && git diff --exit-code internal/store/dbgen
gofmt -l cmd internal && go vet ./... && CGO_ENABLED=0 go build ./... && go test ./...
cd web && npm run build   # 若改前端
```

## 评审门 / 回滚
评审门：全绿 + `trellis-check` + 端到端（建→启用→同步→保留验证）。回滚：分支回退；迁移 0004 预备份；客户端 .bak。

## 完成后
`trellis-check`（主会话内联兜底）→ 提交（用户要求时）→ `/trellis:finish-work` → 更新 `.trellis/spec/`（MCP 投影/保留/fileio 约定）。
