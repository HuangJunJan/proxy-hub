package channel

import (
	"reflect"
	"testing"
)

func TestParseModels(t *testing.T) {
	got, err := ParseModels(`["gpt-4o","gpt-4o-mini"]`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"gpt-4o", "gpt-4o-mini"}) {
		t.Errorf("解析结果不符: %v", got)
	}
	if m, err := ParseModels(""); err != nil || m != nil {
		t.Errorf("空串应得空列表，实际 %v, %v", m, err)
	}
	if m, err := ParseModels("[]"); err != nil || m != nil {
		t.Errorf("空 JSON 应得空列表，实际 %v, %v", m, err)
	}
	if _, err := ParseModels("[bad"); err == nil {
		t.Error("坏 JSON 应报错")
	}
}

func TestParseModelMapping(t *testing.T) {
	got, err := ParseModelMapping(`{"gpt-4":"gpt-4o","claude-3*":"claude-3-5*"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got["gpt-4"] != "gpt-4o" || got["claude-3*"] != "claude-3-5*" {
		t.Errorf("解析结果不符: %v", got)
	}
	if m, err := ParseModelMapping("{}"); err != nil || len(m) != 0 {
		t.Errorf("空 JSON 应得空映射，实际 %v, %v", m, err)
	}
}

func TestStripPrefix(t *testing.T) {
	if s, ok := StripPrefix("azure/gpt-4", "azure/"); !ok || s != "gpt-4" {
		t.Errorf("应剥离前缀得 gpt-4，实际 %q, %v", s, ok)
	}
	if s, ok := StripPrefix("gpt-4", "azure/"); ok || s != "gpt-4" {
		t.Errorf("不匹配前缀应原样返回，实际 %q, %v", s, ok)
	}
	if s, ok := StripPrefix("gpt-4", ""); ok || s != "gpt-4" {
		t.Errorf("空前缀应原样返回，实际 %q, %v", s, ok)
	}
}

func TestStripContextSuffix(t *testing.T) {
	cases := map[string]string{
		"claude-sonnet-4[1M]": "claude-sonnet-4",
		"gpt-4o":              "gpt-4o",
		"model[200K]":         "model",
		"[weird]":             "[weird]", // '[' 在首位不处理
		"no-bracket":          "no-bracket",
	}
	for in, want := range cases {
		if got := StripContextSuffix(in); got != want {
			t.Errorf("StripContextSuffix(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestMatchWildcard(t *testing.T) {
	// 通配命中：捕获剩余段 + 前缀长度。
	captured, n, ok := MatchWildcard("gpt-4*", "gpt-4-turbo")
	if !ok || captured != "-turbo" || n != 5 {
		t.Errorf("MatchWildcard 命中应得 (-turbo, 5, true)，实际 (%q, %d, %v)", captured, n, ok)
	}
	// 通配不命中。
	if _, _, ok := MatchWildcard("gpt-4*", "claude-3"); ok {
		t.Error("不匹配前缀不应命中")
	}
	// 非通配模式一律 false（精确匹配由调用方处理）。
	if _, _, ok := MatchWildcard("gpt-4", "gpt-4"); ok {
		t.Error("非通配模式应返回 false")
	}
	// 最长匹配比较：更长前缀得分更高。
	_, nShort, _ := MatchWildcard("gpt*", "gpt-4-turbo")
	_, nLong, _ := MatchWildcard("gpt-4*", "gpt-4-turbo")
	if !(nLong > nShort) {
		t.Errorf("更长前缀应得分更高：gpt-4*=%d 应 > gpt*=%d", nLong, nShort)
	}
}

func TestSubstituteWildcard(t *testing.T) {
	if got := SubstituteWildcard("azure-*", "-turbo"); got != "azure--turbo" {
		t.Errorf("通配替换错误: %q", got)
	}
	if got := SubstituteWildcard("fixed-model", "-turbo"); got != "fixed-model" {
		t.Errorf("无 * 应原样返回: %q", got)
	}
}

func TestBuildAbilities(t *testing.T) {
	c := Channel{
		ID:       42,
		Group:    "default",
		Priority: 50,
		Weight:   2,
		Enabled:  true,
		Models:   []string{"gpt-4o", "gpt-4"},
		ModelMapping: map[string]string{
			"gpt-4": "gpt-4o", // 覆盖 models 中的 gpt-4（mapping 优先，dedup）
			"big*":  "gpt-4o*",
		},
	}
	abs := BuildAbilities(c)

	// 期望别名集合：big*、gpt-4（映射到 gpt-4o）、gpt-4o（透传）。
	byAlias := map[string]Ability{}
	for _, a := range abs {
		byAlias[a.AliasModel] = a
	}
	if len(abs) != 3 {
		t.Fatalf("应生成 3 条 ability，实际 %d：%+v", len(abs), abs)
	}
	if byAlias["gpt-4"].UpstreamModel != "gpt-4o" {
		t.Errorf("gpt-4 应映射到 gpt-4o，实际 %q", byAlias["gpt-4"].UpstreamModel)
	}
	if byAlias["gpt-4o"].UpstreamModel != "gpt-4o" {
		t.Errorf("gpt-4o 应透传，实际 %q", byAlias["gpt-4o"].UpstreamModel)
	}
	if byAlias["big*"].UpstreamModel != "gpt-4o*" {
		t.Errorf("big* 应映射到 gpt-4o*，实际 %q", byAlias["big*"].UpstreamModel)
	}
	// 公共字段透传 + 排序确定性。
	if byAlias["gpt-4"].ChannelID != 42 || byAlias["gpt-4"].Priority != 50 || byAlias["gpt-4"].Weight != 2 || !byAlias["gpt-4"].Enabled {
		t.Errorf("ability 公共字段未正确透传: %+v", byAlias["gpt-4"])
	}
	for i := 1; i < len(abs); i++ {
		if abs[i-1].AliasModel > abs[i].AliasModel {
			t.Errorf("abilities 应按 alias 排序，实际未排序: %+v", abs)
		}
	}
}

func TestBuildAbilitiesWithPrefix(t *testing.T) {
	c := Channel{
		ID: 1, Group: "default", Prefix: "team1/", Enabled: true,
		Models:       []string{"gpt-4o"},
		ModelMapping: map[string]string{"fast*": "gpt-4o-mini*"},
	}
	byAlias := map[string]Ability{}
	for _, a := range BuildAbilities(c) {
		byAlias[a.AliasModel] = a
	}
	// 客户端面别名含前缀；upstream 不含前缀。
	if a, ok := byAlias["team1/gpt-4o"]; !ok || a.UpstreamModel != "gpt-4o" {
		t.Errorf("透传别名应为 team1/gpt-4o→gpt-4o，实际 %+v ok=%v", a, ok)
	}
	if a, ok := byAlias["team1/fast*"]; !ok || a.UpstreamModel != "gpt-4o-mini*" {
		t.Errorf("通配别名应为 team1/fast*→gpt-4o-mini*（upstream 不带前缀），实际 %+v ok=%v", a, ok)
	}
}
