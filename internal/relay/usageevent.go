// Package relay 编排一次中转：鉴权后提取 → 路由解析 → 选择器选渠道 → 适配器构造上游请求 →
// 调用（带跨渠道重试）→ 回写客户端 → 发出 UsageEvent + MarkResult 健康。
package relay

import (
	"sync/atomic"
	"time"
)

// UsageEvent 是一次完成请求的用量事实（M2 发出，M3 采集器消费落库）。
// 只含 token / 时延 / 状态，**不含**密钥与请求/响应体；成本由 M3 读取时按定价计算。
type UsageEvent struct {
	RequestID           string
	RequestedModel      string // 客户端面名（含 prefix）
	UpstreamModel       string // 上游名
	Group               string
	EndpointFormat      string
	APIKeyID            int64
	ChannelID           int64
	IsStream            bool
	IsError             bool
	StatusCode          int
	InputTokens         int
	OutputTokens        int
	ReasoningTokens     int
	CacheReadTokens     int
	CacheCreationTokens int
	TotalTokens         int
	LatencyMS           int64
	FirstTokenMS        int64 // TTFT；0 表示未知
	ErrorType           string
	SessionID           string
	UsageSource         string // stream | usage_block | estimated | missing
	CreatedAt           time.Time
}

// Emitter 是有界、非阻塞的用量事件通道。满时丢弃并计数（不静默：计数经 Dropped 暴露给仪表盘）。
type Emitter struct {
	ch      chan UsageEvent
	dropped atomic.Int64
}

// NewEmitter 创建容量为 buffer 的事件发射器。
func NewEmitter(buffer int) *Emitter {
	if buffer <= 0 {
		buffer = 16384
	}
	return &Emitter{ch: make(chan UsageEvent, buffer)}
}

// Emit 非阻塞发送事件；通道满则丢弃并递增 dropped。绝不阻塞中转热路径。
func (e *Emitter) Emit(ev UsageEvent) {
	select {
	case e.ch <- ev:
	default:
		e.dropped.Add(1)
	}
}

// Events 返回只读事件通道（M3 采集器消费）。
func (e *Emitter) Events() <-chan UsageEvent {
	return e.ch
}

// Dropped 返回累计丢弃事件数（溢出暴露，非静默丢弃）。
func (e *Emitter) Dropped() int64 {
	return e.dropped.Load()
}

// Close 关闭事件通道。
func (e *Emitter) Close() {
	close(e.ch)
}

// DrainAndDiscard 是 M2 的占位消费者：丢弃事件、保活通道，待 M3 采集器替换。
// 返回的 done 在通道关闭后被 close，便于优雅停机等待。
func DrainAndDiscard(e *Emitter) (done <-chan struct{}) {
	d := make(chan struct{})
	go func() {
		defer close(d)
		for range e.Events() {
			// M2 阶段丢弃；M3 采集器接管落库 + 滚动汇总。
		}
	}()
	return d
}
