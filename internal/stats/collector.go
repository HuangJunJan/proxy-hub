package stats

import (
	"context"
	"log/slog"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/store/dbgen"
	"github.com/huangjunjan/proxy-hub/internal/usage"
)

// 采集器默认参数（见 design §4.2 / §9）。
const (
	defaultBatchSize     = 100
	defaultBatchInterval = 200 * time.Millisecond
	defaultFlushInterval = 60 * time.Second
)

// Collector 是单消费协程：从 usage.Emitter 收事件 → 批量插事实行 + 折叠进内存滚动汇总 →
// 定时 UPSERT 汇总；通道关闭（停机）或 ctx 取消时做最终 flush。绝不在中转热路径做 DB I/O。
type Collector struct {
	dao           *DAO
	emitter       *usage.Emitter
	batchN        int
	batchInterval time.Duration
	flushInterval time.Duration
}

// CollectorConfig 是采集器装配参数（<=0 的时长/批量取默认）。
type CollectorConfig struct {
	DAO           *DAO
	Emitter       *usage.Emitter
	BatchSize     int
	BatchInterval time.Duration
	FlushInterval time.Duration
}

// NewCollector 创建采集器。
func NewCollector(cfg CollectorConfig) *Collector {
	c := &Collector{
		dao:           cfg.DAO,
		emitter:       cfg.Emitter,
		batchN:        cfg.BatchSize,
		batchInterval: cfg.BatchInterval,
		flushInterval: cfg.FlushInterval,
	}
	if c.batchN <= 0 {
		c.batchN = defaultBatchSize
	}
	if c.batchInterval <= 0 {
		c.batchInterval = defaultBatchInterval
	}
	if c.flushInterval <= 0 {
		c.flushInterval = defaultFlushInterval
	}
	return c
}

// Run 启动单消费协程，返回 done（协程退出后被 close，供优雅停机等待）。
// 正常停机：调用方 emitter.Close() ⇒ 通道关闭 ⇒ 最终 flush 后退出。
func (c *Collector) Run(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go c.loop(ctx, done)
	return done
}

// rollupAgg 是某桶的累加器（input 存归一后的纯新输入，见 BillableInput）。
type rollupAgg struct {
	requestCount, successCount, errorCount                         int64
	inputTokens, outputTokens, cacheRead, cacheCreation, reasoning int64
	sumLatencyMs, sumFirstTokenMs, countFirstToken                 int64
}

func (a *rollupAgg) add(ev *usage.Event) {
	a.requestCount++
	if ev.IsError {
		a.errorCount++
	} else {
		a.successCount++
	}
	a.inputTokens += BillableInput(*ev)
	a.outputTokens += int64(ev.OutputTokens)
	a.cacheRead += int64(ev.CacheReadTokens)
	a.cacheCreation += int64(ev.CacheCreationTokens)
	a.reasoning += int64(ev.ReasoningTokens)
	a.sumLatencyMs += ev.LatencyMS
	if ev.FirstTokenMS > 0 {
		a.sumFirstTokenMs += ev.FirstTokenMS
		a.countFirstToken++
	}
}

type hourKey struct {
	hour      string
	channelID int64
	apiKeyID  int64
	model     string
}

type dayKey struct {
	date      string
	channelID int64
	model     string
}

