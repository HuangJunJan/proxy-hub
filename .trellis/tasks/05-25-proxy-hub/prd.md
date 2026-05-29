# Proxy Hub - 多渠道 OpenAI 兼容代理网关

## Goal

构建一个自托管的多渠道 AI 代理网关：把多个上游（OpenAI 兼容服务商：baseUrl + key；以及 ChatGPT OAuth）聚合成单一的 OpenAI 兼容 HTTP API，并提供渠道维度统计与实时请求监控面板。

项目定位为**轻量桥梁**：不限流、不改写业务语义、不做提示词加工；但允许"模型别名映射"作为路由能力（下游请求模型名 → 上游真实模型名）。

首期个人使用 Windows exe；架构上预留团队 / 开源演进空间。

## Background

参考开源项目 CLIProxyAPI 与 sub2api：均为"上游凭证池 → 协议适配 → 统一下游 API"。本项目沿用该思路，但更强调可视化运维（渠道健康 + 实时监控）与零运维部署（单 exe + YAML + SQLite）。

## Scope

### In Scope (v1)

1. **上游渠道类型**（YAML 中按源分组配置，不混合）
   - `openai-api`：OpenAI 兼容（baseUrl + key），覆盖 OpenAI 官方、OpenRouter、DeepSeek、月之暗面、自建网关等。
   - `chatgpt-oauth`：ChatGPT OAuth（复用 ChatGPT 订阅/Codex 凭证）。
2. **下游协议**：仅 OpenAI 兼容（`POST /v1/chat/completions`、`GET /v1/models`），流式 / 非流式皆支持。
3. **模型路由与别名**：`openai-api` 渠道默认透传全部模型：下游请求 `model=X` 时，若没有命中显式别名映射，则把 `X` 原样传给所有启用的 OpenAI 兼容上游候选。`models[]` 仅用于维护需要枚举到 `/v1/models` 或需要改名的显式映射。`chatgpt-oauth` 渠道仍需显式维护模型列表，因为它没有标准 OpenAI models 接口。除模型名映射外不做任何业务改写。
4. **配置形态**：
   - 上游渠道、下游 API Key、管理员账号、监听端口等"配置类"数据存 YAML 文件，方便人工编辑与迁移。
   - 运行时数据（请求日志、统计聚合、渠道健康状态）存 SQLite。
5. **调度策略**：`openai-api` 渠道按 priority 升序排队，同优先级 round-robin；`chatgpt-oauth` 渠道独立成池，池内 round-robin。两类渠道的选择由"下游模型名 → 命中的渠道集合"决定，不预设跨池降级。失败自动切下一个，最多重试 N 次（默认 2）。
6. **下游访问控制**：Proxy Hub 自签 API Key（Bearer Token）鉴权；支持多 Key、可禁用、可备注。
7. **统计 - 渠道维度**：每渠道累计请求数、成功/失败数、平均延迟、Token 用量（prompt / completion / total）、最近错误。
8. **实时请求监控**：最近请求实时流（时间、下游 Key、上游渠道、模型、状态、耗时、Token），可按渠道 / 状态过滤。
9. **Web 控制台**：渠道管理、API Key 管理、统计仪表盘、实时监控、设置（主题 / 语言）。
10. **主题与多语言**：明 / 暗 / 跟随系统；中 / 英 i18n。
11. **部署形态**：单 Windows exe，前端静态资源嵌入；YAML 配置与 SQLite 文件同目录。

### Out of Scope (v1)

- 其他下游协议（Anthropic Messages、Gemini 原生）。
- 提示词改写 / system prompt 注入 / 内容过滤（项目只做模型名映射 + 转发）。
- 多用户 / RBAC。
- 配额 / 限流的精细策略（v1 仅"启用/禁用"+"失败转移"）。
- Postgres / 集群部署（数据层做抽象但不实现）。
- 凭证加密存储（明文 YAML，依赖文件系统权限保护）。
- 计费金额估算（仅记录 Token 用量）。
- Linux/macOS 打包（v1 仅 Windows exe，但 Go 跨平台天然可构建）。

## Functional Requirements

### FR-1 配置文件（YAML）

- FR-1.1 YAML 文件路径可通过命令行 `--config` 或环境变量指定，默认与 exe 同目录的 `config.yaml`。
- FR-1.2 文件包含：监听端口、管理员账号、下游 API Keys、上游渠道列表（类型 A / B 区分）、保留策略等运行参数。
- FR-1.3 上游渠道凭证**明文存储**于 YAML；不在数据库中存储凭证。
- FR-1.4 YAML 是配置的**唯一真相源**。控制台对"上游渠道 / 下游 API Key / 管理员账号"的增删改直接原子写回 YAML 并热加载；不在 SQLite 中冗余存储配置。
- FR-1.5 控制台写 YAML 不要求保留注释 / 注释/顺序可被改写。
- FR-1.6 仓库内附带 `config.example.yaml` 作为带注释的参考模板；运行时如 YAML 不存在则进入首次启动向导（FR-8.5），由向导写入最小可用 YAML。
- FR-1.7 配置写入需文件锁 + 临时文件 + rename 原子替换，避免并发写损坏。
- FR-1.8 YAML 序列化使用 `omitempty`：默认值 / 关闭项 / 空字段不写入文件，保持配置极简、可读。

