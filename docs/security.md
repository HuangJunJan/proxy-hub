# 安全模型

proxy-hub 的安全围绕「密钥分层 + 最小持久化 + 同源写保护」。

## 密钥处理

| 密钥 | 存储 | 规则 |
|---|---|---|
| 上游凭证（渠道 API key / upstream base_url+key） | `data/auths/<channel_id>.json`，权限 0600 | **绝不**入 SQLite、绝不记日志、绝不进用量事件；对外 DTO 仅以 `has_credential` 标识 |
| 入站平台 key | `api_keys.hash`（sha256 hex） | 明文仅创建时返回一次；查找走内存缓存（正/负缓存，fail-closed）；比较用 `subtle.ConstantTimeCompare` |
| admin key | `config.yaml` 的 `admin_key` 或环境 `PROXY_HUB_ADMIN_KEY` | 留空则首次运行生成并打印一次；守护 `/admin/*` 与 `/v0/mcp/*` |

## 鉴权与 CSRF

- `/v1/*`（中转）走**入站 key**（`Authorization: Bearer` 或 `x-api-key`）。
- `/admin/*` 与 `/v0/mcp/*` 走 **admin key**（bearer）。SPA 输入 admin key 存 `localStorage` 并注入请求头。
- **同源写保护**：这些端点的改状态请求（非 GET/HEAD/OPTIONS）做 Origin/Referer 同源校验——浏览器跨域写
  请求被拒（403）；非浏览器客户端（无 Origin，如 curl/CLI）由 bearer 保护放行。`server.allowed_origins`
  可额外放行受信前端域。
- 因采用 bearer（非 cookie），CSRF 本就难以利用；同源校验为纵深防御。

## 数据最小化

- **绝不持久化**请求/响应体；统计只存 token/延迟/状态维度。
- `error_message` 截断（relay 256B、admin 200B）；连接错误串不含密钥（key 在请求头不在 URL）。
- 用量事件经有界 channel 非阻塞发出；溢出**计数并经仪表盘暴露**（非静默），可选同步兜底绝不丢计费。

## 文件投影（MCP）

写外部客户端配置（`~/.claude.json`、`~/.codex/config.toml`）：整文件保留无关内容 + 原子 temp+rename +
首写 `.bak` + 按路径加锁 + 锁内写前重读。只写**显式登记**的 target，绝不隐式改写 `$HOME`。

## 部署

单 `/data` 卷（含 `proxy-hub.db`、`auths/*.json` 0600、`config.yaml`）。迁移前向式 + 预迁移 `.db` 备份；
回滚 = 恢复备份 + 旧二进制。
