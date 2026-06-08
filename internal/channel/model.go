// Package channel 提供渠道领域类型、模型映射解析与内存路由索引（RouteIndex）。
//
// 与 DB 行解耦：DB 行（dbgen 生成）↔ 领域类型的转换在 dao.go。本文件只含纯领域逻辑，
// 不依赖生成代码，便于单测。
package channel

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Platform 是上游平台/方言。
type Platform string

const (
	PlatformOpenAI    Platform = "openai"
	PlatformAnthropic Platform = "anthropic"
)

// ChannelType 是渠道类型：官方 api_key 或自定义 upstream。
type ChannelType string

const (
	TypeAPIKey   ChannelType = "api_key"
	TypeUpstream ChannelType = "upstream"
)

// Channel 是渠道的领域表示（凭证不在此，见 credstore）。
type Channel struct {
	ID           int64
	Name         string
	Enabled      bool
	Platform     Platform
	Type         ChannelType
	BaseURL      string
	Group        string // 对应 DB 列 group_name
	Priority     int
	Weight       int
	Models       []string          // 解析自 models JSON：本渠道支持的上游模型名
	ModelMapping map[string]string // 解析自 model_mapping JSON：alias_model -> upstream_model（支持尾部 *）
	Prefix       string
	ProxyURL     string
	Status       string
	ErrorMessage string
}

// Ability 是派生路由索引的一行（客户端面 alias -> 上游 upstream，绑定到渠道）。
type Ability struct {
	Group         string
	AliasModel    string
	ChannelID     int64
	UpstreamModel string
	Priority      int
	Weight        int
	Enabled       bool
}

// ChannelRuntime 是 RouteIndex 中的运行期候选（不含凭证）。
type ChannelRuntime struct {
	ChannelID     int64
	UpstreamModel string
	Priority      int
	Weight        int
	Platform      Platform
	Type          ChannelType
	BaseURL       string
	ProxyURL      string
}

// ParseModels 解析 models JSON 数组（空串/空 JSON 视为空列表）。
func ParseModels(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析 models JSON 失败: %w", err)
	}
	return out, nil
}

// ParseModelMapping 解析 model_mapping JSON 对象（空串/空 JSON 视为空映射）。
func ParseModelMapping(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("解析 model_mapping JSON 失败: %w", err)
	}
	return out, nil
}

// EncodeModels 把模型列表编码为 JSON（nil/空 → "[]"）。
func EncodeModels(models []string) (string, error) {
	if len(models) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(models)
	if err != nil {
		return "", fmt.Errorf("编码 models 失败: %w", err)
	}
	return string(b), nil
}

// EncodeModelMapping 把映射编码为 JSON（nil/空 → "{}"）。
func EncodeModelMapping(m map[string]string) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("编码 model_mapping 失败: %w", err)
	}
	return string(b), nil
}

// StripPrefix 在 model 以 prefix 开头时去掉前缀；prefix 为空或不匹配时原样返回。
// 第二返回值表示是否命中前缀。
func StripPrefix(model, prefix string) (string, bool) {
	if prefix == "" || !strings.HasPrefix(model, prefix) {
		return model, false
	}
	return strings.TrimPrefix(model, prefix), true
}

// StripContextSuffix 去掉尾部的长上下文标注（形如 "[1M]"）。
// 例：claude-sonnet-4[1M] -> claude-sonnet-4。无此后缀则原样返回（客户端面原名仍用于统计）。
func StripContextSuffix(model string) string {
	if !strings.HasSuffix(model, "]") {
		return model
	}
	idx := strings.LastIndex(model, "[")
	if idx <= 0 { // 找不到 '[' 或它在首位（整串都是括号）则不处理
		return model
	}
	return model[:idx]
}

// IsWildcard 判断别名模式是否为尾部通配（形如 "gpt-4*"）。
func IsWildcard(pattern string) bool {
	return strings.HasSuffix(pattern, "*")
}

// MatchWildcard 在 pattern 为尾部通配时尝试匹配 requested。
// 返回：被 * 捕获的剩余段、用于"最长匹配"比较的前缀长度、是否匹配。
// 非通配 pattern 一律返回 ok=false（精确匹配由调用方单独处理）。
func MatchWildcard(pattern, requested string) (captured string, prefixLen int, ok bool) {
	if !IsWildcard(pattern) {
		return "", 0, false
	}
	prefix := strings.TrimSuffix(pattern, "*")
	if !strings.HasPrefix(requested, prefix) {
		return "", 0, false
	}
	return requested[len(prefix):], len(prefix), true
}

// SubstituteWildcard 把 upstream 模式尾部的 * 替换为捕获段；无 * 则原样返回。
// 例：SubstituteWildcard("azure-*", "-turbo") -> "azure--turbo"；("fixed", x) -> "fixed"。
func SubstituteWildcard(upstreamPattern, captured string) string {
	if !strings.HasSuffix(upstreamPattern, "*") {
		return upstreamPattern
	}
	return strings.TrimSuffix(upstreamPattern, "*") + captured
}

// BuildAbilities 把一个渠道的 model_mapping + models 展开为 abilities（供保存时写 DB + RouteIndex）。
//   - model_mapping 的每个 alias->upstream 生成一条（alias/upstream 可含尾部 *）。
//   - models 中未作为 mapping 键出现的，生成一条透传 ability（alias=upstream=模型名）。
//   - 渠道 prefix（命名空间）烘焙进**客户端面**别名（alias = prefix+原别名），upstream 不带前缀。
//     这样客户端发送的名字即含 prefix（与统计口径一致），且不同渠道的同名模型不会因去前缀而相互串味，
//     修掉父设计的命名空间碰撞——请求期因此无需再剥离 prefix。
//
// 输出按 alias_model 排序，保证确定性（便于增量对比与测试）。
func BuildAbilities(c Channel) []Ability {
	seen := make(map[string]struct{}, len(c.ModelMapping)+len(c.Models))
	abilities := make([]Ability, 0, len(c.ModelMapping)+len(c.Models))

	add := func(alias, upstream string) {
		if alias == "" {
			return
		}
		full := c.Prefix + alias // 客户端面别名含命名空间前缀
		if _, dup := seen[full]; dup {
			return
		}
		seen[full] = struct{}{}
		abilities = append(abilities, Ability{
			Group:         c.Group,
			AliasModel:    full,
			ChannelID:     c.ID,
			UpstreamModel: upstream,
			Priority:      c.Priority,
			Weight:        c.Weight,
			Enabled:       c.Enabled,
		})
	}

	for alias, upstream := range c.ModelMapping {
		add(alias, upstream)
	}
	for _, m := range c.Models {
		add(m, m) // 透传：客户端面名即上游名
	}

	sort.Slice(abilities, func(i, j int) bool {
		return abilities[i].AliasModel < abilities[j].AliasModel
	})
	return abilities
}