### FR-2 上游渠道

- FR-2.1 YAML 顶层按源分两个 section：`openai-api:` 与 `chatgpt-oauth:`，各自是渠道数组。
- FR-2.2 渠道使用 `name` 作为唯一标识（无独立 `id`）；同一 section 内 `name` 必须唯一；改名等同于"删旧建新"，历史日志保留旧 name。
- FR-2.3 `openai-api` 渠道字段：`name`、`base-url`、`priority`（数字越小越先用，缺省 100）、`api-key-entries[]`（凭证池）、`models[]?`（可选显式映射 / 枚举项）、`disabled?`（缺省启用）、`timeout-sec?`、`notes?`。
- FR-2.4 `api-key-entries[]` 每项：`api-key`（必填）、`proxy-url?`（可选，按 key 覆盖代理）。同一渠道内多 key 自动 round-robin 形成凭证池。
- FR-2.5 `chatgpt-oauth` 渠道字段：`name`、`oauth`（含 `access-token` / `refresh-token` / `expires-at`）、`models[]`、`disabled?`、`timeout-sec?`、`notes?`。
- FR-2.6 `models[]` 每项：`name`（上游真实模型名）、`alias?`（下游可见名，缺省 = `name`）。对 `openai-api`，空 `models[]` 表示不枚举显式模型但仍透传全部模型；单条 `{name}` 仅表示把该模型加入 `/v1/models` 的显式枚举。**重复同一 `name` 配不同 `alias`** 为该上游模型添加多个下游可见名；**多个不同 `name` 配同一 `alias`** 在该渠道内形成 alias 内的上游模型池（round-robin）。
- FR-2.7 控制台"新增/编辑 openai-api 渠道"页面提供**"拉取模型列表"按钮**：调用上游 `GET {base-url}/v1/models`，把返回模型渲染为可选候选清单。用户可勾选候选写入 `models[]`，alias 可选；不勾选任何模型也保存渠道并启用默认透传。失败时显示错误并允许手动填写。
- FR-2.8 `chatgpt-oauth` 渠道无标准 models 接口，模型列表由用户在控制台手动维护（提供常用预设建议，如 `gpt-5-codex` / `gpt-4.1`）。
- FR-2.9 支持手动触发"健康检查"（轻量上游请求验证可用）。
- FR-2.10 失败码（401/403/429/5xx/timeout）触发临时熔断（冷却时长可配，默认 60s）。

### FR-3 下游 OpenAI 兼容 API

- FR-3.1 `POST /v1/chat/completions` 支持 `stream: true/false`，逐 chunk 透传。
- FR-3.2 `GET /v1/models` 返回所有启用渠道显式配置的 `models[].alias`（若未设则 `models[].name`）去重并集。OpenAI 兼容渠道的默认透传模型无法预先枚举，因此不会自动出现在该响应里。
- FR-3.3 Bearer Token 鉴权；无效 / 禁用 Key 返回 OpenAI 标准错误体（`401 invalid_api_key`）。
- FR-3.4 上游全部失败时返回最后一次错误的 OpenAI 兼容标准化形式，错误信息脱敏。

### FR-4 调度与故障转移

- FR-4.1 收到下游请求后解析 `model` 字段，先把它当作"下游可见名"在所有启用渠道的 `models[].alias`（未设则 `models[].name`）中查找显式命中集合。
- FR-4.2 若显式命中集合为空，则所有启用的 `openai-api` 渠道都作为默认透传候选，且上游模型名等于下游请求的 `model`。若既无显式命中也无启用的 `openai-api` 渠道，则返回 `404 model_not_found` 的 OpenAI 兼容错误。
- FR-4.3 命中集合内：`openai-api` 渠道按 priority 升序排队（同优先级 round-robin）；`chatgpt-oauth` 渠道独立成池 round-robin。显式别名命中时使用该 alias 对应的 `name`（上游真实模型名）调用上游；默认透传时使用下游请求模型名调用上游。
- FR-4.4 渠道内多个 `api-key-entries` 自动 round-robin。
- FR-4.5 单次失败按命中集合继续尝试下一渠道，最多 N 次（默认 2，可配）。
- FR-4.6 熔断中的渠道跳过；命中集合全部熔断 / 失败则返回最后一次错误（标准化为 OpenAI 错误体）。

