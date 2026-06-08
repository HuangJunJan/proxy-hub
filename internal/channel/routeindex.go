package channel

import (
	"sort"
	"strings"
	"sync"
)

// RouteIndex 是请求期的内存路由索引：把 (group, 客户端面模型) 解析为候选 ChannelRuntime 列表，
// 热路径不碰 DB。解析顺序：精确别名 → 最长 * 通配 → （透传以精确别名形式预先展开，故已被覆盖）。
//
// 维护原始的 per-channel 数据（meta + abilities），任一渠道变更时在写锁下重建派生查找表
// （渠道数量小，整体重建廉价且避免增量 bug；DB 侧仍是增量 upsert/delete，绝不 TRUNCATE）。
type RouteIndex struct {
	mu        sync.RWMutex
	metaByCh  map[int64]channelMeta
	abilsByCh map[int64][]Ability
	groups    map[string]*groupIndex // 派生
}

// channelMeta 是构造 ChannelRuntime 所需的渠道级元数据（不含凭证）。
type channelMeta struct {
	platform Platform
	chType   ChannelType
	baseURL  string
	proxyURL string
}

// groupIndex 是单个 group 的派生查找表。
type groupIndex struct {
	exact map[string][]candidate // alias_model（非通配）-> 候选
	wild  []wildEntry            // 通配项，按前缀长度降序（最长匹配优先）
}

type candidate struct {
	channelID     int64
	upstreamModel string
	priority      int
	weight        int
}

type wildEntry struct {
	prefix          string // alias 去掉尾部 * 后的前缀
	upstreamPattern string // 可能含尾部 *
	channelID       int64
	priority        int
	weight          int
}

// ChannelAbilities 把一个渠道与其展开的 abilities 配对，作为 RouteIndex 的输入单元。
type ChannelAbilities struct {
	Channel   Channel
	Abilities []Ability
}

// NewRouteIndex 创建空索引。
func NewRouteIndex() *RouteIndex {
	return &RouteIndex{
		metaByCh:  map[int64]channelMeta{},
		abilsByCh: map[int64][]Ability{},
		groups:    map[string]*groupIndex{},
	}
}

// Rebuild 用全量渠道重建索引（启动时用）。禁用渠道被忽略。
func (ri *RouteIndex) Rebuild(items []ChannelAbilities) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.metaByCh = map[int64]channelMeta{}
	ri.abilsByCh = map[int64][]Ability{}
	for _, it := range items {
		if !it.Channel.Enabled {
			continue
		}
		ri.storeLocked(it)
	}
	ri.rebuildDerivedLocked()
}

// Set 增量加入/替换一个渠道（渠道保存时用）。禁用渠道等价于 Remove。
func (ri *RouteIndex) Set(item ChannelAbilities) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	delete(ri.metaByCh, item.Channel.ID)
	delete(ri.abilsByCh, item.Channel.ID)
	if item.Channel.Enabled {
		ri.storeLocked(item)
	}
	ri.rebuildDerivedLocked()
}

// Remove 移除一个渠道（删除时用）。
func (ri *RouteIndex) Remove(channelID int64) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	delete(ri.metaByCh, channelID)
	delete(ri.abilsByCh, channelID)
	ri.rebuildDerivedLocked()
}

// storeLocked 记录单渠道的 meta 与（仅启用的）abilities。调用方须持写锁。
func (ri *RouteIndex) storeLocked(it ChannelAbilities) {
	ri.metaByCh[it.Channel.ID] = channelMeta{
		platform: it.Channel.Platform,
		chType:   it.Channel.Type,
		baseURL:  it.Channel.BaseURL,
		proxyURL: it.Channel.ProxyURL,
	}
	enabled := make([]Ability, 0, len(it.Abilities))
	for _, a := range it.Abilities {
		if a.Enabled {
			enabled = append(enabled, a)
		}
	}
	ri.abilsByCh[it.Channel.ID] = enabled
}

