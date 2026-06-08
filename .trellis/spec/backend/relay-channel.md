# 渠道与中转规范（Relay / Channel Guidelines）

> proxy-hub M2 确立的渠道管理、模型路由、上游适配、冷却与用量契约。M3（统计/监控）消费此处的
> `UsageEvent` 与 `channel_model_health`；M4/M5 在此之上扩展。
> 语言：中文注释；标识符/表名/路径/技术名英文。

---

## 分层与依赖方向（勿成环）

```
api ─→ relay ─→ {channel(RouteIndex), selector, adaptor(接口), credstore}
adaptor 实现(openai/claude/convert) ──init Register──→ adaptor 接口（relay 不直接 import 实现）
channel ─→ {store, store/dbgen, credstore, apikey}
relay ─→ channel（healthstore 桥接 dbgen；relay 自身无 dbgen 依赖）
```

- **relay 不直接 import adaptor 实现**（openai/claude）：靠 `adaptor.Register` + `main` 的 `_ import` 注册，避免环。
- **channel 不 import relay**：健康状态 dbgen↔领域 的桥放在 `channel.DAO` + 薄 `relay.HealthStore`（relay→channel 单向）。
- selector / adaptor 接口层 **dbgen 无关**，只消费 `channel.ChannelRuntime`（不含 key——key 在 credstore）。

---

## Adaptor 契约

```go
type Adaptor interface {
    Platform() channel.Platform
    BuildRequest(ctx, in *RelayInput, rt *channel.ChannelRuntime, cred credstore.Cred) (*http.Request, error)
    HandleResponse(ctx, resp *http.Response, w http.ResponseWriter, out *UsageResult) error
}
```

- **同方言透传**：用 `gjson`/`sjson` 取/改 `model`，其余字段原样透传（不做类型化往返）；按 `IsStream` 走 SSE 逐行 flush 或一次性 buffer。
- usage 解析填 `UsageResult`（M2 只填 token，**不算成本**——成本由 M3 按定价计算）。
- 出口代理由 relay 的 per-proxy `*http.Client` 处理（adaptor 不关心）；client `Timeout=0`，生命周期由调用方 ctx 控制（流式可长连）。
- 注册：实现包 `init()` 调 `adaptor.Register(自身)`；`main` 用 `_ "…/adaptor/openai"` 触发。

## 跨方言转换（特性开关后；design §10/§14）

- `internal/adaptor/convert`：类型化 OpenAI-chat ⇄ Claude-messages（**请求 + 非流式响应已实现**；流式 `sse.go` 与 relay 接线**按设计推迟**）。
- 开关 `relay.enable_cross_dialect`（**默认 false**；一致性套件 `convert_test.go` 全绿才考虑置 true）。
- **relay 守门（off 路径已实现并有意义）**：路由解析后，开关关时**预过滤候选**只留同方言；候选全为跨方言 → 干净拒绝 `501 cross_dialect_disabled`（**绝不把不匹配请求体透传上游**）。
- `isCrossDialect(format, platform)` **仅对已知 `openai`/`anthropic` 平台判定**；未知/兼容/测试平台一律视为同方言放行（否则会误拦 fake 平台测试）。

---

## 路由：RouteIndex 与模型映射

- 内存 `RouteIndex`：`map[group]map[alias][]*ChannelRuntime` + 每渠道通配有序表；**热路径不碰 DB**。渠道 upsert/delete **增量**重建该渠道项（写锁），**绝不 TRUNCATE**。
- **prefix 烘焙进客户端面 alias**：`BuildAbilities` 中 `alias = prefix + 原名`，`upstream_model` 不带前缀。故请求期**无需再剥 prefix**（修了父设计的命名空间碰撞）。
- `Candidates(group, model)` 解析顺序：**精确 alias → 最长 `*` 通配 → 原样上游名**。
- 请求期唯一的「再处理」：首次未命中时剥离 `[1M]` 等长上下文后缀（`StripContextSuffix`）兜底再查一次。
- **客户端面原名（含 prefix、含 `[1M]`）写入 `UsageEvent.RequestedModel`**；解析出的 `UpstreamModel` 另存。这是 M3 统计所见的模型名。

---

## 冷却状态机（per 渠道×模型）—— `relay.HealthMirror`

`Classify(status, connErr) → Outcome`，`computeHealth` 算冷却：

