package monitor

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"proxy-hub/internal/store"
)

func TestSubmitFlushesLogsAndStats(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db, err := store.OpenSQLite(ctx, filepath.Join(t.TempDir(), "proxy-hub.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	service := NewService(db, db, nil, Options{BufferSize: 4, BatchSize: 2, FlushInterval: time.Hour})
	go service.Run(ctx, Options{BatchSize: 2, FlushInterval: time.Hour})

	service.Submit(store.LogEntry{
		TimestampMS:     3600000,
		APIKeyTokenMask: "sk-...1234",
		ChannelName:     "openai",
		ChannelType:     "openai-api",
		DownstreamModel: "gpt-4o",
		StatusCode:      200,
		DurationMS:      100,
	})
	service.Submit(store.LogEntry{
		TimestampMS:     3600100,
		APIKeyTokenMask: "sk-...1234",
		ChannelName:     "openai",
		ChannelType:     "openai-api",
		DownstreamModel: "gpt-4o",
		StatusCode:      502,
		DurationMS:      300,
	})

	requireEventually(t, func() bool {
		logs, err := db.Query(context.Background(), store.QueryFilter{ChannelName: "openai"})
		return err == nil && len(logs) == 2
	})
	summaries, err := db.QueryChannelSummary(context.Background(), store.TimeWindow{})
	if err != nil {
		t.Fatalf("QueryChannelSummary() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].Requests != 2 || summaries[0].Successes != 1 || summaries[0].Failures != 1 || summaries[0].AvgDurationMS != 200 {
		t.Fatalf("summaries = %+v", summaries)
	}
}

func TestHubPublishesToSubscribers(t *testing.T) {
	hub := NewHub(1)
	ch, cancel := hub.Subscribe()
	defer cancel()
	hub.Publish(store.LogEntry{ChannelName: "openai"})
	select {
	case got := <-ch:
		if got.ChannelName != "openai" {
			t.Fatalf("ChannelName = %q", got.ChannelName)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published entry")
	}
}

func TestCleanupOnceDeletesExpiredLogs(t *testing.T) {
	ctx := context.Background()
	db, err := store.OpenSQLite(ctx, filepath.Join(t.TempDir(), "proxy-hub.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	old := time.Now().Add(-10 * 24 * time.Hour).UnixMilli()
	recent := time.Now().UnixMilli()
	if err := db.BatchInsert(ctx, []store.LogEntry{
		{
			TimestampMS:     old,
			APIKeyTokenMask: "sk-...old",
			DownstreamModel: "gpt-4o",
			StatusCode:      200,
			DurationMS:      10,
		},
		{
			TimestampMS:     recent,
			APIKeyTokenMask: "sk-...new",
			DownstreamModel: "gpt-4o",
			StatusCode:      200,
			DurationMS:      10,
		},
	}); err != nil {
		t.Fatalf("BatchInsert() error = %v", err)
	}

	service := NewService(db, db, nil, Options{})
	deleted, err := service.CleanupOnce(ctx, 7)
	if err != nil {
		t.Fatalf("CleanupOnce() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("CleanupOnce() deleted = %d, want 1", deleted)
	}
	logs, err := db.Query(ctx, store.QueryFilter{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(logs) != 1 || logs[0].APIKeyTokenMask != "sk-...new" {
		t.Fatalf("remaining logs = %+v", logs)
	}
}

func requireEventually(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
