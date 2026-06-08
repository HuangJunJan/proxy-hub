package relay

import (
	"log/slog"
	"sync"
	"time"
)

// 冷却常量（按 (渠道,模型) 的 HTTP 状态驱动）。
const (
	cooldown429Base  = 1 * time.Second  // 429 指数退避基数
	cooldown429Cap   = 5 * time.Minute  // 429 退避上限
	cooldownAuth     = 30 * time.Minute // 401/403：凭证问题
	cooldownNotFound = 12 * time.Hour   // 404/不支持
	cooldownServer   = 1 * time.Minute  // 5xx / 连接错误
)

// Outcome 是一次上游调用的结果分类，驱动重试与冷却决策。
type Outcome int

const (
	OutcomeSuccess     Outcome = iota // 2xx
	OutcomeRetryable                  // 429 / 5xx / 连接错误 → 冷却 + 换渠道重试
	OutcomeAuthError                  // 401/403 → 较长冷却，不重试同渠道
	OutcomeNotFound                   // 404/不支持 → 很长冷却
	OutcomeClientError                // 其它 4xx → 客户端错误：不冷却、不重试
)

// Classify 把 HTTP 状态/连接错误映射为 Outcome。
func Classify(statusCode int, connErr bool) Outcome {
	if connErr {
		return OutcomeRetryable
	}
	switch {
	case statusCode >= 200 && statusCode < 300:
		return OutcomeSuccess
	case statusCode == 429:
		return OutcomeRetryable
	case statusCode == 401, statusCode == 403:
		return OutcomeAuthError
	case statusCode == 404:
		return OutcomeNotFound
	case statusCode >= 500:
		return OutcomeRetryable
	case statusCode >= 400:
		return OutcomeClientError
	default:
		return OutcomeRetryable
	}
}

// Retryable 报告该结果是否应换渠道重试。
func (o Outcome) Retryable() bool { return o == OutcomeRetryable }

// HealthState 是 (渠道,模型) 的健康状态，镜像 channel_model_health 行。
type HealthState struct {
	ChannelID           int64
	Model               string
	IsHealthy           bool
	ConsecutiveFailures int
	LastSuccessAt       time.Time // 零值表示无
	LastFailureAt       time.Time
	LastError           string
	CooldownUntil       time.Time // 零值/过去表示未冷却
	UpdatedAt           time.Time
}

// computeHealth 依据结果分类 + 既往状态，算出新的健康状态。
//   - 成功：重置连续失败、清冷却、置健康。
//   - 429：指数退避（base×2^(失败数-1)，封顶 cap）。
//   - 401/403：30min；404：12h；5xx/连接：1min。
//   - 其它 4xx：客户端错误，不改变健康（调用方一般不应为此调用 Mark）。
func computeHealth(prev HealthState, oc Outcome, statusCode int, errMsg string, now time.Time) HealthState {
	next := prev
	next.UpdatedAt = now

	if oc == OutcomeSuccess {
		next.IsHealthy = true
		next.ConsecutiveFailures = 0
		next.CooldownUntil = time.Time{}
		next.LastSuccessAt = now
		next.LastError = ""
		return next
	}
	if oc == OutcomeClientError {
		// 客户端错误不反映渠道健康：保持原状，仅更新时间戳。
		return next
	}

	next.ConsecutiveFailures = prev.ConsecutiveFailures + 1
	next.LastFailureAt = now
	next.IsHealthy = false
	next.LastError = errMsg

	var d time.Duration
	switch oc {
	case OutcomeAuthError:
		d = cooldownAuth
	case OutcomeNotFound:
		d = cooldownNotFound
	case OutcomeRetryable:
		if statusCode == 429 {
			d = backoff(cooldown429Base, cooldown429Cap, next.ConsecutiveFailures)
		} else {
			d = cooldownServer
		}
	default:
		d = cooldownServer
	}
	next.CooldownUntil = now.Add(d)
	return next
}

// backoff 计算指数退避：base × 2^(failures-1)，封顶 cap。failures<=1 返回 base。
func backoff(base, cap time.Duration, failures int) time.Duration {
	d := base
	for i := 1; i < failures; i++ {
		d *= 2
		if d >= cap {
			return cap
		}
	}
	if d > cap {
		return cap
	}
	return d
}

// healthKey 是健康镜像的键。
type healthKey struct {
	channelID int64
	model     string
}

// HealthMirror 是 channel_model_health 的内存镜像：MarkResult 写入并同步持久化，selector 读它过滤冷却。
type HealthMirror struct {
	mu      sync.RWMutex
	states  map[healthKey]HealthState
	persist func(HealthState) error // 注入：落库（dbgen UpsertChannelModelHealth）；nil 表示不持久化
	now     func() time.Time
}

// NewHealthMirror 创建镜像。persist 可为 nil（仅内存，用于测试）。
func NewHealthMirror(persist func(HealthState) error) *HealthMirror {
	return &HealthMirror{
		states:  map[healthKey]HealthState{},
		persist: persist,
		now:     time.Now,
	}
}

// Load 用启动时从 DB 读出的状态装配镜像。
func (h *HealthMirror) Load(states []HealthState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range states {
		h.states[healthKey{s.ChannelID, s.Model}] = s
	}
}

// IsBlocked 报告 (渠道,模型) 当前是否在冷却中（cooldown_until > now）。
func (h *HealthMirror) IsBlocked(channelID int64, model string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s, ok := h.states[healthKey{channelID, model}]
	if !ok {
		return false
	}
	return s.CooldownUntil.After(h.now())
}

// Mark 记录一次结果：更新内存状态并（若配置）持久化。
func (h *HealthMirror) Mark(channelID int64, model string, statusCode int, connErr bool, errMsg string) {
	oc := Classify(statusCode, connErr)
	now := h.now()

	h.mu.Lock()
	key := healthKey{channelID, model}
	prev, ok := h.states[key]
	if !ok {
		prev = HealthState{ChannelID: channelID, Model: model, IsHealthy: true}
	}
	next := computeHealth(prev, oc, statusCode, errMsg, now)
	h.states[key] = next
	h.mu.Unlock()

	if h.persist != nil {
		if err := h.persist(next); err != nil {
			slog.Warn("持久化渠道健康失败", "channel_id", channelID, "model", model, "error", err)
		}
	}
}
