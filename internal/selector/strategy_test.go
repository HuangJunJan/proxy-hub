package selector

import (
	"testing"

	"github.com/huangjunjan/proxy-hub/internal/channel"
)

func TestFillFirstSticksToPreferred(t *testing.T) {
	s := NewWithStrategy(StrategyFillFirst)
	cands := []*channel.ChannelRuntime{
		{ChannelID: 1, Priority: 50, Weight: 1, UpstreamModel: "m"},
		{ChannelID: 2, Priority: 50, Weight: 5, UpstreamModel: "m"}, // weight 最大 → 首选
		{ChannelID: 3, Priority: 50, Weight: 5, UpstreamModel: "m"}, // 同 weight，id 更大
	}
	for i := 0; i < 10; i++ {
		got, err := s.Pick(cands, "", nil)
		if err != nil || got.ChannelID != 2 {
			t.Fatalf("fill_first 应稳定选 ch2（weight 最大），实际 %v err=%v", got, err)
		}
	}
	// ch2 冷却 → 回退到同 weight 的次选 ch3。
	block := func(id int64, _ string) bool { return id == 2 }
	if got, _ := s.Pick(cands, "", block); got.ChannelID != 3 {
		t.Fatalf("ch2 冷却应回退 ch3，实际 %v", got)
	}
}

func TestFillFirstHighestPriorityTier(t *testing.T) {
	s := NewWithStrategy(StrategyFillFirst)
	cands := []*channel.ChannelRuntime{
		{ChannelID: 1, Priority: 100, Weight: 1, UpstreamModel: "m"}, // 更高优先级档
		{ChannelID: 2, Priority: 50, Weight: 9, UpstreamModel: "m"},
	}
	if got, _ := s.Pick(cands, "", nil); got.ChannelID != 1 {
		t.Fatalf("应选最高优先级档 ch1，实际 %v", got)
	}
}

func TestUnknownStrategyFallsBackRoundRobin(t *testing.T) {
	s := NewWithStrategy("bogus")
	if s.strategy != StrategyRoundRobin {
		t.Errorf("未知策略应回退 round_robin，实际 %q", s.strategy)
	}
}
