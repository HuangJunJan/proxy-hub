# M5 —— 加固与发布：技术设计（Design）

> `06-04-m5-release-hardening` 子任务设计。把 M1–M4 推到可发布 v1。继承父设计与各里程碑约定。
> 语言：注释中文；标识符/路径/技术名英文；`.sql` 查询 ASCII 注释。

## 0. 已就绪（前里程碑，本里程碑只验证/文档化）

- 定价覆盖 UI（M3 PricingPage）、读时成本、pricing_missing 暴露 —— 已完成。
- 优雅停机 flush（M3 collector 关机最终 flush + emitter 排空）—— 已实现，本里程碑补验证测试。
- 入站/admin bearer 鉴权（M1/M2 `middleware.Auth` 常量时间比较）—— 已有，本里程碑加 origin 校验。

## 1. 本里程碑新增

### 1.1 主动健康探测（可选，默认关）—— internal/health
- 迁移 `0005_health.sql`：`health_check_logs`（`id, channel_id, model, success, http_status, response_time_ms, message, checked_at`）+ 索引 `(channel_id, checked_at)`。
- `health.Prober`：按 `health.interval`（默认 5m）定时，对**启用渠道 × 其 models**发最小探针（复用 admin `/test` 的 probe 形态：max_tokens=1）；记 `health_check_logs` + 调 relay `HealthMirror.Mark`（成功重置/失败冷却，与被动路径同源）。**默认 `health.enabled=false`**，开启才起 goroutine；绝不阻塞中转热路径（独立 goroutine + 独立 http client）。
- 可用性汇总：查询 `health_check_logs` 近 N 次的成功率（仪表盘只读端点 `GET /admin/health/checks`，可选）。
- dbgen 无关：探测器经注入的「probe 函数」+ DAO（channel.DAO 列渠道/写 health_check_logs）。

### 1.2 FillFirst 选择器策略 + 开关
- `config.Selector.Strategy`（`round_robin`(默认) | `fill_first`）。
- selector 当前：冷却过滤 → 会话亲和 → 最高优先级档 → **档内加权随机**。新增 fill_first：档内**按 (priority desc, weight desc, channelID asc) 取第一个可用**（粘住首选，溢出才下一个），实现「填满优先渠道」。经 `selector.New(strategy)` 注入。
- 默认 round_robin（加权随机）不变。

### 1.3 origin/CSRF 加固（OQ-6）
- `middleware.OriginCheck(allowedOrigins []string)`：对**改文件/状态的请求**（`/admin/*` 与 `/v0/mcp/*` 的非 GET：POST/PUT/DELETE）校验 `Origin`/`Referer` 同源（在 allowed 列表内，或与 Host 同源）；不符 403。GET 只读不拦。bearer 仍强制（叠加）。
- `config.Server.AllowedOrigins []string`（默认空 = 仅同源 Host）。文档化该模型（bearer + 同源写保护）。

### 1.4 发布
- `.goreleaser.yaml`：linux/macos/windows × amd64/arm64，`CGO_ENABLED=0`，注入 version ldflags；archives + checksums。
- 复核 Dockerfile/compose（M1 雏形）：单二进制、单 /data 卷、单端口。

### 1.5 文档（docs/ + README）
- README quickstart（拉起、admin key、建渠道、发请求、看仪表盘）。
- docs：security（密钥/卷/权限/origin 模型）、配置参考（完整 config.yaml）、MCP 同步指南、"为何不做订阅渠道"。

## 2. 配置新增（config）

```
server:
  allowed_origins: []        # 改文件端点的额外放行 origin（默认仅同源 Host）
selector:
  strategy: round_robin      # round_robin | fill_first
health:
  enabled: false             # 主动探测总开关（默认关）
  interval: 5m               # 探测周期
  timeout: 20s               # 单次探针超时
```
环境 `PROXY_HUB_SELECTOR_STRATEGY` / `PROXY_HUB_HEALTH_ENABLED` / `PROXY_HUB_HEALTH_INTERVAL` 等；校验枚举/非负。

## 3. 装配（main）

- `health.enabled` 为真才 `health.NewProber(...).Run(ctx)`（独立 goroutine，停机随 ctx 取消）。
- selector 按 `cfg.Selector.Strategy` 构造。
- `middleware.OriginCheck` 加到 `/admin` 与 `/v0/mcp` 组（在 Auth 之后）。

## 4. 测试

- health：合成 probe 函数 → 探一轮 → 写 health_check_logs + Mark 调用；默认关不起 goroutine。
- selector：fill_first 粘首选、round_robin 加权分布（注入随机源）；冷却过滤两者一致。
- middleware：OriginCheck 同源放行 / 跨域 POST 403 / GET 放行 / allowed 列表放行。
- 优雅停机：emitter 关闭 → collector flush，事件不丢（已可用 M3 collector 测试覆盖，补一条端到端）。
- goreleaser：`goreleaser check`（配置合法）；本机无 docker 守护进程则跳过镜像构建（记录）。

## 5. 验收映射 / 回滚

§1.1 ⇒ FR-C2-6 主动探测 + 默认关；§1.2 ⇒ FR-C1-6 FillFirst 开关；§1.3 ⇒ 安全 NFR + OQ-6；§1.4/§1.5 ⇒ 发布/文档。
回滚：特性分支；迁移 0005 前向 + 预迁移备份；health 默认关、origin 默认仅同源（保守）。