### FR-5 下游 API Key

- FR-5.1 控制台创建（生成 `sk-proxy-hub-...` 风格 token，写入 YAML 的 `api-keys[]`）。
- FR-5.2 `api-keys[]` 每项：`token`（必填）、`name?`（可选别名，便于监控页区分）、`notes?`、`disabled?`。token 在 YAML 中明文存储。
- FR-5.3 控制台管理员可在 API Keys 页面查看 / 复制完整 token；列表同时保留掩码 token 便于紧凑展示与日志关联。
- FR-5.4 支持编辑名称/备注、启用/禁用、删除、复制 token、查看使用情况（请求数、Token 数、最近使用时间）。

### FR-6 统计 - 渠道维度（SQLite）

- FR-6.1 每渠道实时维护：累计请求数、成功/失败数、平均延迟、累计 prompt/completion/total Token。
- FR-6.2 按时间窗（24h / 7d / 30d）查看请求数与 Token 量趋势。
- FR-6.3 失败原因 Top N（按 HTTP 状态码 / 错误类别分组）。

### FR-7 实时请求监控（SQLite + 推送）

- FR-7.1 每条请求落一条记录：时间、下游 Key、上游渠道、模型、状态码、耗时、prompt/completion/total Token、错误摘要。
- FR-7.2 控制台"实时监控"页通过 **SSE** (`GET /api/admin/requests/stream`) 推送新增请求；非实时支持按渠道 / 状态 / 时间过滤查询。
- FR-7.3 请求日志保留策略可配（默认 7 天），超期自动清理。
- FR-7.4 请求/响应正文落库策略：默认仅失败请求记录正文（请求体 + 上游错误响应）；YAML 可配置升级为"全量记录"（成功请求也存）。配置项示例：`requestLog.bodyMode: failed_only | always | none`，默认 `failed_only`。

### FR-8 控制台 UX

- FR-8.1 React (最新版) + Vite + Shadcn UI + Axios。
- FR-8.2 主题：明 / 暗 / 跟随系统三档，localStorage 持久化。
- FR-8.3 i18n：中 / 英；语言切换即时生效，localStorage 持久化。
- FR-8.4 控制台单用户登录（账号 + 密码，YAML 中存账号 + 密码哈希）。
- FR-8.5 **首次启动向导**：若 YAML 不存在或缺少 admin / apiKeys，进入 setup 模式，所有页面重定向到 `/setup` 向导：设置管理员账号密码 → 生成首个 API Key（仅此刻明文展示一次） → 写入 YAML → 退出 setup 模式进入正常控制台。若用户手写 YAML 已含 admin + ≥1 个 apiKey，则跳过向导直接进入登录页。
- FR-8.6 Windows exe 启动时自动尝试打开默认浏览器到 `http://localhost:<port>`（可通过 CLI flag 关闭）。

### FR-9 跨域、限流与可观测（v1 范围）

- FR-9.1 CORS：默认关闭；YAML `cors.allowedOrigins: []` 可配置允许的来源。
- FR-9.2 限流：v1 不实现 Per-Key QPS 限流；中间件链预留挂载点。
- FR-9.3 指标导出：v1 不提供 Prometheus / CSV 导出；预留 `/metrics` 路径以便后续扩展。

### FR-10 ChatGPT OAuth 凭证管理

- FR-10.1 调用上游前检测 `accessToken` 是否在 `expiresAt - 60s` 内过期；过期则用 `refreshToken` 同步刷新一次。
- FR-10.2 刷新成功后更新 YAML 中该渠道的 `oauth` 段（accessToken / expiresAt / 必要时 refreshToken），经过原子写流程。
- FR-10.3 刷新失败标记渠道失败一次并进入熔断；记录详细错误，不在 API 响应中暴露细节。
- FR-10.4 不做后台轮询刷新，仅按需刷新，降低无意义调用。

## Non-Functional Requirements

- NFR-1 单 exe + YAML + SQLite，零外部依赖。
- NFR-2 在 4C8G Windows 上稳定支撑 ≥ 20 QPS 流式请求转发。
- NFR-3 P50 转发额外开销（不含上游）< 30ms。
- NFR-4 上游凭证 / 下游 API Key 明文存于 YAML；控制台密码 bcrypt/argon2 哈希。
- NFR-5 控制台与 API 文案全 i18n 化，无硬编码。
- NFR-6 数据层抽象到 repository 接口，便于后续切换 Postgres。
- NFR-7 关键路径结构化 JSON 日志。

## Acceptance Criteria

