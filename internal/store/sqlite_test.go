package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRequestLogRepository(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	keyIndex := 1
	firstTokenMS := int64(37)
	promptTokens := int64(12)
	completionTokens := int64(8)
	reasoningTokens := int64(4)
	totalTokens := int64(24)
	err := store.BatchInsert(ctx, []LogEntry{
		{
			TimestampMS:      1000,
			APIKeyTokenMask:  "sk-...1234",
			APIKeyName:       "local",
			Endpoint:         "/v1/chat/completions",
			RequestType:      "chat.completions",
			ChannelName:      "openai",
			ChannelType:      "openai-api",
			DownstreamModel:  "gpt-4o",
			UpstreamModel:    "gpt-4o",
			UpstreamKeyIndex: &keyIndex,
			StatusCode:       200,
			IsStream:         true,
			DurationMS:       120,
			FirstTokenMS:     &firstTokenMS,
			ReasoningEffort:  "high",
			BillingMode:      "token",
			PromptTokens:     &promptTokens,
			CompletionTokens: &completionTokens,
			ReasoningTokens:  &reasoningTokens,
			TotalTokens:      &totalTokens,
			Attempts:         1,
			UserAgent:        "proxy-hub-test-agent",
		},
		{
			TimestampMS:     2000,
			APIKeyTokenMask: "sk-...1234",
			ChannelName:     "deepseek",
			ChannelType:     "openai-api",
			DownstreamModel: "gpt-5.4",
			UpstreamModel:   "deepseek-chat",
			StatusCode:      502,
			DurationMS:      40,
			ErrorKind:       "upstream_5xx",
			RequestBody:     []byte(`{"model":"gpt-5.4"}`),
			Attempts:        2,
		},
	})
	if err != nil {
		t.Fatalf("BatchInsert() error = %v", err)
	}

	got, err := store.Query(ctx, QueryFilter{ChannelName: "deepseek"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Query() len = %d, want 1", len(got))
	}
	if got[0].DownstreamModel != "gpt-5.4" || got[0].Attempts != 2 {
		t.Fatalf("Query() entry = %+v", got[0])
	}
	all, err := store.Query(ctx, QueryFilter{})
	if err != nil {
		t.Fatalf("Query(all) error = %v", err)
	}
	first := all[len(all)-1]
	if first.Endpoint != "/v1/chat/completions" || first.RequestType != "chat.completions" || first.ReasoningEffort != "high" || first.BillingMode != "token" || first.UserAgent != "proxy-hub-test-agent" {
		t.Fatalf("context fields were not round-tripped: %+v", first)
	}
	if first.FirstTokenMS == nil || *first.FirstTokenMS != 37 {
		t.Fatalf("FirstTokenMS = %v, want 37", first.FirstTokenMS)
	}
	if first.ReasoningTokens == nil || *first.ReasoningTokens != 4 {
		t.Fatalf("ReasoningTokens = %v, want 4", first.ReasoningTokens)
	}

	filtered, err := store.Query(ctx, QueryFilter{APIKey: "local", Model: "gpt-4o", Endpoint: "chat", RequestType: "chat.completions", StatusClass: "success"})
	if err != nil {
		t.Fatalf("Query(filtered) error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ChannelName != "openai" {
		t.Fatalf("Query(filtered) = %+v, want openai log", filtered)
	}
	filtered, err = store.Query(ctx, QueryFilter{ErrorKind: "upstream", StatusClass: "error"})
	if err != nil {
		t.Fatalf("Query(error filtered) error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ChannelName != "deepseek" {
		t.Fatalf("Query(error filtered) = %+v, want deepseek log", filtered)
	}

	deleted, err := store.DeleteBefore(ctx, 1500)
	if err != nil {
		t.Fatalf("DeleteBefore() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("DeleteBefore() = %d, want 1", deleted)
	}
}

func TestStatsRepository(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	err := store.UpsertHourly(ctx, []HourlyDelta{
		{
			ChannelName:      "openai",
			HourTimestampMS:  0,
			Requests:         2,
			Successes:        2,
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
			AvgDurationMS:    100,
		},
		{
			ChannelName:     "openai",
			HourTimestampMS: 0,
			Requests:        1,
			Failures:        1,
			AvgDurationMS:   400,
		},
		{
			ChannelName:     "deepseek",
			HourTimestampMS: 3600000,
			Requests:        1,
			Successes:       1,
			AvgDurationMS:   80,
		},
	})
	if err != nil {
		t.Fatalf("UpsertHourly() error = %v", err)
	}

	summaries, err := store.QueryChannelSummary(ctx, TimeWindow{})
	if err != nil {
		t.Fatalf("QueryChannelSummary() error = %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("QueryChannelSummary() len = %d, want 2", len(summaries))
	}
	openai := summaries[1]
	if openai.ChannelName != "openai" {
		openai = summaries[0]
	}
	if openai.Requests != 3 || openai.Successes != 2 || openai.Failures != 1 || openai.AvgDurationMS != 200 {
		t.Fatalf("openai summary = %+v", openai)
	}

	points, err := store.QuerySeries(ctx, "openai", MetricRequests, TimeWindow{})
	if err != nil {
		t.Fatalf("QuerySeries() error = %v", err)
	}
	if len(points) != 1 || points[0].Value != 3 {
		t.Fatalf("QuerySeries() = %+v, want one point value 3", points)
	}
}

func openTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "proxy-hub.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	return store
}
