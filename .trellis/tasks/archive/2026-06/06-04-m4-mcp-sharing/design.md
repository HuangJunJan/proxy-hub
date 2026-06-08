# M4 —— Codex/Claude MCP 共享管理：技术设计（Design）

> `06-04-m4-mcp-sharing` 子任务设计。继承父 `design.md`（能力 C4）。逻辑从 cc-switch（Rust）移植。
> 独立模块：仅与其余部分共享内嵌 DB 与 `fileio`。语言：注释中文；标识符/路径英文；`.sql` 查询 ASCII 注释。

## 0. 铁律（父 NFR-5）

**绝不破坏客户端配置无关内容**：Claude `~/.claude.json` 的 `projects` 历史与其它顶层键、Codex
`~/.codex/config.toml` 的 `[mcp_servers]` 之外的表与注释，必须保留。绝不隐式改写 `$HOME`：只写
**显式登记**的 `mcp_sync_targets`（可选 HOME 自动探测默认关）。

## 1. 保留策略（关键取舍）

- **Claude（JSON）用 `sjson` 外科手术**：`sjson.SetBytes(raw, "mcpServers", obj)` 只改 `mcpServers` 一个
  顶层键的值，其余字节（含 `projects`）原样保留。删除用 `sjson.DeleteBytes`。空文件起始为 `{}`。
  比 `encoding/json` 整体往返（会重排键/改格式）保真得多。`sjson` 已在 M2 引入。
- **Codex（TOML）用文本段手术**：`go-toml/v2` 全量 Unmarshal→Marshal **不保留注释**，故写入改为
  **按表头文本切割**：识别所有顶层 `[mcp_servers...]` / 遗留 `[mcp.servers...]` 段（从其 `[header]`
  行到下一个顶层 `[header]` 或 EOF），删除这些段，再把启用服务器序列化为新 `[mcp_servers.<id>]` 段
  追加到文末。`[mcp_servers]` 之外的表与注释逐字保留。读取/导入仍用 `go-toml/v2` Unmarshal 解析。
- 新依赖：`github.com/pelletier/go-toml/v2`（仅 Codex 解析）。

## 2. 数据模型（迁移 `0004_mcp.sql`）

### mcp_servers —— MCP SSOT
```
id TEXT PRIMARY KEY              -- = 配置键（如 "context7"）
name TEXT NOT NULL DEFAULT ''
spec_json TEXT NOT NULL          -- 规范宽松 spec：stdio {type,command,args,env,cwd} / http|sse {type,url,headers}；原样存，保留未知字段
description TEXT NOT NULL DEFAULT ''
homepage TEXT NOT NULL DEFAULT ''
docs TEXT NOT NULL DEFAULT ''
tags_json TEXT NOT NULL DEFAULT '[]'
enabled_codex INTEGER NOT NULL DEFAULT 0
enabled_claude INTEGER NOT NULL DEFAULT 0
created_at TEXT NOT NULL
updated_at TEXT NOT NULL
```
### mcp_sync_targets —— 显式可写客户端配置
```
id INTEGER PRIMARY KEY AUTOINCREMENT
client TEXT NOT NULL CHECK(client IN ('codex','claude'))
config_path TEXT NOT NULL        -- 绝对路径，运维登记
label TEXT NOT NULL DEFAULT ''
enabled INTEGER NOT NULL DEFAULT 1
last_synced_at TEXT
last_sync_status TEXT NOT NULL DEFAULT ''
created_at TEXT NOT NULL
updated_at TEXT NOT NULL
```
索引：`mcp_sync_targets(client)`。

## 3. internal/fileio —— 原子写 + 锁 + 备份

```go
// UpdateFile 在按 path 加锁下：父目录缺失则返回 ErrParentMissing（跳过）；读当前内容（不存在=空）；
// 首次写前生成 <path>.bak（已存在则不覆盖）；调 transform 得新内容；原子 temp+rename 落盘（拷贝源权限，缺省 0600）。
// 写前重读由「锁内读当前 → transform」保证：活跃客户端在读写之间的改动不被覆盖。
func UpdateFile(path string, transform func(current []byte) (next []byte, err error)) error
var ErrParentMissing = errors.New(...)
```
- 每 path 一把 `sync.Mutex`（全局 map + 守护锁）。同进程并发同目标互斥。
- `.bak` 仅首次（坏同步的回滚点）。temp 同目录 + `os.Rename`（同卷原子）。

## 4. internal/mcp/validation —— 宽松校验

`Validate(spec map[string]any) error`：须为对象；`type` ∈ {stdio,http,sse}（缺省 stdio）；stdio 须有
`command`，http/sse 须有 `url`。**保留所有未知字段**（不裁剪 spec，往返不丢信息）。

