# proxy-hub MVP — 执行计划（父任务图）

> 有序的里程碑计划、校验命令、评审门、回滚点。每个里程碑是一个可独立验证的子任务。每个子任务的
> 详细执行计划在其自身的 `implement.md`（在该子任务启动时撰写）。架构见 `design.md`。
>
> 语言约定：本项目所有规划文档与代码注释一律使用中文（见 `AGENTS.md`）。

## 顺序与依赖

```
M1 foundations ─┬─> M2 channel-relay ─┬─> (实时数据) ──> M3 stats ─┐
                │                      │                            ├─> M5 release-hardening
                └──────────────────────┴──> M4 mcp-sharing ─────────┘
```

- **M1 阻塞一切**（store、config、API 骨架、打包）。
- **M2 依赖 M1。** M2 是最大的里程碑；其跨方言转换器是风险最高的子项，由一致性套件守门。
- **M3 依赖 M1**（schema/store）并消费 M2 的 `UsageEvent`/`MarkResult` 实时数据；schema + 采集器
  可在 M2 完成前用合成事件构建。
- **M4 依赖 M1**（DB + `fileio` 助手），其余与 M2/M3 无关——可与 M3 并行。
- **M5 是最终打磨**，横跨所有特性。

建议启动顺序：**M1 → M2 → (M3 ∥ M4) → M5**。启动拥有下一个交付物的子任务；绝不启动父任务。

## 各里程碑计划

### M1 —— 基础（`06-04-m1-foundations`）
交付物：`go.mod`（Go 1.25）；`cmd/proxy-hub/main.go` 瘦入口；`internal/store`（开 SQLite WAL +
pragmas、读连接池 + 单写协程、版本化迁移器含预迁移 `.db` 备份 + 表重建模式）；`internal/config`
（YAML + 环境覆盖 + fsnotify 热重载）；`internal/api` Gin 服务器 + 中间件 + `/healthz`；
Dockerfile → alpine（单服务/卷/端口）+ 容器 HEALTHCHECK；`go:embed` 空 SPA 壳；`.goreleaser.yaml`
雏形；`config.example.yaml`。
评审门 / 演示：`docker run -v ./data:/data -p 7777:7777 proxy-hub` 启动，`/healthz` 返回 200，
DB + 迁移已建，编辑配置触发热重载日志。回滚：回退分支。

### M2 —— 渠道管理 + API-Key 中转（`06-04-m2-channel-relay`）
交付物：`channels`/`abilities`/`api_keys` 表 + DAO + 增量 `RouteIndex`；渠道 CRUD 管理 API +
channel-test；`credstore`（`data/auths/*.json`）；入站 API-Key 鉴权；`Adaptor` 接口 + 同方言
透传；统一端点中转到 api_key/upstream 渠道；RoundRobin + 加权优先级 + 会话亲和选择器；
跨渠道重试循环；`MarkResult` 按 (渠道,模型) 冷却 + `channel_model_health`；模型映射（精确 +
尾部 `*` 最长匹配 + prefix，含命名空间碰撞解析顺序）；每渠道出口代理；**最后**：单一
OpenAI-chat ⇄ Claude-messages 类型化转换器 + 一致性套件（流式 + 工具）。
评审门 / 演示：创建 OpenAI + Claude api_key 渠道及一个 upstream 渠道，经统一端点跑真实流量，
模型重命名 + 通配生效，429 时失败转移，会话粘在一个渠道，OpenAI 请求由 Claude upstream 服务
（反之亦然）通过一致性套件。回滚：特性分支；若套件未过，可关闭转换器开关、仅发布同方言路由。

### M3 —— 统计与监控（`06-04-m3-stats-monitoring`）
交付物：`request_logs`（只存 token）+ `usage_hourly_rollups` + `usage_daily_rollups` +
`model_pricing`（从 `pricing/seed.json` 种入）；`internal/stats` 异步采集器（有界 channel +
单消费 + 批量插入 + 滚动缓冲 + 60s flush + 关机 flush + **非静默溢出**计数/同步回退）；读取时
计算成本，含正确的 Claude-vs-OpenAI 缓存语义；从 relay 结果做被动 `channel_model_health`；
仪表盘读取 API（overview/timeseries/breakdown/logs/health）；保留期清理；React 仪表盘（概览卡片、
趋势图、分组表、分页 request_id 钻取）。
评审门 / 演示：跑流量 → 仪表盘展示按 key/渠道/模型 的 token/成本/延迟/TTFT/错误率 + 趋势 + 钻取；
定价变更可对历史重算；人为制造的溢出被计数/暴露而非静默丢弃。回滚：特性分支。

