# 统计与监控规范（Stats / Monitoring Guidelines）

> proxy-hub M3 确立的用量采集、聚合、读时定价与仪表盘契约。M4/M5 在此之上扩展。
> 语言：中文注释；标识符/表名/列名/路径英文；`.sql` 查询文件 ASCII 注释。

---

## 数据流（热路径零 DB I/O）

```
relay.Serve ──Emit(usage.Event)──> usage.Emitter(有界,非阻塞)
                                        │ Events()
                              stats.Collector(单消费协程)
                                ├─ 批量 InsertLogsBatch(100行/200ms) ─> request_logs(原始 token)
                                └─ 折叠内存滚动汇总, 60s UPSERT ────────> usage_*_rollups(归一 token/延迟)
仪表盘读 ──> stats.DAO(只读句柄, SQL 内聚合) ──读时──> pricing.Table(decimal) 算成本
```

- **`internal/usage`** 是中立包（`Event`/`Emitter`/`DrainAndDiscard`）：relay 生产、stats 消费，二者都不互相依赖（避免环）。
- **热路径只 `Emitter.Emit`（非阻塞）**：默认满则丢弃 + `Dropped()` 计数；`onFull != nil`（`stats.sync_fallback_on_full`）时改同步兜底（绝不丢计费，代价是热路径偶发阻塞，opt-in）。
- 采集器是**单协程**：批量插事实行 + 内存滚动汇总定时 UPSERT；通道关闭（停机）触发最终 flush。所有写经 `store.Write()` 单写句柄。

## 成本：读取时计算，绝不预存

- `request_logs` / `usage_*_rollups` **只存 token/延迟**；`model_pricing`（per-million decimal TEXT）单独表。
- 成本在**查询时**由 `pricing.Table.Compute` 用 `shopspring/decimal` 按分量算 ⇒ **改价即对全部历史视图重算**。改价（admin PUT）后 handler 重载内存表。
- **缓存语义归一在采集时**（`BillableInput`）：OpenAI 的 `prompt_tokens` 含缓存读 ⇒ 汇总存 `input - cache_read`（下限 0）；Claude `input_tokens` 本为纯新 ⇒ 直接存。故汇总成本无需再分方言。事实表存**原始** input（钻取保真）；仪表盘聚合视图的 input 为归一纯新值。
- 未知模型 ⇒ `pricing_missing=true`，成本按 0，token 统计照常；仪表盘列出缺价模型。
- 定价种子 `internal/stats/pricing_seed.json`（`go:embed`，per-million），启动经 `SeedModelPricing`（`ON CONFLICT DO NOTHING`）种入，**绝不覆盖 admin 改价**。

## 汇总键与查询

- 小时汇总 PK `(bucket_hour, channel_id, api_key_id, requested_model)`；日汇总 PK `(bucket_date, channel_id, requested_model)`。桶键为 UTC（`HourBucket`=RFC3339 截整点 / `DayBucket`=YYYY-MM-DD），字符串字典序即时间序，范围比较用 `bucket >= since`。
- UPSERT 累加：`... ON CONFLICT(pk) DO UPDATE SET x = x + excluded.x`。事实表是 SOT，汇总可重建。
- 聚合视图（overview/timeseries/breakdown-model/channel/api_key）读小时汇总；**error_type 分组读 `request_logs`**（汇总不带 error_type）。钻取 logs 读 `request_logs`（`sqlc.narg` 可选过滤 + 分页）。
- **保留清理**：`CleanupRawLogs`（`DELETE FROM request_logs WHERE created_at < ?`）启动跑一次 + 每日 ticker；汇总永不删。`retention_days<=0` 跳过。

## 仪表盘 API（`/admin/stats/*` + `/admin/pricing`，admin key 守护）

`overview`(区间汇总+成本+缺价+dropped) · `timeseries?interval=hour|day` · `breakdown?by=model|channel|api_key|error_type` · `logs`(分页钻取，可按 request_id/api_key_id/channel_id/model 过滤，含读时成本) · `health`(渠道×模型) · `pricing` GET/PUT :model/DELETE :model（改后重载内存表）。区间 `range` 取 1h|24h|7d|30d。

## 安全不变量（延续 M2，必守）

- 事实表/汇总表**无成本列、无请求/响应体**；`error_message` 截断（M3 暂空，event 未携带）；`UsageEvent` 不含密钥。详见 [relay-channel.md](./relay-channel.md)。

## 测试要点

- pricing：decimal 成本、OpenAI 扣缓存 vs Claude 纯新、pricing_missing、seed 解析。
- collector：合成事件 → 批量+滚动缓冲 flush（通道关闭驱动确定性）、**溢出计数非静默**、onFull 兜底。
- dao：批量插入、UPSERT 累加、各查询、CleanupRawLogs（删原始留汇总）、seed 不覆盖 admin。
