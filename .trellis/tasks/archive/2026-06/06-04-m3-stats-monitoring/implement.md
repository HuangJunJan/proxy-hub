# M3 —— 统计与监控：执行计划（Implement）

> `06-04-m3-stats-monitoring` 有序执行清单与校验。依据本子任务 `prd.md` + `design.md` + 父设计。
> 语言：代码注释中文；`.sql` 查询文件 ASCII 注释。继承 M1/M2 约定（`.trellis/spec/backend/`）。

## 前置

- 分支：从当前（含 M1+M2）开 `task/m3-stats-monitoring`（`task.py set-branch`）。
- 工具链：Go 1.25（后端）；**Node + npm（前端 Vite）——开工前先 `node -v`/`npm -v` 确认可用**，不可用则
  先做后端（步骤 1-7、9-10）、前端（步骤 8）待环境就绪。
- 新依赖：`github.com/shopspring/decimal`（金额）。前端：React/Vite/Tailwind/shadcn/ui/TanStack Query。

## 有序步骤（每步保持可独立编译 + 测试）

1. **抽出 internal/usage**（边界重整，先做以解耦）
   - 移 `relay/usageevent.go` 的 `UsageEvent→usage.Event`、`Emitter`、`DrainAndDiscard` 到 `internal/usage/usage.go`。
   - 调 `relay`：`Engine.emitter`/`Config.Emitter` 改 `*usage.Emitter`；`fillUsage` 填 `usage.Event`；`Serve` 引用更新。
   - 调 `main`：`usage.NewEmitter`；更新 `.trellis/spec/backend/relay-channel.md` 的 UsageEvent 归属说明。
   - 校验：`go build ./... && go test ./internal/relay/...` 全绿（relay 行为不变）。
2. **迁移 0003 + queries + sqlc**
   - `internal/store/migrations/0003_stats.sql`：`request_logs`/`usage_hourly_rollups`/`usage_daily_rollups`/`model_pricing` + 索引（design §2）。**ASCII 注释**。
   - `internal/store/queries/{request_logs,rollups,pricing}.sql`：批量插入、UPSERT 累加、仪表盘查询、清理。**ASCII 注释**。
   - `sqlc generate` → 提交 `dbgen`；校验 `go build ./internal/store/...` + `git diff --exit-code internal/store/dbgen`。
   - 注意字段名（`database-guidelines.md` sqlc 约定）：`api_key_id→ApiKeyID`、`first_token_ms` 可空→`sql.NullInt64`、bool 列为 int64。
3. **stats/event.go**：`ClassifyError`、`HourBucket`/`DayBucket`。单测（边界）。
4. **stats/pricing.go**：seed.json（`pricing/seed.json` + `go:embed`，curated per-million）；`Table` 载入/重载；`Cost` decimal + OpenAI/Claude 缓存语义 + `pricing_missing`。单测（扣缓存对比、missing）。
5. **stats/dao.go**：`InsertLogsBatch`/`InsertLog`、`UpsertHourly`/`UpsertDaily`、仪表盘查询、`CleanupRawLogs`、`SeedPricing`/`ListPricing`/`UpsertPricing`/`DeletePricing`、`LoadHealth`（复用 channel.DAO 或新查询）。单测（批量、UPSERT 累加、查询形状、清理留汇总）。
6. **stats/collector.go**：单消费协程（批 100/200ms + 滚动缓冲 + 60s UPSERT flush + 关机 flush + 溢出计数 + 可选同步兜底）。单测（合成事件、注入 now/手动 tick、溢出非静默、关机 flush）。
7. **api/stats_handlers.go**：`/admin/stats/{overview,timeseries,breakdown,logs,health}` + `/admin/pricing` CRUD；成本在 handler 用 `pricing.Table` 算；DTO 含 cost(decimal 字符串)+pricing_missing+overflow。注册到 server.go admin 组。`config.StatsConfig`（design §9）+ env + 校验 + example。
   - main 装配：`usage.NewEmitter` → 注入 relay + `stats.NewCollector`；`collector.Run` 取代 `DrainAndDiscard`；启动种子定价 + 保留清理 ticker；停机 flush 等 done。
   - 校验：合成流量经 collector → 查询 API 返回正确聚合；`go test ./internal/...` 全绿。
8. **前端 web/**（Node 可用时）
   - `npm create vite`（React+TS）+ Tailwind + shadcn/ui + TanStack Query + Router；`vite.config.ts` port 8888 + proxy → 7777。
   - admin key 输入 + localStorage + 请求注入；API client（TanStack Query）。
   - 页面：概览/趋势/分组/日志钻取/渠道健康/（渠道·Key·定价管理复用 M2 admin API）。
   - `npm run build` → `web/dist`；后端 `go:embed` 提供（M1 `registerStatic` 已就绪）。
   - 校验：`npm run lint`/`typecheck`/`build` 干净；dev 8888 代理 7777 联通；生产单端口加载 SPA。
9. **保留期清理**：启动跑一次 + 每日 ticker（`CleanupRawLogs`）；日志记删除行数。
10. **端到端 + 收尾**：合成/真实流量 → 仪表盘各视图；改价历史重算；制造溢出仪表盘可见；保留期生效。

## 校验命令（完成前全绿）

```bash
sqlc generate && git diff --exit-code internal/store/dbgen
gofmt -l cmd internal && go vet ./... && CGO_ENABLED=0 go build -o ./bin/proxy-hub ./cmd/proxy-hub && go test ./...
# 前端（Node 可用时）：
cd web && npm ci && npm run lint && npm run typecheck && npm run build   # 产出 web/dist
# 端到端：起服务，合成流量 → curl /admin/stats/overview 等；改价重算；溢出计数可见。
curl -fsS http://127.0.0.1:7777/healthz
```

## 单元/一致性测试要点

- pricing：OpenAI 扣 cache_read vs Claude 纯新、pricing_missing、decimal 精度。
- collector：批量阈值/定时、滚动缓冲折叠、60s flush、**溢出计数非静默**、关机 flush（注入时钟）。
- dao：批量插入、UPSERT 累加正确、各查询形状、CleanupRawLogs 删原始留汇总。
- 前端：组件渲染 + API mock（vitest/RTL）；build 干净。

## 评审门 / 回滚

- 评审门：上述全绿 + `trellis-check` 干净 + 端到端演示（流量→仪表盘→改价重算→溢出可见→保留清理）。
- 回滚：特性分支整体回退；迁移 0003 前向 + 预迁移备份；前端纯增量；`internal/usage` 抽取可回退（重引耦合）。

## 完成后

- `trellis-check`（子代理不可用时主会话内联复核 + 验证套件兜底，见 M1/M2 经验）。
- 提交（Phase 3.4，**用户要求时才 commit/push**）→ `/trellis:finish-work` 归档 + 日志。
- 更新 `.trellis/spec/`：stats 采集器/定价/汇总约定、前端 `frontend/` 规范（首个前端里程碑）。