// rebuildDerivedLocked 从原始 per-channel 数据重建 groups 派生表。调用方须持写锁。
func (ri *RouteIndex) rebuildDerivedLocked() {
	groups := map[string]*groupIndex{}
	for chID, abils := range ri.abilsByCh {
		for _, a := range abils {
			gi := groups[a.Group]
			if gi == nil {
				gi = &groupIndex{exact: map[string][]candidate{}}
				groups[a.Group] = gi
			}
			if IsWildcard(a.AliasModel) {
				gi.wild = append(gi.wild, wildEntry{
					prefix:          strings.TrimSuffix(a.AliasModel, "*"),
					upstreamPattern: a.UpstreamModel,
					channelID:       chID,
					priority:        a.Priority,
					weight:          a.Weight,
				})
			} else {
				gi.exact[a.AliasModel] = append(gi.exact[a.AliasModel], candidate{
					channelID:     chID,
					upstreamModel: a.UpstreamModel,
					priority:      a.Priority,
					weight:        a.Weight,
				})
			}
		}
	}
	// 每个 group 的通配项按前缀长度降序，便于最长匹配时尽早命中并剪枝。
	for _, gi := range groups {
		sort.SliceStable(gi.wild, func(i, j int) bool {
			return len(gi.wild[i].prefix) > len(gi.wild[j].prefix)
		})
	}
	ri.groups = groups
}

// Candidates 解析 (group, requested) 得候选运行期列表。requested 是客户端面模型名（含 prefix，
// prefix 已在 BuildAbilities 烘焙进 alias，故此处无需再剥离）；调用方可在未命中时用剥离 [1M] 后缀的名再查一次。
// 命中精确别名则返回其全部候选；否则取最长匹配的通配别名（同长度可多渠道）；都不命中返回 ok=false。
func (ri *RouteIndex) Candidates(group, requested string) ([]*ChannelRuntime, bool) {
	ri.mu.RLock()
	defer ri.mu.RUnlock()

	gi := ri.groups[group]
	if gi == nil {
		return nil, false
	}

	// 1) 精确别名。
	if cs := gi.exact[requested]; len(cs) > 0 {
		out := make([]*ChannelRuntime, 0, len(cs))
		for _, c := range cs {
			if rt := ri.runtimeLocked(c.channelID, c.upstreamModel, c.priority, c.weight); rt != nil {
				out = append(out, rt)
			}
		}
		if len(out) > 0 {
			return out, true
		}
	}

	// 2) 最长 * 通配（wild 已按前缀长度降序）。
	bestLen := -1
	var out []*ChannelRuntime
	for _, w := range gi.wild {
		if bestLen >= 0 && len(w.prefix) < bestLen {
			break // 已收集到更长前缀的匹配，剩余更短，剪枝
		}
		if !strings.HasPrefix(requested, w.prefix) {
			continue
		}
		if bestLen < 0 {
			bestLen = len(w.prefix)
		}
		captured := requested[len(w.prefix):]
		upstream := SubstituteWildcard(w.upstreamPattern, captured)
		if rt := ri.runtimeLocked(w.channelID, upstream, w.priority, w.weight); rt != nil {
			out = append(out, rt)
		}
	}
	if len(out) > 0 {
		return out, true
	}
	return nil, false
}

// runtimeLocked 由渠道 ID + 解析出的上游名构造 ChannelRuntime。调用方须持读/写锁。
func (ri *RouteIndex) runtimeLocked(channelID int64, upstreamModel string, priority, weight int) *ChannelRuntime {
	meta, ok := ri.metaByCh[channelID]
	if !ok {
		return nil
	}
	return &ChannelRuntime{
		ChannelID:     channelID,
		UpstreamModel: upstreamModel,
		Priority:      priority,
		Weight:        weight,
		Platform:      meta.platform,
		Type:          meta.chType,
		BaseURL:       meta.baseURL,
		ProxyURL:      meta.proxyURL,
	}
}

// Models 返回某 group 下全部客户端面别名（通配项原样如 "gpt-4*"），按字典序排序。供 /v1/models 列举。
func (ri *RouteIndex) Models(group string) []string {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	gi := ri.groups[group]
	if gi == nil {
		return nil
	}
	set := make(map[string]struct{}, len(gi.exact)+len(gi.wild))
	for alias := range gi.exact {
		set[alias] = struct{}{}
	}
	for _, w := range gi.wild {
		set[w.prefix+"*"] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}
