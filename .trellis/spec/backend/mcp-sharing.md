# MCP 共享与文件投影规范（MCP Sharing Guidelines）

> proxy-hub M4 确立的 MCP SSOT、向外部客户端配置的安全投影、与 fileio 原子写契约。独立模块。
> 语言：中文注释；标识符/路径英文；`.sql` 查询 ASCII 注释。

---

## 铁律（父 NFR-5）

向外部客户端配置文件（`~/.claude.json`、`~/.codex/config.toml`）写入时，**绝不破坏无关内容**：
Claude 的 `projects` 历史与其它顶层键、Codex `[mcp_servers]` 之外的表与注释必须保留。绝不隐式改写
`$HOME`：只写 `mcp_sync_targets` 中**显式登记的绝对路径**。同步单向；导入只读。

## fileio（internal/fileio）—— 原子写 + 锁 + 备份

```go
func UpdateFile(path string, transform func(current []byte) ([]byte, error)) error
var ErrParentMissing = errors.New(...)   // 父目录不存在 ⇒ 跳过（不建目录树）
```
- 按 path 进程内互斥：**锁内「读当前 → transform → 原子落盘」**，使活跃客户端在读-写之间的外部改动不被覆盖（写前重读语义）。
- 首次触碰已存在文件先写一次 `<path>.bak`（恢复点；同进程仅首次）。temp + `os.Rename`（同目录原子；Windows 共享冲突有界重试）。沿用原文件权限（新建 0600）。
- 「监听目录又删改其中文件」「投影写外部配置」一律走 fileio，不要裸 `os.WriteFile`。

## 投影策略（关键取舍：保留 > 重写）

- **Claude（JSON）用 sjson 外科手术**：逐 id `sjson.SetBytes(buf, "mcpServers."+escapeJSONKey(id), obj)`（启用）或 `DeleteBytes`（禁用），**只触碰 mcpServers 子树**，其余顶层键字节级保留。含点/通配的 id 用 `escapeJSONKey` 转义。非托管 mcpServers 项保留（只增删本注册表内 id）。空文件起始 `{}`。
- **Codex（TOML）用文本段手术**：go-toml/v2 全量 Marshal **不保留注释**，故写入靠**按表头切割**——删除所有 `[mcp_servers*]` / 遗留 `[mcp.servers*]` 段（表头到下一表头/EOF），其余行逐字保留；再把启用 server 手写为新 `[mcp_servers.<id>]` 段（`tomlString`/`tomlArray` 转义、env/http_headers 子表、扩展字段透传）追加。读取/导入用 go-toml/v2 Unmarshal。
- **Windows stdio 包装**：非 WSL 目标对 `npx/npm/node/pnpm/yarn/bunx/...` 包成 `cmd /c <command> <args...>`（`isWrappable` 白名单）；WSL 目标（路径含 `\\wsl$`/`\\wsl.localhost`/`/mnt/`）跳过。由 service 按 `goos==windows && !isWSLPath(target)` 传入 `wrapWindows`。

## service 编排（位图 diff）

`UpsertServer`（校验 + 保留已存 enabled 位 + 全量重对账）、`ToggleClient(id,client,on)`（改位图 → 对该 client 目标重对账）、`DeleteServer`（以「禁用」并入对账使各文件移除其项）、`SyncAll`/`SyncTarget`、`Import`（只读：id 冲突仅翻开关位 + `spec differs` 告警，**绝不覆盖已存 spec**）、`ImportBundle`（朴素 `{mcpServers}` + apps）。
对账失败按 target 记 `last_sync_status`；父目录缺失记 `skipped` 非硬错。

## API / CLI

- `/v0/mcp/*`（admin key 守护，与 `/admin/*` 同级）：servers CRUD + `:id/toggle`、targets CRUD、`POST /sync`·`/sync/:id`·`/import/:id`·`/import-bundle`。
- CLI `proxy-hub mcp sync [--config p]`：装配 DB + service.SyncAll（cron 友好，无需起 HTTP）。

## 数据 / 安全

- `mcp_servers`（id=配置键 PK、spec_json 原样存保留未知字段、per-client 启用位图、UI 元数据）、`mcp_sync_targets`（client、绝对 config_path、enabled、last_sync_*）。迁移 `0004_mcp.sql`。
- spec 的 env/headers 可能含密钥：存 DB + 写出文件继承受限权限；**不记日志完整 spec**。`/v0/mcp/*` 走 admin key。

## 测试要点

- fileio：原子写、`.bak` 仅首次、缺父目录跳过、锁串行化、写前重读。
- 投影**保留性往返**（NFR-5 硬门）：填充 `~/.claude.json`（projects + 顶层键 + 非托管项）→ 仅 mcpServers 变；带注释/无关表 `config.toml` → `[mcp_servers]` 外逐字保留 + 遗留 `[mcp.servers]` 清理；Windows cmd/c；禁用移除。
- service：位图 diff、删除移除、导入冲突翻位不覆盖 spec。
