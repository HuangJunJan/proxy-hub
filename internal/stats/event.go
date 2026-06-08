// Package stats 采集中转用量事件、聚合时序汇总、读取时按定价表计算成本，并支撑仪表盘查询。
//
// 设计要点（见任务 design.md）：
//   - 热路径只非阻塞发送 usage.Event；本包单消费协程批量落库 + 内存滚动缓冲定时 flush。
//   - 事实表/汇总表只存 token/延迟；成本读取时由 model_pricing 用 decimal 计算（改价可重算历史）。
//   - 绝不静默丢弃：溢出计数经 Emitter.Dropped 暴露。
package stats

import (
	"time"

	"github.com/huangjunjan/proxy-hub/internal/usage"
)

// hourFormat 是小时桶键格式（UTC RFC3339 截到整点，如 2026-06-08T14:00:00Z）。
// dayFormat 是日桶键格式（UTC，如 2026-06-08）。字符串字典序即时间序，便于范围比较。
const (
	dayFormat = "2006-01-02"
)

// HourBucket 返回 t 所在小时的 UTC RFC3339 桶键。
func HourBucket(t time.Time) string {
	return t.UTC().Truncate(time.Hour).Format(time.RFC3339)
}

// DayBucket 返回 t 所在日的 UTC 桶键（YYYY-MM-DD）。
func DayBucket(t time.Time) string {
	return t.UTC().Format(dayFormat)
}

// NormalizeErrorType 归一错误类别用于仪表盘分组：优先用 relay 给出的 errType，
// 缺失时按 HTTP 状态码兜底归类。非错误返回空串。
func NormalizeErrorType(statusCode int, isError bool, errType string) string {
	if !isError {
		return ""
	}
	if errType != "" {
		return errType
	}
	switch {
	case statusCode == 429:
		return "rate_limit"
	case statusCode == 401, statusCode == 403:
		return "auth"
	case statusCode == 404:
		return "not_found"
	case statusCode >= 500:
		return "upstream_5xx"
	case statusCode >= 400:
		return "client_error"
	default:
		return "unknown"
	}
}

// isOpenAIFormat 报告入站方言是否 OpenAI 家族（其 usage.prompt_tokens 含缓存读，需扣减得纯新输入）。
func isOpenAIFormat(endpointFormat string) bool {
	// usage.Event.EndpointFormat 取自 adaptor.EndpointFormat：openai|responses 属 OpenAI 家族；claude 为纯新。
	return endpointFormat == "openai" || endpointFormat == "responses"
}

// BillableInput 返回计费用「纯新输入 token」（已扣除缓存读），统一两种方言的语义：
//   - OpenAI：prompt_tokens 含缓存读 ⇒ 纯新 = input - cache_read（下限 0）。
//   - Claude：input_tokens 本就是纯新 ⇒ 直接用 input。
//
// 汇总表存归一后的纯新输入，使聚合成本无需再分方言即可正确计算（事实表仍存原始 input）。
func BillableInput(ev usage.Event) int64 {
	in := int64(ev.InputTokens)
	if isOpenAIFormat(ev.EndpointFormat) {
		in -= int64(ev.CacheReadTokens)
		if in < 0 {
			in = 0
		}
	}
	return in
}
