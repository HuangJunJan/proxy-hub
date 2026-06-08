package channel

import (
	"reflect"
	"sort"
	"testing"
)

// ca 用 BuildAbilities 把渠道展开为 RouteIndex 输入单元（联动 model.go 与 routeindex.go）。
func ca(c Channel) ChannelAbilities {
	return ChannelAbilities{Channel: c, Abilities: BuildAbilities(c)}
}

// channelIDs 取候选的渠道 ID 集合（排序后便于断言）。
func channelIDs(rts []*ChannelRuntime) []int64 {
	ids := make([]int64, 0, len(rts))
	for _, rt := range rts {
		ids = append(ids, rt.ChannelID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func baseChannel(id int64, group string) Channel {
	return Channel{ID: id, Group: group, Enabled: true, Platform: PlatformOpenAI, Type: TypeAPIKey, Priority: 50, Weight: 1}
}

// TestExactBeatsWildcard：精确别名优先于通配。
func TestExactBeatsWildcard(t *testing.T) {
	c := baseChannel(1, "default")
	c.Models = []string{"gpt-4o"}                       // 透传 ability：gpt-4o -> gpt-4o
	c.ModelMapping = map[string]string{"gpt-*": "up-*"} // 通配：gpt-* -> up-*
	ri := NewRouteIndex()
	ri.Rebuild([]ChannelAbilities{ca(c)})

	rts, ok := ri.Candidates("default", "gpt-4o")
	if !ok || len(rts) != 1 {
		t.Fatalf("gpt-4o 应命中 1 个候选，实际 %v, %d", ok, len(rts))
	}
	if rts[0].UpstreamModel != "gpt-4o" {
		t.Errorf("精确应胜通配，upstream 期望 gpt-4o，实际 %q", rts[0].UpstreamModel)
	}
}

// TestWildcardSubstitution：通配命中并把捕获段代入上游模式。
func TestWildcardSubstitution(t *testing.T) {
	c := baseChannel(1, "default")
	c.ModelMapping = map[string]string{"gpt-*": "up-*"}
	ri := NewRouteIndex()
	ri.Rebuild([]ChannelAbilities{ca(c)})

	rts, ok := ri.Candidates("default", "gpt-4-turbo")
	if !ok || len(rts) != 1 {
		t.Fatalf("gpt-4-turbo 应命中通配，实际 %v, %d", ok, len(rts))
	}
	if rts[0].UpstreamModel != "up-4-turbo" {
		t.Errorf("通配替换错误，upstream 期望 up-4-turbo，实际 %q", rts[0].UpstreamModel)
	}
}

// TestLongestWildcardWins：多个通配命中时取最长前缀。
func TestLongestWildcardWins(t *testing.T) {
	c := baseChannel(1, "default")
	c.ModelMapping = map[string]string{"gpt-*": "a-*", "gpt-4*": "b-*"}
	ri := NewRouteIndex()
	ri.Rebuild([]ChannelAbilities{ca(c)})

	rts, ok := ri.Candidates("default", "gpt-4o")
	if !ok || len(rts) != 1 {
		t.Fatalf("应命中最长通配，实际 %v, %d", ok, len(rts))
	}
	if rts[0].UpstreamModel != "b-o" {
		t.Errorf("最长通配 gpt-4* 应胜，upstream 期望 b-o，实际 %q", rts[0].UpstreamModel)
	}
}

// TestLoadBalanceCandidates：同 (group, alias) 多渠道返回多候选（供选择器加权）。
func TestLoadBalanceCandidates(t *testing.T) {
	a := baseChannel(1, "default")
	a.Models = []string{"gpt-4o"}
	b := baseChannel(2, "default")
	b.Models = []string{"gpt-4o"}
	ri := NewRouteIndex()
	ri.Rebuild([]ChannelAbilities{ca(a), ca(b)})

	rts, ok := ri.Candidates("default", "gpt-4o")
	if !ok {
		t.Fatal("应命中")
	}
	if got := channelIDs(rts); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("应返回渠道 1 与 2 两个候选，实际 %v", got)
	}
}

// TestDisabledExcludedAndGroupIsolation：禁用渠道不入索引；跨 group 不串。
func TestDisabledExcludedAndGroupIsolation(t *testing.T) {
	disabled := baseChannel(1, "default")
	disabled.Enabled = false
	disabled.Models = []string{"gpt-4o"}

	team := baseChannel(2, "team")
	team.Models = []string{"gpt-4o"}

	ri := NewRouteIndex()
	ri.Rebuild([]ChannelAbilities{ca(disabled), ca(team)})

	if _, ok := ri.Candidates("default", "gpt-4o"); ok {
		t.Error("禁用渠道不应命中")
	}
	if _, ok := ri.Candidates("default", "gpt-4o"); ok {
		t.Error("team 渠道不应出现在 default group")
	}
	if rts, ok := ri.Candidates("team", "gpt-4o"); !ok || len(rts) != 1 || rts[0].ChannelID != 2 {
		t.Errorf("team group 应命中渠道 2，实际 %v", rts)
	}
}

// TestSetRemoveIncremental：Set 替换、Remove 移除，互不影响其它渠道。
func TestSetRemoveIncremental(t *testing.T) {
	a := baseChannel(1, "default")
	a.Models = []string{"gpt-4o"}
	b := baseChannel(2, "default")
	b.Models = []string{"gpt-4o"}
	ri := NewRouteIndex()
	ri.Rebuild([]ChannelAbilities{ca(a), ca(b)})

	ri.Remove(1)
	rts, ok := ri.Candidates("default", "gpt-4o")
	if !ok || len(rts) != 1 || rts[0].ChannelID != 2 {
		t.Fatalf("移除渠道 1 后应只剩渠道 2，实际 %v", channelIDs(rts))
	}

	// 重新加入渠道 1，换成不同模型。
	a2 := baseChannel(1, "default")
	a2.Models = []string{"claude-3"}
	ri.Set(ca(a2))
	if rts, ok := ri.Candidates("default", "claude-3"); !ok || len(rts) != 1 || rts[0].ChannelID != 1 {
		t.Errorf("重新加入渠道 1（claude-3）应命中，实际 %v", channelIDs(rts))
	}
	// 渠道 2 不受影响。
	if rts, ok := ri.Candidates("default", "gpt-4o"); !ok || len(rts) != 1 || rts[0].ChannelID != 2 {
		t.Errorf("渠道 2 应不受影响，实际 %v", channelIDs(rts))
	}
}

// TestSetDisabledRemoves：Set 一个禁用渠道等价于移除。
func TestSetDisabledRemoves(t *testing.T) {
	a := baseChannel(1, "default")
	a.Models = []string{"gpt-4o"}
	ri := NewRouteIndex()
	ri.Rebuild([]ChannelAbilities{ca(a)})

	a.Enabled = false
	ri.Set(ca(a))
	if _, ok := ri.Candidates("default", "gpt-4o"); ok {
		t.Error("Set 禁用渠道后不应再命中")
	}
}

// TestPrefixRoutingAndModels：prefix 烘焙进客户端面别名；不带前缀的名不串味；Models 列举别名。
func TestPrefixRoutingAndModels(t *testing.T) {
	c := baseChannel(1, "default")
	c.Prefix = "team1/"
	c.Models = []string{"gpt-4o"}
	c.ModelMapping = map[string]string{"fast*": "gpt-4o-mini*"}
	ri := NewRouteIndex()
	ri.Rebuild([]ChannelAbilities{ca(c)})

	// 含前缀的精确名命中，upstream 不带前缀。
	if rts, ok := ri.Candidates("default", "team1/gpt-4o"); !ok || len(rts) != 1 || rts[0].UpstreamModel != "gpt-4o" {
		t.Fatalf("team1/gpt-4o 应命中→gpt-4o，实际 %v ok=%v", rts, ok)
	}
	// 不带前缀的名不命中（前缀即命名空间，修父设计的碰撞）。
	if _, ok := ri.Candidates("default", "gpt-4o"); ok {
		t.Error("不带前缀的 gpt-4o 不应命中带前缀渠道")
	}
	// 含前缀的通配命中并代入捕获段。
	if rts, ok := ri.Candidates("default", "team1/fast-1"); !ok || rts[0].UpstreamModel != "gpt-4o-mini-1" {
		t.Errorf("team1/fast-1 应命中→gpt-4o-mini-1，实际 %v ok=%v", rts, ok)
	}
	// Models 列举（通配项原样，字典序）。
	if got, want := ri.Models("default"), []string{"team1/fast*", "team1/gpt-4o"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Models 期望 %v，实际 %v", want, got)
	}
}
