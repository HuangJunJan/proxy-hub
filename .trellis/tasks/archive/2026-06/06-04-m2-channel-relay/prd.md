# M2 —— 渠道管理 + API-Key 中转 + 模型映射

> `06-04-proxy-hub-mvp` 的子任务。共享设计见父 `design.md`。这是最大的里程碑，也是产品核心
> （能力 C1）。
>
> 语言约定：本项目所有规划文档与代码注释一律使用中文（见 `AGENTS.md`）。

## 目标

管理上游渠道（API-key + 自定义 upstream），并经统一的 OpenAI/Claude 兼容端点把客户端请求中转过去，
带模型映射、负载均衡、失败转移与按 (渠道, 模型) 的冷却。同方言路由是核心；一个类型化的
OpenAI⇄Claude 转换器补足跨方言支持。

## 范围

**范围内**
- `channels` + `abilities` + `api_keys` 表 + DAO；**增量** `RouteIndex`（内存
  `model → []ChannelRuntime`），渠道 upsert/delete 时重建（绝不 TRUNCATE）。
- 渠道 CRUD 管理 API + "测试渠道"探测（记录响应时间）。
- `credstore`：文件级凭证 `data/auths/<id>.json`（`api_key` / `upstream` 形状），启动时 + 经
  `fsnotify` 变更时加载。
- 入站 API-Key 鉴权中间件（OpenAI `Authorization: Bearer`、Claude `x-api-key`），经内存缓存对
  `api_keys.hash` 校验。
- 统一端点：`/v1/chat/completions`、`/v1/messages`、`/v1/responses`、`/v1/models`。
- `Adaptor` 接口（`ConvertOpenAIRequest/ConvertClaudeRequest/DoRequest/DoResponse`）+ **同方言
  透传**；`/v1/responses` 由 OpenAI/兼容 upstream 服务。
- 选择器：最高优先级档 → 加权随机，**会话亲和**（session id 取自 header/metadata/内容哈希），
  `isBlockedForModel` 过滤；**跨渠道重试**循环。
- `MarkResult` 按 (渠道, 模型) 的 HTTP 状态冷却状态机（429 指数 / 401·403 长 / 404 很长 /
  5xx 短 / 成功重置），写 `channel_model_health`。
- 模型映射：精确 → 尾部 `*` 最长匹配 → 透传；`prefix` 命名空间；**固定解析顺序**以避免别名/上游
  命名空间碰撞；保留客户端面模型名；保存时校验映射冲突。每渠道出口代理。
- **最后**：单一类型化 **OpenAI-chat ⇄ Claude-messages** 转换器（tools/tool_calls、流式 SSE
  chunk 形状）+ 一致性测试套件，置于特性开关之后。

**范围外**
- OAuth/订阅渠道（已砍）。OpenAI/Claude 以外的任何方言。NxN 矩阵。统计持久化/仪表盘（M3）——
  M2 只*发出* `UsageEvent` + 调用 `MarkResult`。

## 需求（来自父 `prd.md`）

FR-C1-1 … FR-C1-7。发出供 M3 消费的 `UsageEvent`（FR-C2-1），并写 `channel_model_health`
（FR-C2-6）。

## 依赖

**依赖 M1**（store、config、API 骨架、`fileio`/credstore 底座）。提供供 M3 消费的实时
`UsageEvent` 流 + `MarkResult` 写入（M3 的 schema/采集器可并行用合成事件构建）。

## 交付清单

- [ ] `internal/channel/{model,dao,runtime,routeindex}` + 增量 RouteIndex。
- [ ] `internal/credstore`（auths/*.json，fsnotify 重载）。
- [ ] 渠道 CRUD 管理 API + channel-test；入站 API-Key 鉴权中间件 + 缓存。
- [ ] `internal/adaptor/{adaptor,openai,claude}` + 同方言透传；统一端点。
- [ ] `internal/selector/{selector,roundrobin,weighted,affinity}`；跨渠道重试中转。
- [ ] `internal/relay/markresult.go` 按 (渠道,模型) 冷却 + `channel_model_health`。
- [ ] 模型映射（精确/通配/prefix + 解析顺序 + 保存时校验）。
- [ ] `internal/adaptor/convert` OpenAI⇄Claude 类型化转换器 + 一致性套件（带开关）。

## 验收标准

- [ ] 创建 OpenAI + Claude `api_key` 渠道及一个 `upstream` 渠道；测试通过；真实流量经统一端点流转。
- [ ] 模型重命名 + 尾部 `*` 通配 + `prefix` 路由正确；客户端面模型名正是后续统计所见。
- [ ] 可重试上游错误（429/5xx）时中转失败转移到另一渠道；坏的 (渠道, 模型) 进入冷却并被跳过直至恢复。
- [ ] 多轮会话经亲和粘在一个渠道。
- [ ] OpenAI 格式请求由 Claude upstream 服务、Claude 格式请求由 OpenAI upstream 服务，通过一致性
      套件（流式 + 工具）。若套件未过，则转换器开关关闭、同方言路由仍可发布。
- [ ] `gofmt`/`go vet`/`go test ./...` 干净；`trellis-check` 干净。

## 备注

子任务 `design.md`（adaptor 契约、转换器映射表、冷却常量）与 `implement.md`（有序步骤、以带开关的
转换器收尾）在子任务启动时撰写。
