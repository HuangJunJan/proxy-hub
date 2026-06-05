# M5 —— 加固与发布

> `06-04-proxy-hub-mvp` 的子任务。横跨所有能力的最终打磨。共享设计见父 `design.md`。
>
> 语言约定：本项目所有规划文档与代码注释一律使用中文（见 `AGENTS.md`）。

## 目标

把可用的 M1–M4 特性推到可发布的 v1：可选主动健康检查、剩余的负载均衡开关、管理面鉴权/CSRF 决策、
跨平台发布二进制、完整文档与端到端测试覆盖。

## 范围

**范围内**
- 主动健康探测 + `health_check_logs`（可选，默认关）：按渠道、按模型 的定时心跳，带可用性滚动汇总。
- `FillFirst` 选择器策略作为可配置开关（RoundRobin 仍为默认）。
- 定价覆盖管理 UI（编辑 `model_pricing`；暴露 `pricing_missing`）。
- **管理面/SPA 鉴权 + CSRF/origin 决策（OQ-6）**：SPA 用 bearer token + 对改文件端点
  （`/admin/*`、`/v0/mcp/*`）做 origin 校验；并文档化该模型。
- `goreleaser` 跨平台发布二进制（linux/macos/windows，amd64+arm64）。
- 文档：quickstart、security（密钥/卷/权限）、完整 `config.yaml` 参考、MCP 同步指南、
  "为何不做订阅渠道"说明。
- 端到端 + 集成测试；优雅停机 flush 验证（统计缓冲 + 写者）。

**范围外**
- 多节点/集群、外部 DB/Redis 抽象层、按 key 配额/RPM（OQ-4）、告警——均 MVP 后。

## 需求（来自父 `prd.md`）

FR-C2-6（主动探测）、FR-C1-6（FillFirst 开关）、安全类 NFR、父验收标准中的发布/文档部分。

## 依赖

**依赖 M1–M4**（它加固并打包它们的产出）。

## 交付清单

- [ ] `internal/health` 主动探测器 + `health_check_logs` + 可用性汇总（可选）。
- [ ] `FillFirst` 策略 + 配置开关。
- [ ] 定价覆盖 UI。
- [ ] 管理面/SPA 鉴权模型（bearer + origin 校验）实现 + 文档化。
- [ ] `.goreleaser.yaml` 完整；CI 构建各 OS 二进制。
- [ ] 文档：quickstart、security、配置参考、MCP 指南、范围说明。
- [ ] 端到端测试（渠道 → 中转 → 统计 → MCP 同步）；优雅停机 flush 测试。

## 验收标准

- [ ] 下载的 linux/macos/windows 发布二进制可由单一 `config.yaml`、无其它服务下跑完整生命周期。
- [ ] 主动探测（开启时）填充 `health_check_logs` 与可用性统计且不影响实时流量；默认关已验证。
- [ ] FillFirst 可经配置选择；RoundRobin 仍为默认。
- [ ] 改文件的 管理/MCP 端点拒绝跨域/CSRF；强制 bearer 鉴权。
- [ ] 优雅停机时统计缓冲与 SQLite 写者完成 flush 且无数据丢失。
- [ ] 所有 lint/类型检查/测试 全绿；`trellis-check` 干净；父验收清单通过。

## 备注

子任务 `design.md`/`implement.md` 在子任务启动时撰写。
