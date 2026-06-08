package stats

import (
	"testing"
)

func TestPricingCompute(t *testing.T) {
	tbl := NewTable()
	tbl.Load([]PriceRow{
		{ModelID: "gpt-4o", InputPerMillion: "2.5", OutputPerMillion: "10", CacheReadPerMillion: "1.25", CacheCreationPerMillion: "0"},
		{ModelID: "claude-x", InputPerMillion: "3", OutputPerMillion: "15", CacheReadPerMillion: "0.3", CacheCreationPerMillion: "3.75"},
	})

	// gpt-4o：input 1M -> 2.5；output 1M -> 10；cache_read 1M -> 1.25；total 13.75。
	c, missing := tbl.Compute("gpt-4o", 1_000_000, 1_000_000, 1_000_000, 0)
	if missing {
		t.Fatal("gpt-4o 应有定价")
	}
	if c.Total.String() != "13.75" {
		t.Errorf("total = %s，期望 13.75", c.Total.String())
	}
	if c.Input.String() != "2.5" || c.Output.String() != "10" || c.CacheRead.String() != "1.25" {
		t.Errorf("分量成本不符: in=%s out=%s cr=%s", c.Input, c.Output, c.CacheRead)
	}

	// cache_creation 计费。
	c2, _ := tbl.Compute("claude-x", 0, 0, 0, 2_000_000)
	if c2.Total.String() != "7.5" { // 2M * 3.75 / 1M = 7.5
		t.Errorf("claude cache_creation total = %s，期望 7.5", c2.Total.String())
	}

	// 缺价模型：missing=true，成本 0。
	c3, missing3 := tbl.Compute("unknown-model", 1_000_000, 1_000_000, 0, 0)
	if !missing3 {
		t.Error("unknown-model 应标记 missing")
	}
	if !c3.Total.IsZero() {
		t.Errorf("缺价成本应为 0，实际 %s", c3.Total)
	}

	if !tbl.Has("gpt-4o") || tbl.Has("nope") {
		t.Error("Has 判定错误")
	}
}

func TestPricingComputeZeroTokens(t *testing.T) {
	tbl := NewTable()
	tbl.Load([]PriceRow{{ModelID: "m", InputPerMillion: "5", OutputPerMillion: "5", CacheReadPerMillion: "5", CacheCreationPerMillion: "5"}})
	c, missing := tbl.Compute("m", 0, 0, 0, 0)
	if missing || !c.Total.IsZero() {
		t.Errorf("零 token 成本应为 0 且非 missing，实际 total=%s missing=%v", c.Total, missing)
	}
}

func TestSeedEntries(t *testing.T) {
	entries, err := SeedEntries()
	if err != nil {
		t.Fatalf("解析内置定价种子失败: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("种子应非空")
	}
	// 种子值应可被定价表解析（不为解析失败的 0 兜底误导）。
	found := false
	for _, e := range entries {
		if e.ModelID == "gpt-4o" {
			found = true
			if e.OutputPerMillion == "" {
				t.Error("gpt-4o 种子缺 output 单价")
			}
		}
	}
	if !found {
		t.Error("种子应含 gpt-4o")
	}
}