## 5. internal/mcp/store —— DAO

包裹 dbgen：`UpsertServer/GetServer/ListServers/DeleteServer/SetEnabled(client,on)`、
`CreateTarget/ListTargets/GetTarget/UpdateTarget/DeleteTarget/SetTargetStatus`。领域类型 `Server`
（id/name/spec(map)/meta/enabledCodex/enabledClaude）、`Target`。

## 6. internal/mcp/clients —— 投影写入器

接口（service 调用）：把「启用集」投影进单个目标文件。
- `claude.Apply(path string, servers []Server) error`：经 `fileio.UpdateFile` → 用 `sjson` 把
  `mcpServers` 设为 `{id: claudeSpec}`。`claudeSpec` 剥离 UI 辅助字段（description/homepage/docs/tags），
  **Windows 非 WSL** 对 `command∈{npx,npm,node,pnpm,yarn,bunx,...}` 做 `cmd /c <command> <args...>` 包装
  （`args` 前插 `/c`+原 command）；WSL 目标（路径以 `\\wsl$`/`\\wsl.localhost` 或 `/mnt/` 线索）跳过包装。
- `codex.Apply(path string, servers []Server) error`：经 `fileio.UpdateFile` → 文本段手术删除旧
  `[mcp_servers*]`/`[mcp.servers*]` → 追加新段：stdio→`command`/`args`/`[.env]`/`cwd`；http|sse→`url`/
  `[.http_headers]`；扩展字段透传。TOML 字符串/数组/表正确转义。
- 导入：`claude.Read(path)`/`codex.Read(path)` 解析现有 `mcpServers`/`[mcp_servers]` 为 `[]Server`。

## 7. internal/mcp/service —— SSOT 编排

```go
UpsertServer(s Server)            // 校验 spec → DAO upsert
ToggleClient(id, client, on)      // 改位图 → 对该 client 的所有启用目标重新 Apply（含移除）
DeleteServer(id)                  // DAO 删 → 各 client 目标移除该 id
SyncAll() / SyncTarget(targetID)  // 全量对账：按目标 client + 各 server 的 enabled_<client> 位图 Apply
Import(targetID)                  // 只读读入：现有项 → 注册表；id 冲突 ⇒ 翻该 client 开关位 + 告警 "spec differs"，绝不覆盖已存 spec
ImportBundle({mcpServers}, apps)  // 朴素 bundle 导入
```
位图 diff：编辑/toggle 时按「前后 enabled_<client>」决定对各 client 目标写入（仍启用/新启用）或移除（新禁用）。每次 Apply 后写 `last_synced_at`/`last_sync_status`。

## 8. API `/v0/mcp/*`（admin key 守护）+ CLI

- `GET/POST /v0/mcp/servers`、`GET/PUT/DELETE /v0/mcp/servers/:id`、`PUT /v0/mcp/servers/:id/toggle`（body {client,enabled}）。
- `GET/POST /v0/mcp/targets`、`PUT/DELETE /v0/mcp/targets/:id`。
- `POST /v0/mcp/sync`（全量）、`POST /v0/mcp/sync/:targetID`、`POST /v0/mcp/import/:targetID`、`POST /v0/mcp/import-bundle`。
- CLI `proxy-hub mcp sync`：装配 DB + service → SyncAll（运维 cron 友好）。
- 管理 UI：server 列表 + per-client 开关 + 「立即同步」+ 目标管理 + 导入。

## 9. 安全

MCP spec 的 env/headers 可能含密钥：DB + 写出文件继承受限权限（0600）；不额外记日志。写仅限显式
target。`/v0/mcp/*` 走 admin key。

## 10. 测试

- fileio：原子写、`.bak` 仅首次、父目录缺失跳过、并发锁、写前重读（锁内读到外部改动）。
- claude：填充过的 `~/.claude.json`（含 `projects` + 其它键）往返 → 仅 `mcpServers` 变、其余字节不变；cmd/c 包装；WSL 跳过；移除。
- codex：带注释 + 无关表的 `config.toml` 往返 → `[mcp_servers]` 外逐字保留；遗留 `[mcp.servers]` 清理；stdio/http 序列化；扩展字段透传。
- service：位图 diff（启用→写、禁用→移除）、删除移除、导入冲突翻位不覆盖。
- validation：type/必填/保留未知字段。

## 11. 回滚

特性分支；迁移 0004 前向 + 预迁移备份。客户端文件首次写前 `.bak` 即回滚点。导入只读、同步单向。
