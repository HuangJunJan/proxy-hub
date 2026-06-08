package selector

import (
	"testing"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/channel"
)

var selBaseTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func rt(id int64, priority, weight int) *channel.ChannelRuntime {
	return &channel.ChannelRuntime{ChannelID: id, UpstreamModel: "m", Priority: priority, Weight: weight}
}

func TestPickAllBlocked(t *testing.T) {
	s := New()
	cands := []*channel.ChannelRuntime{rt(1, 50, 1), rt(2, 50, 1)}
	_, err := s.Pick(cands, "", func(int64, string) bool { return true })
	if err != ErrNoCandidate {
		t.Errorf("全部冷却应返回 ErrNoCandidate，实际 %v", err)
	}
}

func TestPickFiltersBlocked(t *testing.T) {
	s := New()
	cands := []*channel.ChannelRuntime{rt(1, 50, 1), rt(2, 50, 1)}
	// 阻塞渠道 1，应只可能选到 2。
	got, err := s.Pick(cands, "", func(id int64, _ string) bool { return id == 1 })
	if err != nil {
		t.Fatal(err)
	}
	if got.ChannelID != 2 {
		t.Errorf("应选到未冷却的渠道 2，实际 %d", got.ChannelID)
	}
}

func TestPickHighestPriorityTier(t *testing.T) {
	s := New()
	// 渠道 1 优先级更高，应总是选中（即便权重低）。
	cands := []*channel.ChannelRuntime{rt(1, 100, 1), rt(2, 50, 100)}
	for i := 0; i < 20; i++ {
		got, err := s.Pick(cands, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.ChannelID != 1 {
			t.Fatalf("最高优先级档应只含渠道 1，实际选到 %d", got.ChannelID)
		}
	}
}

func TestWeightedPickDeterministic(t *testing.T) {
	s := New()
	cands := []*channel.ChannelRuntime{rt(1, 50, 1), rt(2, 50, 3)} // total=4

	// r=0 → 落在渠道 1（权重区间 [0,1)）。
	s.randFloat = func() float64 { return 0.0 }
	if got, _ := s.Pick(cands, "", nil); got.ChannelID != 1 {
		t.Errorf("r=0 应选渠道 1，实际 %d", got.ChannelID)
	}
	// r=0.5 → int(0.5*4)=2 → 落在渠道 2（区间 [1,4)）。
	s.randFloat = func() float64 { return 0.5 }
	if got, _ := s.Pick(cands, "", nil); got.ChannelID != 2 {
		t.Errorf("r=0.5 应选渠道 2，实际 %d", got.ChannelID)
	}
}

func TestAffinitySticky(t *testing.T) {
	s := New()
	now := selBaseTime
	s.now = func() time.Time { return now }
	cands := []*channel.ChannelRuntime{rt(1, 100, 1), rt(2, 50, 1)}

	// 记录会话亲和到渠道 2（即便它优先级较低）。
	s.RecordAffinity("sess-A", 2)
	got, err := s.Pick(cands, "sess-A", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.ChannelID != 2 {
		t.Errorf("会话应粘附渠道 2，实际 %d", got.ChannelID)
	}

	// 推进超过 TTL：亲和过期，回到最高优先级档（渠道 1）。
	now = selBaseTime.Add(defaultAffinityTTL + time.Second)
	got, err = s.Pick(cands, "sess-A", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.ChannelID != 1 {
		t.Errorf("亲和过期后应回到渠道 1，实际 %d", got.ChannelID)
	}
}

func TestAffinityFallbackWhenBlocked(t *testing.T) {
	s := New()
	s.now = func() time.Time { return selBaseTime }
	cands := []*channel.ChannelRuntime{rt(1, 100, 1), rt(2, 50, 1)}

	s.RecordAffinity("sess-B", 2)
	// 亲和的渠道 2 被冷却 → 应回退到可用集（渠道 1）。
	got, err := s.Pick(cands, "sess-B", func(id int64, _ string) bool { return id == 2 })
	if err != nil {
		t.Fatal(err)
	}
	if got.ChannelID != 1 {
		t.Errorf("亲和渠道被阻塞应回退到渠道 1，实际 %d", got.ChannelID)
	}
}