### M4 —— MCP 共享管理（`06-04-m4-mcp-sharing`）
交付物：`mcp_servers` + `mcp_sync_targets` 表；`internal/mcp` store/service/clients；`fileio`
（原子 temp+rename + `.bak` + 拷权限 + 跳过缺失 + 按目标锁 + 写前重读）；Claude 写入器（整文件
保留、只替换 `mcpServers`、Windows `cmd /c` + WSL 检测）；Codex 写入器（`pelletier/go-toml/v2`
`[mcp_servers.<id>]` 外科手术式编辑 + 遗留清理 + JSON↔TOML）；`/v0/mcp/*` API + `proxy-hub
mcp sync` CLI；可选 HOME 自动探测；管理 UI（列表/开关/"立即同步"/目标）；导入（只读、冲突翻
开关位、"spec differs"告警）+ import-bundle。
评审门 / 演示：定义一个 server，启用 Codex+Claude，"立即同步" → 正确出现在 `~/.codex/config.toml`
与 `~/.claude.json`，所有无关内容（含 Claude `projects`）字节级保留；再次导入读回而不覆盖 spec。
回滚：`.bak` 恢复任何误同步的客户端文件；特性分支回退代码。

### M5 —— 加固与发布（`06-04-m5-release-hardening`）
交付物：主动健康探测 + `health_check_logs`（可选）；FillFirst 策略开关；定价覆盖 UI；管理端/SPA
鉴权 + CSRF/origin 决策（OQ-6）；goreleaser 跨平台二进制；完整文档（quickstart、security、配置
参考）；端到端 + 集成测试；关机 flush 验证。
评审门 / 演示：下载各 OS 发布二进制，由单一配置文件、无其它服务下跑完整生命周期（渠道 → 中转 →
统计 → MCP 同步）。

## 校验命令（每个子任务在报告完成前运行）

```bash
gofmt -l . && go vet ./...
go build -o /dev/null ./...          # CGO_ENABLED=0
go test ./...                        # 含 转换器一致性 + fileio 往返测试
# 前端（涉及时）：
cd web && pnpm lint && pnpm typecheck && pnpm build
```

每个子任务实现后运行 `trellis-check`；提交前修完发现的问题。

## 评审门

1. **规划门（每子任务）**：子任务 `prd.md` + `design.md` + `implement.md` 审阅通过后再
   `task.py start <child>`（状态 → in_progress）。父 `design.md` 是共享参考；子 `design.md` 只需
   覆盖子任务特有细节。
2. **合并前门（每子任务）**：校验命令全绿 + `trellis-check` 干净 + 里程碑演示可复现。
3. **跨子集成（父）**：父 `prd.md` 的验收清单端到端通过后再归档父任务。

## 回滚点

- 每个里程碑是自己的分支/子任务；可独立回退。
- DB 迁移：前向式 + 预迁移 `.db` 备份；回滚 = 恢复备份 + 旧二进制。
- MCP 客户端文件写入：首次写前的 `.bak` 备份即坏同步的回滚。
- M2 跨方言转换器由开关守门：套件未过则仅发布同方言路由。

## 关于任务创建的说明

本任务树因 Bash 安全分类器一度不可用，是用 Write 工具手工创建的（非 `task.py create`）。目录名
严格按 `task.py` 在 2026-06-04 会生成的格式（`06-04-<slug>`）。**已于 2026-06-04 运行
`task.py list` + `task.py validate` 确认：父子结构被正确识别、全部 6 个任务校验通过。** 启动任何
子任务前：
1. 审阅工件（`prd.md` / `design.md` / `implement.md`）—— Phase 1.4 评审门。
2. `python ./.trellis/scripts/task.py start 06-04-m1-foundations` —— 状态翻 in_progress，进入实现。

若 `task.py` 不识别手工创建的目录（极少数情况），用
`task.py create "<title>" --slug <slug> [--parent 06-04-proxy-hub-mvp]` 重建并把工件正文拷过去
（slug 已对齐本脚本在 2026-06-04 会生成的带日期前缀目录名）。
