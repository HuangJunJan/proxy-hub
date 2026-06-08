# proxy-hub

> 单二进制、可自托管的 **API-Key 网关**：把 OpenAI / Claude 官方 API-Key 与第三方 OpenAI/Claude
> 兼容上游，统一到 OpenAI- 与 Claude-兼容端点之后；带模型映射、负载均衡/故障转移、用量统计与监控，
> 以及 Codex/Claude 的 MCP 服务器共享管理。Go 1.25 + 内嵌 SQLite，`CGO_ENABLED=0` 跨平台单文件。

**不做**：订阅/OAuth 渠道（仅 API-Key + 自定义 upstream）。理由见下「范围」。

---

## 能力

- **C1 渠道与中转**：统一端点 `/v1/chat/completions`、`/v1/messages`、`/v1/responses`、`/v1/models`；
  模型映射（精确 / 尾部 `*` 通配 / `prefix` 命名空间）；最高优先级档加权随机（或 `fill_first`）+ 会话亲和；
  可重试错误跨渠道故障转移；per-(渠道,模型) 冷却。同方言透传；OpenAI⇄Claude 跨方言转换（特性开关后）。
- **C2 统计与监控**：非阻塞采集每请求用量 → 批量落库 + 时序滚动汇总；**读取时**按 `model_pricing`（decimal）
  算成本（改价可重算历史）；仪表盘（概览/趋势/分组/请求日志钻取/渠道健康/定价）；可选主动健康探测。
- **C3 轻量部署**：单静态二进制（`go:embed` 前端 + 迁移 + 定价种子）；单 `/data` 卷、单端口。
- **C4 MCP 共享**：MCP 服务器只定义一次，投影进 Codex `~/.codex/config.toml` 与 Claude `~/.claude.json`，
  **绝不破坏无关内容**（Claude `projects`、Codex 注释/无关表）。

## 快速开始

```bash
# 1. 构建（前端已随仓库提交于 web/dist；改前端后需 cd web && npm ci && npm run build）
CGO_ENABLED=0 go build -o proxy-hub ./cmd/proxy-hub

# 2. 起服务（admin_key 留空则首次运行自动生成并打印一次，请保存）
./proxy-hub --config config.example.yaml
#   监听 :7777；控制台 SPA 同端口提供。健康检查：
curl -fsS http://127.0.0.1:7777/healthz

# 3. 建一个上游渠道（OpenAI 官方 key 为例）
curl -X POST -H "Authorization: Bearer $ADMIN_KEY" -H "Content-Type: application/json" \
  -d '{"name":"openai","platform":"openai","type":"api_key","models":["gpt-4o"],"api_key":"sk-..."}' \
  http://127.0.0.1:7777/admin/channels

# 4. 建一个入站 key（明文仅此一次返回）
curl -X POST -H "Authorization: Bearer $ADMIN_KEY" -H "Content-Type: application/json" \
  -d '{"name":"app1","group":"default"}' http://127.0.0.1:7777/admin/api-keys

# 5. 发请求（用上一步返回的入站 key）
curl -X POST -H "Authorization: Bearer sk-ph-..." -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}' \
  http://127.0.0.1:7777/v1/chat/completions
```

开发期前端：`cd web && npm install && npm run dev`（端口 **8888**，自动代理 `/v1`·`/admin`·`/v0`·`/healthz` 到 **7777**）。

## 配置

完整键与默认值见 [`config.example.yaml`](./config.example.yaml)。优先级：内置默认 → `config.yaml` → 环境变量
（前缀 `PROXY_HUB_`，嵌套以下划线连接，如 `PROXY_HUB_SERVER_ADDR`）。非密钥键支持热重载。

要点：`selector.strategy`（`round_robin`|`fill_first`）、`health.enabled`（主动探测，默认关）、
`relay.enable_cross_dialect`（默认关）、`stats.*`（采集批量/flush/溢出兜底）、`retention_days`（原始日志保留）。

## 安全模型

- **上游凭证**只存 `data/auths/<channel_id>.json`（0600）：**绝不**入 SQLite、绝不记日志、绝不进用量事件。
- **入站 key** 只存 `sha256`；明文仅创建时返回一次；比较用常量时间。
- **管理面/MCP 鉴权**：`/admin/*` 与 `/v0/mcp/*` 走 admin key（bearer）。SPA 输入 admin key 存
  localStorage 并随请求注入。
- **CSRF/origin**：对这些端点的**改状态请求**（非 GET）做同源校验——浏览器跨域写请求被拒；非浏览器客户端
  （无 Origin）由 bearer 保护。额外放行域见 `server.allowed_origins`。
- **不持久化**任何请求/响应体；`error_message` 截断。
- **MCP 投影**：整文件保留 + 原子写 + 首写 `.bak` + 按路径加锁，绝不破坏客户端配置无关内容。

## 部署

单容器、单 `/data` 卷、单端口。见 [`Dockerfile`](./Dockerfile) 与 [`docker-compose.yml`](./docker-compose.yml)。
发布二进制（linux/macOS/Windows × amd64/arm64）由 [`.goreleaser.yaml`](./.goreleaser.yaml) 产出。
`data/`：`proxy-hub.db`（WAL）· `auths/*.json`（0600）· `config.yaml`。

## MCP 共享

在控制台 MCP 页（或 `/v0/mcp/*` API）定义 MCP 服务器、按客户端开关、登记目标文件（绝对路径）、一键同步。
也可 `proxy-hub mcp sync`（cron 友好）。详见 [docs/mcp.md](./docs/mcp.md)。

## 范围：为何不做订阅/OAuth 渠道

proxy-hub 专注 **API-Key 计费模型**：渠道是「API-Key」或「OpenAI/Claude 兼容的 upstream」。**不**实现
OAuth/PKCE、token 刷新循环、订阅席位等——这类「无过期凭证」之外的复杂度被刻意砍掉，以保持单二进制、
零外部依赖、可审计的密钥处理。需要订阅渠道请用其它项目。

## 开发

```bash
gofmt -l cmd internal          # 无输出
go vet ./...
CGO_ENABLED=0 go build ./...
go test ./...                  # 含一致性/保留性套件
cd web && npm run build        # 产出 web/dist（go:embed）
```
数据库查询经 `sqlc` 生成（改 `internal/store/queries/*.sql` 后 `sqlc generate` 再提交 `internal/store/dbgen`）。