- [ ] AC-1 YAML 中 `openai-api` section 配置 ≥ 2 个渠道（不同优先级），手动健康检查均显示可用。
- [ ] AC-2 `curl` 携带 Proxy Hub API Key 调 `POST /v1/chat/completions`，非流式 / 流式均返回 OpenAI 兼容响应。
- [ ] AC-3 高优渠道禁用后，再次请求自动落到次优渠道；日志中可见上游切换。
- [ ] AC-4 高优渠道 401 失败，自动重试切换到次优并成功返回；失败渠道失败计数 +1，进入熔断状态。
- [ ] AC-5 仪表盘渠道卡片的请求数 / 成功率 / Token 总数与实际一致。
- [ ] AC-6 实时监控页 1 秒内显示新请求，可按渠道过滤。
- [ ] AC-7 控制台明暗模式切换并持久化；刷新后保持。
- [ ] AC-8 控制台中英文切换，所有可见文案均有翻译。
- [ ] AC-9 在控制台修改渠道后，YAML 文件被原子写回，重启后生效。
- [ ] AC-10 重启进程后渠道 / Key / 历史统计 / 请求日志均保持不丢失。
- [ ] AC-11 `proxy-hub.exe` 双击启动后自动打开浏览器到 `http://localhost:<port>` 加载控制台。
- [ ] AC-12 配置 1 个 `chatgpt-oauth` 渠道后，对应模型的请求能成功路由。
- [ ] AC-13 控制台新增 openai-api 渠道时点击"拉取模型列表"按钮，能获取上游模型并供勾选；不勾选模型仍可保存并默认透传，勾选后 alias 可选。
- [ ] AC-14 模型别名生效：在 deepseek 渠道为 `deepseek-chat` 加一条 `alias: gpt-5.4` 后，下游 `model=gpt-5.4` 的请求被路由到该渠道并以 `deepseek-chat` 调上游。
- [ ] AC-15 `GET /v1/models` 返回所有启用渠道显式配置的 alias（未设则 name）去重并集；默认透传能力不自动枚举到该列表。
- [ ] AC-16 首次启动（无 YAML 或缺少 admin/apiKeys）自动进入 setup 向导；完成后 YAML 被写入且后续启动直接进入登录页。
- [ ] AC-17 同一 openai-api 渠道配置 ≥ 2 个 `api-key-entries`，多次请求观察到 key 轮询（监控页可查）。
- [ ] AC-18 YAML 中省略所有默认值（如 `disabled` / `priority: 100`）后，控制台改一项 → 写回 YAML 仍只包含非默认值，文件保持精简。

## Open Questions

所有 v1 关键决策已敲定。后续若 design.md / implement.md 阶段又冒出新问题，再回此处补记。

- OQ-A ~~配置写入路径~~ **已定**：方案 (a) — YAML 是唯一真相源，控制台原子写回 + 热加载，不保留注释，提供 `config.example.yaml` 作为带注释模板。
- OQ-B ~~ChatGPT OAuth 路由策略~~ **已定**：按"下游模型名 → 命中渠道集合"路由；YAML 按 `openai-api:` / `chatgpt-oauth:` 分组；模型支持 `aliases` 别名映射；控制台提供"拉取模型列表"按钮。
- OQ-C ~~`GET /v1/models` 返回什么~~ **已定**：所有启用渠道显式配置的 `models[].alias`（未设则 `name`）去重并集；OpenAI 兼容渠道的默认透传模型不自动枚举。
- OQ-D ~~请求/响应正文是否落库~~ **已定**：默认 `failed_only`（仅失败请求记录正文），YAML 可配置 `always`（全量）或 `none`（永不记录）。
- OQ-E ~~每 Key QPS 限流~~ **已定**：v1 不做，中间件链预留挂载点。
- OQ-F ~~CORS~~ **已定**：默认关闭，YAML `cors.allowedOrigins: []` 可配。
- OQ-G ~~OAuth 自动刷新~~ **已定**：必做；按需同步刷新（`expiresAt - 60s` 触发），不做后台轮询。
- OQ-H ~~Prometheus / CSV 导出~~ **已定**：v1 不做。
- OQ-I ~~首次启动向导~~ **已定**：方案 (b) — 缺 admin/apiKeys 时进入 `/setup` 引导；完成后写 YAML 退出。
- OQ-J ~~实时监控推送通道~~ **已定**：SSE (`GET /api/admin/requests/stream`)。

## Notes

- 复杂任务：后续需补 `design.md`（模块划分、YAML schema、SQLite schema、调度算法、流式转发实现要点、热加载策略）与 `implement.md`（执行清单 + 验证命令）后再 `task.py start`。
- 数据层 / 上游 adapter / 下游协议 handler 做接口抽象，为 v2（更多协议、Postgres、多用户）留口。
