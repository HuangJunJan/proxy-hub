// Package selector 在候选渠道中选择一个：冷却过滤 → 会话亲和 → 最高优先级档 → 加权随机。
// 设计为 dbgen 无关：只消费 channel.ChannelRuntime，冷却判断经注入的 BlockFunc。
package selector

import (
	"errors"
	mrand "math/rand/v2"
	"sync"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/channel"
)

// ErrNoCandidate 表示没有可用候选（候选为空或全部在冷却）。
var ErrNoCandidate = errors.New("无可用渠道（候选为空或全部冷却）")

// BlockFunc 报告 (渠道, 上游模型) 当前是否被冷却阻塞。
type BlockFunc func(channelID int64, upstreamModel string) bool

// defaultAffinityTTL 是会话亲和的默认存活时长。
const defaultAffinityTTL = 10 * time.Minute

type affEntry struct {
	channelID int64
	expireAt  time.Time
}

// Selector 持有会话亲和缓存与可注入的时钟/随机源（便于确定性测试）。
type Selector struct {
	mu        sync.Mutex
	affinity  map[string]affEntry
	ttl       time.Duration
	now       func() time.Time
	randFloat func() float64 // 返回 [0,1)
}

// New 创建选择器（默认时钟 time.Now、默认随机源 math/rand/v2）。
func New() *Selector {
	return &Selector{
		affinity:  map[string]affEntry{},
		ttl:       defaultAffinityTTL,
		now:       time.Now,
		randFloat: mrand.Float64,
	}
}

// Pick 从候选中选择一个渠道。isBlocked 为 nil 时视为均不阻塞。
func (s *Selector) Pick(candidates []*channel.ChannelRuntime, sessionID string, isBlocked BlockFunc) (*channel.ChannelRuntime, error) {
	// 1. 冷却过滤。
	avail := make([]*channel.ChannelRuntime, 0, len(candidates))
	for _, c := range candidates {
		if isBlocked == nil || !isBlocked(c.ChannelID, c.UpstreamModel) {
			avail = append(avail, c)
		}
	}
	if len(avail) == 0 {
		return nil, ErrNoCandidate
	}

	// 2. 会话亲和：若该会话上次成功的渠道仍在可用集内，优先复用。
	if sessionID != "" {
		if chID, ok := s.getAffinity(sessionID); ok {
			for _, c := range avail {
				if c.ChannelID == chID {
					return c, nil
				}
			}
		}
	}

	// 3. 最高优先级档。
	top := avail[0].Priority
	for _, c := range avail {
		if c.Priority > top {
			top = c.Priority
		}
	}
	tier := make([]*channel.ChannelRuntime, 0, len(avail))
	for _, c := range avail {
		if c.Priority == top {
			tier = append(tier, c)
		}
	}

	// 4. 档内加权随机。
	return s.weightedPick(tier), nil
}

// weightedPick 在同优先级档内按 weight 加权随机；weight<=0 视为 1。
func (s *Selector) weightedPick(cands []*channel.ChannelRuntime) *channel.ChannelRuntime {
	if len(cands) == 1 {
		return cands[0]
	}
	total := 0
	for _, c := range cands {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	r := int(s.randFloat() * float64(total))
	if r >= total {
		r = total - 1
	}
	for _, c := range cands {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		if r < w {
			return c
		}
		r -= w
	}
	return cands[len(cands)-1]
}

// getAffinity 读取会话亲和（过期则清除）。
func (s *Selector) getAffinity(sessionID string) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.affinity[sessionID]
	if !ok {
		return 0, false
	}
	if s.now().After(e.expireAt) {
		delete(s.affinity, sessionID)
		return 0, false
	}
	return e.channelID, true
}

// RecordAffinity 在一次成功调用后记录会话→渠道亲和（由 relay 在成功后调用，失败渠道不粘附）。
func (s *Selector) RecordAffinity(sessionID string, channelID int64) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.affinity[sessionID] = affEntry{channelID: channelID, expireAt: s.now().Add(s.ttl)}
}
