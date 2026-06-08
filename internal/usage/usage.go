// Package usage 定义中转用量事件与其有界非阻塞发射器。
//
// 置于中立包（既非 relay 也非 stats）：relay 作为生产者 Emit、stats 采集器作为消费者读 Events，
// 二者都只依赖本包，避免 stats → relay 的耦合与潜在导入环。
package usage

import (
	"sync/atomic"
	"time"
)

// Event 是一次完成请求的用量事实（relay 发出，stats 采集器消费落库）。
// 只含 token / 时延 / 状态 / 维度 ID，**不含**密钥与请求/响应体；成本由读取时按定价计算。
type Event struct {
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

// Emitter 是有界、非阻塞的用量事件通道。满时：若配置了 onFull 则交其同步兜底（绝不丢计费），
// 否则丢弃并计数（不静默：计数经 Dropped 暴露给仪表盘）。
type Emitter struct {
	ch      chan Event
	dropped atomic.Int64
	onFull  func(Event) // 可选；装配期设定（须在任何 Emit 前），通道满时调用以同步落库兜底
}

// NewEmitter 创建容量为 buffer 的事件发射器（buffer<=0 取默认 16384）。
// onFull 为 nil 时通道满即丢弃计数；非 nil 时通道满改调 onFull（同步兜底，绝不丢计费数据）。
// onFull 必须在任何 Emit 之前设定（装配期），运行期只读，故无数据竞争。
func NewEmitter(buffer int, onFull func(Event)) *Emitter {
	if buffer <= 0 {
		buffer = 16384
	}
	return &Emitter{ch: make(chan Event, buffer), onFull: onFull}
}

// Emit 非阻塞发送事件；通道满则按 onFull 兜底或丢弃计数。默认（onFull=nil）绝不阻塞中转热路径。
func (e *Emitter) Emit(ev Event) {
	select {
	case e.ch <- ev:
	default:
		if e.onFull != nil {
			e.onFull(ev) // 同步兜底（opt-in）：在热路径上承担一次写入，换取不丢计费
			return
		}
		e.dropped.Add(1)
	}
}

// Events 返回只读事件通道（stats 采集器消费）。
func (e *Emitter) Events() <-chan Event {
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

// DrainAndDiscard 是丢弃事件、保活通道的占位消费者（M2 用；M3 由 stats 采集器替换，保留供测试/降级）。
// 返回的 done 在通道关闭后被 close，便于优雅停机等待。
func DrainAndDiscard(e *Emitter) (done <-chan struct{}) {
	d := make(chan struct{})
	go func() {
		defer close(d)
		for range e.Events() {
			// 丢弃。
		}
	}()
	return d
}
