# M2 —— 渠道管理 + API-Key 中转 + 模型映射：执行计划（Implement）

> `06-04-m2-channel-relay` 的有序执行清单与校验。依据本子任务 `prd.md` + `design.md` + 父 `design.md`。
> 语言约定：代码注释一律中文（见 `AGENTS.md`）。继承 M1 约定（见 `.trellis/spec/backend/`）。

## 前置

- 工具链：Go 1.25（本机 go1.25.5）。**sqlc**（开发期）：`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
  （或下载发布二进制）。生成代码提交入库 ⇒ `go build`/Docker/CI **不**需要 sqlc。
- 分支：在 `main` 上开 `task/m2-channel-relay`（`task.py set-branch` 记录）。
- 依赖 M1 已合入（store/config/api 骨架）。

## 有序步骤（保持每步可独立编译 + 测试）

1. **sqlc 脚手架 + 迁移 0002**
   - `sqlc.yaml`（engine sqlite；schema=`internal/store/migrations`；queries=`internal/store/queries`；out=`internal/store/dbgen`）。
   - `internal/store/migrations/0002_channels.sql`：`channels`/`abilities`/`api_keys`/`channel_model_health` + 索引（见 design §2）。
   - `internal/store/queries/{channels,abilities,api_keys,health}.sql`：CRUD + 列表 + upsert 查询。
   - `sqlc generate` → 提交 `internal/store/dbgen`。校验：`go build ./internal/store/...`。
2. **channel 领域 + dao + manager**
   - `internal/channel/model.go`：`Channel`/`Ability`/`ChannelRuntime` + `model_mapping`/`models` 的 JSON 编解码 + 映射解析纯函数（精确/最长通配/透传）。
   - `internal/channel/dao.go`：包裹 `dbgen`，渠道/ability/api_key 持久化；**渠道保存事务**（写 channels + 同事务增量重建该渠道 abilities，绝不 TRUNCATE）。
   - `internal/channel/manager.go`：装配 dao + credstore + routeindex；`SaveChannel`/`DeleteChannel` 编排（DB + 凭证文件 + RouteIndex 增量更新 + 映射冲突校验）。
   - 单测：映射解析顺序、ability 增量重建、保存时冲突校验。
3. **RouteIndex**
   - `internal/channel/routeindex.go`：`map[group]map[alias][]*ChannelRuntime` + 通配有序表；`Rebuild`(启动全量)、`UpsertChannel`/`RemoveChannel`(增量，写锁)、`Candidates(group, model)`(解析顺序，返回候选 + upstream)。
   - 单测：增量更新不影响其它渠道；最长通配匹配；prefix 剥离。
4. **credstore**
   - `internal/credstore/store.go`：启动全量加载 `data/auths/*.json` + `fsnotify` 增量重载；`Get/Put/Delete`（原子 temp+rename，0600）；内存 map + RWMutex。
   - 单测：put→get 往返、文件 0600、删除、坏 JSON 容错（跳过 + 告警）。
5. **入站鉴权 + api_keys**
   - 升级 `internal/api/middleware/auth.go`：入站 key 中间件（`Authorization: Bearer` / `x-api-key` → sha256 → 内存缓存 + 负缓存 → 注入 api_key_id/group）；比较用 `crypto/subtle.ConstantTimeCompare`（**同时修 M1 admin auth 的非常量时间比较**）。
   - `internal/api/apikey_handlers.go`：`/admin/api-keys` CRUD（创建返回明文一次；DB 存 sha256）。
   - 单测：哈希校验、负缓存、禁用 key 拒绝。
6. **adaptor 同方言透传 + 统一端点**（先打通同方言端到端）
   - `internal/adaptor/{adaptor.go, openai/openai.go, claude/claude.go}`：`BuildRequest`/`HandleResponse`；gjson 取/改 model；stream/非 stream SSE 透传；usage 解析填 `UsageEvent`（不算成本）。
   - `internal/api/relay_handlers.go`：`/v1/chat/completions`、`/v1/messages`、`/v1/responses`、`/v1/models`。
   - 出口代理：per 渠道 `*http.Client`（proxy_url + 超时 + 连接复用）。
   - 校验：建一个 OpenAI api_key 渠道，经 `/v1/chat/completions` 透传到真实/mock upstream 成功。
7. **选择器 + 冷却 + 重试**
   - `internal/selector/{selector,roundrobin,weighted,affinity}.go`：冷却过滤 → 会话亲和 → 最高优先级档加权随机。
   - `internal/relay/markresult.go`：冷却状态机（429 指数 / 401·403 30min / 404 12h / 5xx 1min / 成功重置）写 `channel_model_health` + 内存镜像。
   - `internal/relay/relay.go`：完整请求流 + 跨渠道重试（可重试失败换渠道，至多 `max_retries`）。
   - 单测：冷却状态机各分支、加权分布、亲和粘附、重试转移。
8. **模型映射全链路**
   - 接通 §6 解析顺序到 relay：prefix 剥离、`[1M]` 后缀剥离、保留客户端面模型名写 `UsageEvent.requested_model`。
   - 保存时映射冲突校验 + 告警。
   - 单测：重命名/通配/prefix 路由、客户端面名保留。
9. **UsageEvent 管道**
   - `internal/relay/usageevent.go`：结构 + 有界 channel（~16k）非阻塞发送；M2 挂占位消费者（drain）。**非密钥/非请求体**。
   - 校验：高并发下发送不阻塞 relay 热路径。
10. **跨方言转换器（最后，带开关）**
   - `internal/adaptor/convert/{openai_claude.go, sse.go}`：类型化 OpenAI-chat ⇄ Claude-messages（tools/tool_calls、流式 chunk）。
   - `relay`：候选 platform 与入站方言不一致时经转换器；特性开关 `relay.enable_cross_dialect`（默认 false）。
   - `convert/suite_test.go`：一致性套件（请求/响应、流式、工具）。**套件绿 → 开关置 true**；未过 → 关开关，仅发布同方言路由。

## 校验命令（完成前全绿）

```bash
sqlc generate && git diff --exit-code internal/store/dbgen   # 生成代码无未提交差异
gofmt -l cmd internal
go vet ./...
CGO_ENABLED=0 go build -o ./bin/proxy-hub ./cmd/proxy-hub
go test ./...                                                # 含 convert 一致性套件
# 端到端（本地）：
./bin/proxy-hub --config ./config.example.yaml &
#  - admin key 建 OpenAI + Claude api_key 渠道 + 一个 upstream 渠道；channel-test 通过
#  - 入站 key 经 /v1/chat/completions、/v1/messages 跑真实流量
#  - 制造 429 → 失败转移；多轮会话亲和粘附
curl -fsS http://127.0.0.1:7777/healthz
```

## 单元/一致性测试要点

- channel：映射解析顺序（精确→最长通配→透传）、ability 增量重建、保存冲突校验。
- selector：冷却各分支、加权分布、会话亲和、重试转移。
- credstore：往返 + 0600 + 坏 JSON 容错。
- auth：哈希校验 + 常量时间比较 + 负缓存 + 禁用拒绝。
- convert：OpenAI⇄Claude 请求/响应/流式/工具一致性套件（守门开关）。

## 评审门 / 回滚

- 评审门：上述校验全绿 + `trellis-check` 干净 + 端到端演示（建渠道→流量→映射→失败转移→亲和）。
- **转换器开关**：一致性套件未过 ⇒ `relay.enable_cross_dialect=false`，仅发布同方言路由（M2 其余项仍可验收/上线）。
- 回滚：特性分支整体回退；DB 迁移 0002 有预迁移备份（M1 框架）；credstore 误写删文件即可。

## 完成后

- `trellis-check`（子代理不可用时主会话人工复核 + 验证套件兜底，见 M1 经验）。
- 提交（Phase 3.4）→ `/trellis:finish-work` 归档 + 日志。
- 按需更新 `.trellis/spec/backend/`（adaptor 契约、selector/cooldown 约定、sqlc 使用规范）。
