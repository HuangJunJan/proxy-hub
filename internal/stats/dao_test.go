package stats

import (
	"context"
	"testing"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/config"
	"github.com/huangjunjan/proxy-hub/internal/store"
	"github.com/huangjunjan/proxy-hub/internal/store/dbgen"
	"github.com/huangjunjan/proxy-hub/internal/usage"
)

// testDAO 在临时目录打开真实 store（含 0001-0003 迁移）并返回 DAO。
func testDAO(t *testing.T) *DAO {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("打开 store 失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewDAO(st)
}

func ev(model string, in, out int, format string, isErr bool, at time.Time) usage.Event {
	return usage.Event{
		RequestID: "r-" + model, RequestedModel: model, UpstreamModel: model, Group: "default",
		EndpointFormat: format, APIKeyID: 1, ChannelID: 2, InputTokens: in, OutputTokens: out,
		TotalTokens: in + out, LatencyMS: 50, StatusCode: 200, IsError: isErr, CreatedAt: at,
	}
}

func TestDAOInsertAndQueryLogs(t *testing.T) {
	d := testDAO(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 8, 14, 0, 0, 0, time.UTC)

	events := []usage.Event{
		ev("gpt-4o", 100, 20, "openai", false, now),
		ev("claude-x", 200, 40, "claude", true, now.Add(time.Second)),
	}
	if err := d.InsertLogsBatch(ctx, events); err != nil {
		t.Fatalf("批量插入失败: %v", err)
	}

	total, err := d.CountLogs(ctx, LogFilter{})
	if err != nil || total != 2 {
		t.Fatalf("CountLogs = %d, err=%v；期望 2", total, err)
	}
	// 按模型过滤。
	byModel, err := d.CountLogs(ctx, LogFilter{Model: "gpt-4o"})
	if err != nil || byModel != 1 {
		t.Fatalf("CountLogs(model=gpt-4o) = %d；期望 1", byModel)
	}
	logs, err := d.ListLogs(ctx, LogFilter{Limit: 10})
	if err != nil || len(logs) != 2 {
		t.Fatalf("ListLogs len = %d；期望 2", len(logs))
	}
	// 倒序：最新（claude-x）在前。
	if logs[0].RequestedModel != "claude-x" {
		t.Errorf("ListLogs 应倒序，首条 = %s", logs[0].RequestedModel)
	}
	// 空切片为 no-op。
	if err := d.InsertLogsBatch(ctx, nil); err != nil {
		t.Errorf("空批量应 no-op: %v", err)
	}
}

func TestDAOUpsertHourlyAccumulates(t *testing.T) {
	d := testDAO(t)
	ctx := context.Background()
	row := dbgen.UpsertHourlyRollupParams{
		BucketHour: "2026-06-08T14:00:00Z", ChannelID: 1, ApiKeyID: 1, RequestedModel: "gpt-4o",
		RequestCount: 2, SuccessCount: 2, InputTokens: 100, OutputTokens: 50,
	}
	if err := d.UpsertHourly(ctx, []dbgen.UpsertHourlyRollupParams{row}); err != nil {
		t.Fatal(err)
	}
	if err := d.UpsertHourly(ctx, []dbgen.UpsertHourlyRollupParams{row}); err != nil {
		t.Fatal(err)
	}
	sum, err := d.SumHourlyRange(ctx, "2026-06-08T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if sum.RequestCount != 4 || sum.InputTokens != 200 || sum.OutputTokens != 100 {
		t.Errorf("累加结果不符: req=%d in=%d out=%d；期望 4/200/100", sum.RequestCount, sum.InputTokens, sum.OutputTokens)
	}
}

func TestDAOCleanupRawLogs(t *testing.T) {
	d := testDAO(t)
	ctx := context.Background()
	old := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	if err := d.InsertLogsBatch(ctx, []usage.Event{
		ev("old", 1, 1, "openai", false, old),
		ev("recent", 1, 1, "openai", false, recent),
	}); err != nil {
		t.Fatal(err)
	}
	deleted, err := d.CleanupRawLogs(ctx, "2026-06-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("应删除 1 行旧日志，实际 %d", deleted)
	}
	remain, _ := d.CountLogs(ctx, LogFilter{})
	if remain != 1 {
		t.Errorf("清理后应剩 1 行，实际 %d", remain)
	}
}

func TestDAOPricing(t *testing.T) {
	d := testDAO(t)
	ctx := context.Background()
	now := "2026-06-08T14:00:00Z"
	if err := d.SeedPricing(ctx, []SeedEntry{
		{ModelID: "gpt-4o", InputPerMillion: "2.5", OutputPerMillion: "10", CacheReadPerMillion: "1.25", CacheCreationPerMillion: "0"},
	}, now); err != nil {
		t.Fatal(err)
	}
	// 再次 seed 不应覆盖（DO NOTHING）。
	if err := d.SeedPricing(ctx, []SeedEntry{
		{ModelID: "gpt-4o", InputPerMillion: "999", OutputPerMillion: "999", CacheReadPerMillion: "0", CacheCreationPerMillion: "0"},
	}, now); err != nil {
		t.Fatal(err)
	}
	rows, err := d.PriceRows(ctx)
	if err != nil || len(rows) != 1 || rows[0].InputPerMillion != "2.5" {
		t.Fatalf("seed 不应被二次覆盖: rows=%+v err=%v", rows, err)
	}
	// admin 覆盖。
	if err := d.UpsertPricing(ctx, dbgen.UpsertModelPricingParams{
		ModelID: "gpt-4o", InputPerMillion: "3", OutputPerMillion: "12", CacheReadPerMillion: "0", CacheCreationPerMillion: "0", Source: "admin", UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	list, _ := d.ListPricing(ctx)
	if len(list) != 1 || list[0].InputPerMillion != "3" || list[0].Source != "admin" {
		t.Fatalf("admin 覆盖未生效: %+v", list)
	}
	if err := d.DeletePricing(ctx, "gpt-4o"); err != nil {
		t.Fatal(err)
	}
	if list, _ := d.ListPricing(ctx); len(list) != 0 {
		t.Errorf("删除后应为空，实际 %d", len(list))
	}
}