| 结果 | 冷却 | 换渠道重试? |
|---|---|---|
| 2xx 成功 | 重置（连续失败清零、清 cooldown、置健康、记 last_success） | — |
| 429 | 指数退避 base 1s ×2 **cap 5min** | 是 |
| 401 / 403 | 30min（凭证问题） | 否 |
| 404 / 不支持 | 12h | 否 |
| 5xx / 连接错误 | 1min | 是 |
| 其它 4xx | **不改健康**（客户端错误） | 否 |

- `HealthMirror` 内存 map + 写时**同步落库** `channel_model_health` + 更新镜像；`persist` 可注入 nil（测试）。
- **selector 读镜像 `IsBlocked`（cooldown_until > now）过滤，热路径不查 DB**；启动 `Load` 从 DB 装配镜像。

## selector 选择顺序

`Pick(candidates, sessionID, isBlocked)`：①冷却过滤 → ②会话亲和（上次成功渠道仍在可用集则复用）→ ③最高优先级档 → ④档内**加权随机**（weight<=0 视为 1）。`RecordAffinity` **仅成功后**由 relay 调用（失败渠道不粘附）；亲和 TTL 默认 10min，仅内存、关机丢弃。sessionID 取自 `X-Session-Id` 头。

### 策略开关（M5）

`selector.NewWithStrategy(cfg.Selector.Strategy)`：`round_robin`（默认，第④步加权随机）| `fill_first`（第④步取**首选**：weight 最大、平手 channelID 最小 ⇒ 填满首选渠道，冷却/溢出才用下一个）。①②③ 不变。未知策略回退 round_robin。

## 主动健康探测（M5，可选默认关）—— internal/health

`health.Prober`（`health.enabled=true` 才由 main 起独立 goroutine）：按 `health.interval` 对**启用渠道 × 其模型**发 max_tokens=1 最小探针（复用 adaptor.BuildRequest，独立 http client，绝不阻塞中转），写 `health_check_logs`（迁移 0005）+ 调 `HealthMirror.Mark`（与被动路径同源的成功重置/失败冷却）。`doProbe` 可注入（测试）。仪表盘 `GET /admin/health/checks?limit=` 读最近记录。停机随 ctx 取消。

## relay 重试循环

`attempt <= maxRetries`（默认 2）；每轮 `pickExcluding`（跳过已 tried + IsBlocked）；可重试失败 → `Mark` 冷却 + 丢弃响应体 + 换渠道；终态（成功/不可重试/重试用尽）回写客户端 + 成功时 `RecordAffinity`。**函数返回必发且仅发一条 `UsageEvent`**（defer）。错误体按入站方言成形：OpenAI `{error:{message,type,code}}` vs Claude `{type:"error",error:{type,message}}`。

---

## UsageEvent 与安全不变量（每个里程碑必守）

- `UsageEvent` **只含** token 拆分 / 时延 / TTFT / 状态 / `requested_model`·`upstream_model` / id 维度；**绝不含密钥、绝不含请求/响应体**。经有界 channel（默认 16384）**非阻塞** `Emit`（满则丢弃 + `Dropped` 计数暴露，**非静默**）。M2 挂 `DrainAndDiscard` 占位消费者；M3 采集器替换。
- **上游凭证**：仅 `data/auths/<channel_id>.json`（0600）；**绝不**入任何表 / 日志 / UsageEvent。`channels` 表无 key 列；对外 DTO 用 `has_credential` 布尔表达「是否已配凭证」，不回显密钥。
- **入站 key**：仅存 `sha256` hex（`api_keys.hash`）；明文仅创建时返回一次；比较用 `subtle.ConstantTimeCompare`；内存缓存正+负缓存、**fail-closed**，变更即 `Invalidate`。
- **错误信息**：`error_message`/连接错误截断（relay 256B、admin 200B）；连接错误串不含 key（key 在请求头不在 URL）。
- **请求/响应体绝不落盘**；透传不缓存。

---

## 测试要点（M2 基线）

- `channel`：映射解析顺序（精确→最长通配→透传）、prefix 烘焙、ability 增量重建、保存冲突校验。
- `selector`：冷却过滤、加权分布、会话亲和粘附。
- `relay`：成功 / 故障转移 / `model_not_found` / `[1M]` 兜底 / **跨方言关拒绝（501）**；冷却状态机各分支、指数退避封顶。
- `credstore`：往返 + 0600 + 坏 JSON 容错 + 删除（含 Windows 共享冲突重试，见 `quality-guidelines.md`）。
- `convert`：OpenAI⇄Claude 请求/响应一致性套件（**守门开关**：套件未过 → `enable_cross_dialect=false`）。
