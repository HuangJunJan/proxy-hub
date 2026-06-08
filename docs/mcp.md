# MCP 共享同步指南

proxy-hub 把 MCP 服务器在一处定义（SSOT，`mcp_servers` 表），按 per-client 启用位图投影进 Codex 与
Claude Code 的客户端配置文件，**绝不破坏无关内容**（Claude 的 `projects` 历史等顶层键、Codex `[mcp_servers]`
之外的表与注释）。同步单向（proxy-hub → 客户端），导入只读。

## 概念

- **server**：一个 MCP 服务器定义。`id` = 客户端文件里的配置键。`spec` 宽松：stdio `{type,command,args,env,cwd}`
  / http|sse `{type,url,headers}`（`type` 省略视为 stdio；未知字段保留）。每 server 带 `enabled_codex` /
  `enabled_claude` 开关位。
- **target**：一个**显式登记**的可写客户端配置文件（绝对路径 + client 类型）。proxy-hub **绝不**自动探测
  `$HOME`，只写你登记的 target。

## 操作（控制台 MCP 页 或 API）

| 动作 | API |
|---|---|
| 列出/新增/改/删 server | `GET/POST /v0/mcp/servers`、`PUT/DELETE /v0/mcp/servers/:id` |
| 按客户端开关 | `PUT /v0/mcp/servers/:id/toggle`（body `{client,enabled}`） |
| 列出/新增/改/删 target | `GET/POST /v0/mcp/targets`、`PUT/DELETE /v0/mcp/targets/:id` |
| 同步（全量 / 按 target） | `POST /v0/mcp/sync`、`POST /v0/mcp/sync/:id` |
| 从现有配置导入 | `POST /v0/mcp/import/:id`（按 target）、`POST /v0/mcp/import-bundle`（`{mcpServers,apps}`） |

CLI（cron 友好，无需起服务）：`proxy-hub mcp sync [--config config.yaml]`。

## 行为细节

- **启用/禁用**：toggle 某 client 启用位后即对该 client 的所有启用 target 重对账——启用 ⇒ 写入/更新该
  server；禁用 ⇒ 仅从该客户端文件移除该 server（不动其它项）。
- **保留**：Claude 用 `sjson` 只改 `mcpServers` 键（其余字节保留）；Codex 用文本段手术只改 `[mcp_servers]`
  段（保留注释/无关表），并清理遗留 `[mcp.servers]`。每个 target 首次写前生成 `<path>.bak`。
- **Windows**：stdio 的 `npx/npm/node/...` 自动包成 `cmd /c <command> <args...>`。
- **导入**：把目标现有 MCP 配置读入注册表；**id 冲突时只翻开关位 + 告警 "spec differs"，绝不覆盖已存 spec**。
- **并发安全**：按目标路径加锁 + 锁内「读现状→改→原子写」，活跃客户端在读写之间的改动不被覆盖。

## 安全

MCP spec 的 `env`/`headers` 可能含密钥：存 DB + 写出文件继承受限权限；不记日志完整 spec。`/v0/mcp/*` 走
admin key，且改状态请求受同源校验（见 [security.md](./security.md)）。
