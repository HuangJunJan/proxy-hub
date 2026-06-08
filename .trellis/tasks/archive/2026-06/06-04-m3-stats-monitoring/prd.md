# M3 —— 统计与监控

> `06-04-proxy-hub-mvp` 的子任务。共享设计见父 `design.md`（能力 C2）。
>
> 语言约定：本项目所有规划文档与代码注释一律使用中文（见 `AGENTS.md`）。

## 目标

在绝不阻塞中转、绝不静默丢失计费数据的前提下采集每请求用量；聚合成快速的时序滚动汇总；成本在
读取时由配置驱动的定价表计算；并暴露仪表盘 API + React 管理后台，呈现按 key/渠道/模型 的 token、
成本、延迟（含 TTFT）与错误分析。

## 范围

**范围内**
- 表：`request_logs`（append-only，**只存 token——不预存成本**）、`usage_hourly_rollups`、
  `usage_daily_rollups`（只存 token/延迟聚合）、`model_pricing`（从内置 `pricing/seed.json` 种入，
  管理端可覆盖）。
- `internal/stats`：`event`（UsageEvent + 错误分类器）、`collector`（有界 channel + 单消费协程 +
  批量插入（100 行 / 200ms）+ 内存滚动缓冲 + 60s UPSERT flush + 关机 flush + **非静默溢出**：
  计数 + 可选同步回退）、`pricing`（读取时 decimal 计算成本、正确的 Claude-vs-OpenAI 缓存语义、
  `pricing_missing` 标记）、`dao`（批量插入、汇总 upsert、仪表盘查询、保留期清理）。
- 从 relay 结果路径做被动 `channel_model_health` 更新（与 M2 的 `MarkResult` 共享）；`/healthz` 已在
  M1 提供。
- 仪表盘读取 API：overview、timeseries、breakdown（按模型/渠道/key/error_type）、分页请求日志钻取
  （按 `request_id` 查）、渠道健康。
- React 仪表盘页面：概览卡片、趋势图、分组表、请求日志钻取。
  - 前端工程（Vite）**开发服务器端口固定 8888**，开发期把 API 路径（`/v1`、`/admin`、`/v0`、
    `/healthz`）代理到后端 **7777**（`vite.config.ts` 的 `server.port` + `server.proxy`）。
  - **生产环境无独立前端端口**：前端构建产物经 `go:embed` 由后端单端口（7777）提供，遵守父 `prd.md`
    的「单服务 / 单卷 / 单对外端口」约束（FR-C3-3）。
- 保留期清理（配置 `retention_days`，原始日志默认 30；汇总保留）在启动 + 每日，批量 DELETE。

**范围外**
- 主动心跳探测 + `health_check_logs`（M5）。按 key 配额/RPM（OQ-4）。告警。跨方言/中转逻辑（M2）。

## 需求（来自父 `prd.md`）

FR-C2-1 … FR-C2-6；NFR-2（热路径不阻塞）、NFR-3（decimal 金额、不静默丢弃、不存请求体）。

## 依赖

**依赖 M1**（store/迁移）。消费 **M2** 的 `UsageEvent` 流 + `MarkResult` 健康写入获取实时数据；
schema + 采集器 + 仪表盘可在 M2 完成前用合成事件构建并单测。**可与 M4 并行。**

## 交付清单

- [ ] `request_logs`、`usage_hourly_rollups`、`usage_daily_rollups`、`model_pricing` 的迁移；
      内置 `pricing/seed.json` + 加载器。
- [ ] `internal/stats/{event,collector,pricing,dao}`，含批量插入 + 滚动缓冲 + flush。
- [ ] 非静默溢出（仪表盘暴露计数 + 同步回退选项）。
- [ ] 读取时成本计算，含正确缓存 token 语义 + `pricing_missing` 标记。
- [ ] 仪表盘读取 API（overview/timeseries/breakdown/logs/health）+ 保留期清理作业。
- [ ] React 仪表盘页面（概览/图表/分组/日志钻取）。

## 验收标准

- [ ] 高负载下，中转热路径只做非阻塞发送；请求路径上无 DB I/O。
- [ ] 仪表盘展示按 key/渠道/模型 的 token、成本、延迟、TTFT、错误率，含时序趋势与 `request_id` 钻取。
- [ ] 修改某模型定价可对历史聚合重算（成本读取时计算）。
- [ ] 人为制造的溢出被计数并在仪表盘暴露——绝不静默丢弃。
- [ ] 超过 `retention_days` 的旧原始日志被清理；汇总保留且仍可查询。
- [ ] `gofmt`/`go vet`/`go test ./...` + `web` lint/typecheck/build 干净；`trellis-check` 干净。

## 备注

子任务 `design.md`（采集器内部、汇总键、查询形状）与 `implement.md` 在子任务启动时从父 `design.md`
撰写。
