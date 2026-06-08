package health

import (
	"context"
	"testing"

	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/config"
	"github.com/huangjunjan/proxy-hub/internal/store"
)

func testDAO(t *testing.T) *channel.DAO {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("打开 store 失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return channel.NewDAO(st)
}

func TestProbeWritesLogAndMarks(t *testing.T) {
	dao := testDAO(t)
	var marked []string
	p := NewProber(Config{
		DAO:  dao,
		Mark: func(_ int64, model string, _ int, _ bool, _ string) { marked = append(marked, model) },
	})
	// 注入假探测：成功 200。
	p.probeFn = func(_ context.Context, _ channel.Channel, _ string) probeResult {
		return probeResult{success: true, status: 200, ms: 12}
	}
	p.probe(context.Background(), channel.Channel{ID: 1}, "gpt-4o")

	checks, err := dao.ListRecentHealthChecks(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || !checks[0].Success || checks[0].HTTPStatus != 200 || checks[0].Model != "gpt-4o" || checks[0].ResponseTimeMS != 12 {
		t.Fatalf("应写入 1 条成功探测记录: %+v", checks)
	}
	if len(marked) != 1 || marked[0] != "gpt-4o" {
		t.Errorf("应 Mark 一次 gpt-4o，实际 %v", marked)
	}
}

func TestProbeOnceNoChannels(t *testing.T) {
	dao := testDAO(t)
	p := NewProber(Config{DAO: dao})
	p.ProbeOnce(context.Background()) // 无渠道 → 不 panic、无记录
	checks, _ := dao.ListRecentHealthChecks(context.Background(), 10)
	if len(checks) != 0 {
		t.Errorf("无渠道不应有探测记录，实际 %d", len(checks))
	}
}
