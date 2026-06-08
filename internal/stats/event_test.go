package stats

import (
	"testing"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/usage"
)

func TestBuckets(t *testing.T) {
	ts := time.Date(2026, 6, 8, 14, 37, 30, 0, time.UTC)
	if got := HourBucket(ts); got != "2026-06-08T14:00:00Z" {
		t.Errorf("HourBucket = %q，期望 2026-06-08T14:00:00Z", got)
	}
	if got := DayBucket(ts); got != "2026-06-08" {
		t.Errorf("DayBucket = %q，期望 2026-06-08", got)
	}
	// 非 UTC 输入应先转 UTC。
	loc := time.FixedZone("X", 3600)
	if got := HourBucket(time.Date(2026, 6, 8, 0, 30, 0, 0, loc)); got != "2026-06-07T23:00:00Z" {
		t.Errorf("HourBucket(非UTC) = %q，期望 2026-06-07T23:00:00Z", got)
	}
}

func TestNormalizeErrorType(t *testing.T) {
	cases := []struct {
		status  int
		isErr   bool
		errType string
		want    string
	}{
		{200, false, "", ""},
		{200, false, "x", ""},                           // 非错误一律空
		{500, true, "upstream_error", "upstream_error"}, // 优先 relay 给的类型
		{429, true, "", "rate_limit"},
		{401, true, "", "auth"},
		{403, true, "", "auth"},
		{404, true, "", "not_found"},
		{502, true, "", "upstream_5xx"},
		{400, true, "", "client_error"},
		{0, true, "", "unknown"},
	}
	for _, c := range cases {
		if got := NormalizeErrorType(c.status, c.isErr, c.errType); got != c.want {
			t.Errorf("NormalizeErrorType(%d,%v,%q) = %q，期望 %q", c.status, c.isErr, c.errType, got, c.want)
		}
	}
}

func TestBillableInput(t *testing.T) {
	// OpenAI：prompt 含缓存读，扣减得纯新。
	if got := BillableInput(usage.Event{EndpointFormat: "openai", InputTokens: 100, CacheReadTokens: 30}); got != 70 {
		t.Errorf("OpenAI billable = %d，期望 70", got)
	}
	// OpenAI：缓存读 > 输入，下限 0。
	if got := BillableInput(usage.Event{EndpointFormat: "openai", InputTokens: 10, CacheReadTokens: 30}); got != 0 {
		t.Errorf("OpenAI billable 下限 = %d，期望 0", got)
	}
	// responses 同属 OpenAI 家族。
	if got := BillableInput(usage.Event{EndpointFormat: "responses", InputTokens: 100, CacheReadTokens: 40}); got != 60 {
		t.Errorf("responses billable = %d，期望 60", got)
	}
	// Claude：input 本为纯新，不扣。
	if got := BillableInput(usage.Event{EndpointFormat: "claude", InputTokens: 100, CacheReadTokens: 30}); got != 100 {
		t.Errorf("Claude billable = %d，期望 100", got)
	}
}
