package stats

import (
	"context"
	"testing"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/usage"
)

func TestCollectorDrainsAndFlushesOnClose(t *testing.T) {
	d := testDAO(t)
	em := usage.NewEmitter(100, nil)
	// 禁用定时（设为很大），靠通道关闭触发最终 flush，使测试确定性。
	c := NewCollector(CollectorConfig{
		DAO: d, Emitter: em, BatchSize: 1000, BatchInterval: time.Hour, FlushInterval: time.Hour,
	})
	done := c.Run(context.Background())

	now := time.Date(2026, 6, 8, 14, 0, 0, 0, time.UTC)
	const n = 5
	for i := 0; i < n; i++ {
		em.Emit(ev("gpt-4o", 100, 20, "openai", false, now))
	}
	em.Close()
	<-done

	ctx := context.Background()
	count, err := d.CountLogs(ctx, LogFilter{})
	if err != nil || count != n {
		t.Fatalf("事实行 = %d；期望 %d (err=%v)", count, n, err)
	}
	sum, err := d.SumHourlyRange(ctx, "2026-06-08T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if sum.RequestCount != n {
		t.Errorf("汇总 request_count = %d；期望 %d", sum.RequestCount, n)
	}
	// 归一纯新输入：openai input 100、cache_read 0 -> 100；5 条 -> 500。output 20*5=100。
	if sum.InputTokens != 500 || sum.OutputTokens != 100 {
		t.Errorf("汇总 token in=%d out=%d；期望 500/100", sum.InputTokens, sum.OutputTokens)
	}
}

func TestCollectorOverflowCountsNonSilent(t *testing.T) {
	// buffer 1、无 onFull、无消费者：填满后再发即计数丢弃（绝不静默）。
	em := usage.NewEmitter(1, nil)
	em.Emit(ev("m", 1, 1, "openai", false, time.Now())) // 占满 buffer
	em.Emit(ev("m", 1, 1, "openai", false, time.Now())) // 满 -> drop
	em.Emit(ev("m", 1, 1, "openai", false, time.Now())) // 满 -> drop
	if got := em.Dropped(); got != 2 {
		t.Errorf("Dropped = %d；期望 2", got)
	}
}

func TestCollectorOnFullFallback(t *testing.T) {
	// onFull 兜底：通道满时改调 onFull（同步），不计 dropped。
	var caught int
	em := usage.NewEmitter(1, func(usage.Event) { caught++ })
	em.Emit(ev("m", 1, 1, "openai", false, time.Now())) // 占满
	em.Emit(ev("m", 1, 1, "openai", false, time.Now())) // 满 -> onFull
	if caught != 1 || em.Dropped() != 0 {
		t.Errorf("onFull 兜底: caught=%d dropped=%d；期望 1/0", caught, em.Dropped())
	}
}
