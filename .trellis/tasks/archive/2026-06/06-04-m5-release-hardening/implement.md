# M5 —— 加固与发布：执行计划（Implement）

> 依据本子任务 `prd.md` + `design.md` + 父设计。注释中文；`.sql` 查询 ASCII 注释。

## 前置
- 分支 `task/m5-release-hardening`（从含 M1-M4 的 HEAD）。
- 已就绪项（仅验证/文档）：定价 UI、停机 flush、bearer 鉴权。

## 有序步骤（每步可独立编译 + 测试）
1. **config 新增**：`Server.AllowedOrigins`、`Selector.Strategy`、`Health{Enabled,Interval,Timeout}` + 默认 + env + 校验 + example。
2. **FillFirst 选择器**：`selector.New(strategy)`；档内 fill_first 取序（priority desc/weight desc/id asc）vs round_robin 加权随机。单测两策略。relay/main 传 strategy。
3. **origin 加固**：`middleware.OriginCheck(allowed)`；server.go 加到 /admin 与 /v0/mcp 组（Auth 之后，仅拦非 GET）。单测。
4. **迁移 0005 + 健康探测**：`0005_health.sql`（health_check_logs）+ queries + sqlc。`internal/health`：Prober（注入 probe 函数 + DAO + HealthMirror.Mark），默认关。`GET /admin/health/checks` 只读汇总。单测（合成 probe）。
5. **main 装配**：selector strategy；health.enabled 才起 Prober；OriginCheck 中间件。
6. **goreleaser + Docker 复核**：`.goreleaser.yaml`（多 OS/arch，CGO 关，ldflags 版本）；`goreleaser check`。
7. **文档**：README quickstart + `docs/`（security/config-reference/mcp-guide/scope-rationale）。
8. **验证 + 端到端**：全绿 + 停机 flush 测试 + 探测开/关验证。

## 校验命令（完成前全绿）
```bash
sqlc generate && git diff --exit-code internal/store/dbgen
gofmt -l cmd internal && go vet ./... && CGO_ENABLED=0 go build -o ./bin/proxy-hub ./cmd/proxy-hub && go test ./...
cd web && npm run build
"$(go env GOPATH)/bin/goreleaser" check 2>/dev/null || echo "goreleaser 未安装，跳过 check"
```

## 评审门 / 回滚
评审门：全绿 + `trellis-check` + 父验收清单（v1 生命周期/探测默认关/FillFirst 可选/origin 拒跨域/停机 flush）。
回滚：分支回退；迁移 0005 前向 + 预备份；health 默认关、origin 默认保守。

## 完成后
`trellis-check`（主会话内联兜底）→ 提交 → `/trellis:finish-work` → 更新 `.trellis/spec/`（健康探测/选择器策略/origin 安全模型）。父任务 `06-04-proxy-hub-mvp` 全 5/5 后归档。