func (c *Collector) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	batch := make([]usage.Event, 0, c.batchN)
	hourly := map[hourKey]*rollupAgg{}
	daily := map[dayKey]*rollupAgg{}

	batchTicker := time.NewTicker(c.batchInterval)
	defer batchTicker.Stop()
	flushTicker := time.NewTicker(c.flushInterval)
	defer flushTicker.Stop()

	// flushBatch 把累积的事实行批量落库（失败仅记日志，事件已在汇总缓冲中计入）。
	flushBatch := func() {
		if len(batch) == 0 {
			return
		}
		if err := c.dao.InsertLogsBatch(context.Background(), batch); err != nil {
			slog.Error("批量插入请求日志失败", "count", len(batch), "error", err)
		}
		batch = batch[:0]
	}

	// flushRollups 把内存滚动汇总 UPSERT 落库；成功才清空对应缓冲（失败保留待下次重试）。
	flushRollups := func() {
		if len(hourly) > 0 {
			rows := make([]dbgen.UpsertHourlyRollupParams, 0, len(hourly))
			for k, a := range hourly {
				rows = append(rows, dbgen.UpsertHourlyRollupParams{
					BucketHour: k.hour, ChannelID: k.channelID, ApiKeyID: k.apiKeyID, RequestedModel: k.model,
					RequestCount: a.requestCount, SuccessCount: a.successCount, ErrorCount: a.errorCount,
					InputTokens: a.inputTokens, OutputTokens: a.outputTokens, CacheReadTokens: a.cacheRead,
					CacheCreationTokens: a.cacheCreation, ReasoningTokens: a.reasoning,
					SumLatencyMs: a.sumLatencyMs, SumFirstTokenMs: a.sumFirstTokenMs, CountFirstToken: a.countFirstToken,
				})
			}
			if err := c.dao.UpsertHourly(context.Background(), rows); err != nil {
				slog.Error("UPSERT 小时汇总失败（保留缓冲重试）", "buckets", len(rows), "error", err)
			} else {
				clear(hourly)
			}
		}
		if len(daily) > 0 {
			rows := make([]dbgen.UpsertDailyRollupParams, 0, len(daily))
			for k, a := range daily {
				rows = append(rows, dbgen.UpsertDailyRollupParams{
					BucketDate: k.date, ChannelID: k.channelID, RequestedModel: k.model,
					RequestCount: a.requestCount, SuccessCount: a.successCount, ErrorCount: a.errorCount,
					InputTokens: a.inputTokens, OutputTokens: a.outputTokens, CacheReadTokens: a.cacheRead,
					CacheCreationTokens: a.cacheCreation, ReasoningTokens: a.reasoning,
					SumLatencyMs: a.sumLatencyMs, SumFirstTokenMs: a.sumFirstTokenMs, CountFirstToken: a.countFirstToken,
				})
			}
			if err := c.dao.UpsertDaily(context.Background(), rows); err != nil {
				slog.Error("UPSERT 日汇总失败（保留缓冲重试）", "buckets", len(rows), "error", err)
			} else {
				clear(daily)
			}
		}
	}

	fold := func(ev *usage.Event) {
		hk := hourKey{HourBucket(ev.CreatedAt), ev.ChannelID, ev.APIKeyID, ev.RequestedModel}
		ha := hourly[hk]
		if ha == nil {
			ha = &rollupAgg{}
			hourly[hk] = ha
		}
		ha.add(ev)
		dk := dayKey{DayBucket(ev.CreatedAt), ev.ChannelID, ev.RequestedModel}
		da := daily[dk]
		if da == nil {
			da = &rollupAgg{}
			daily[dk] = da
		}
		da.add(ev)
	}

	finalFlush := func() {
		// 退出前尽量排空：先收掉通道里已缓冲的事件，再做最终 flush。
		for {
			select {
			case ev, ok := <-c.emitter.Events():
				if !ok {
					flushBatch()
					flushRollups()
					return
				}
				batch = append(batch, ev)
				fold(&ev)
			default:
				flushBatch()
				flushRollups()
				return
			}
		}
	}

	for {
		select {
		case ev, ok := <-c.emitter.Events():
			if !ok {
				flushBatch()
				flushRollups()
				return
			}
			batch = append(batch, ev)
			fold(&ev)
			if len(batch) >= c.batchN {
				flushBatch()
			}
		case <-batchTicker.C:
			flushBatch()
		case <-flushTicker.C:
			flushRollups()
		case <-ctx.Done():
			finalFlush()
			return
		}
	}
}
