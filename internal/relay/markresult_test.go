package relay

import (
	"testing"
	"time"
)

var baseTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestClassify(t *testing.T) {
	cases := []struct {
		status  int
		connErr bool
		want    Outcome
	}{
		{200, false, OutcomeSuccess},
		{299, false, OutcomeSuccess},
		{429, false, OutcomeRetryable},
		{500, false, OutcomeRetryable},
		{503, false, OutcomeRetryable},
		{0, true, OutcomeRetryable}, // 连接错误
		{401, false, OutcomeAuthError},
		{403, false, OutcomeAuthError},
		{404, false, OutcomeNotFound},
		{400, false, OutcomeClientError},
		{422, false, OutcomeClientError},
	}
	for _, c := range cases {
		if got := Classify(c.status, c.connErr); got != c.want {
			t.Errorf("Classify(%d, %v) = %d，期望 %d", c.status, c.connErr, got, c.want)
		}
	}
}

func TestComputeHealth429Backoff(t *testing.T) {
	var s HealthState
	s.IsHealthy = true
	// 连续 429：1s, 2s, 4s, 8s ...
	wants := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	for i, want := range wants {
		s = computeHealth(s, OutcomeRetryable, 429, "rate limited", baseTime)
		gotDur := s.CooldownUntil.Sub(baseTime)
		if gotDur != want {
			t.Errorf("第 %d 次 429 冷却应为 %v，实际 %v（连续失败 %d）", i+1, want, gotDur, s.ConsecutiveFailures)
		}
		if s.IsHealthy {
			t.Error("失败后应标记不健康")
		}
	}
}

func TestComputeHealth429Cap(t *testing.T) {
	var s HealthState
	s.ConsecutiveFailures = 20 // 远超 cap
	s = computeHealth(s, OutcomeRetryable, 429, "", baseTime)
	if got := s.CooldownUntil.Sub(baseTime); got != cooldown429Cap {
		t.Errorf("429 退避应封顶 %v，实际 %v", cooldown429Cap, got)
	}
}

func TestComputeHealthDurations(t *testing.T) {
	cases := []struct {
		oc     Outcome
		status int
		want   time.Duration
	}{
		{OutcomeAuthError, 401, cooldownAuth},
		{OutcomeNotFound, 404, cooldownNotFound},
		{OutcomeRetryable, 500, cooldownServer},
	}
	for _, c := range cases {
		s := computeHealth(HealthState{IsHealthy: true}, c.oc, c.status, "", baseTime)
		if got := s.CooldownUntil.Sub(baseTime); got != c.want {
			t.Errorf("outcome %d status %d 冷却应为 %v，实际 %v", c.oc, c.status, c.want, got)
		}
	}
}

func TestComputeHealthSuccessResets(t *testing.T) {
	s := HealthState{IsHealthy: false, ConsecutiveFailures: 5, CooldownUntil: baseTime.Add(time.Hour)}
	s = computeHealth(s, OutcomeSuccess, 200, "", baseTime)
	if !s.IsHealthy || s.ConsecutiveFailures != 0 || !s.CooldownUntil.IsZero() {
		t.Errorf("成功应重置健康，实际 %+v", s)
	}
	if !s.LastSuccessAt.Equal(baseTime) {
		t.Error("成功应更新 LastSuccessAt")
	}
}

func TestComputeHealthClientErrorNoChange(t *testing.T) {
	prev := HealthState{IsHealthy: true, ConsecutiveFailures: 0}
	s := computeHealth(prev, OutcomeClientError, 400, "bad", baseTime)
	if !s.IsHealthy || s.ConsecutiveFailures != 0 || !s.CooldownUntil.IsZero() {
		t.Errorf("客户端错误不应影响渠道健康，实际 %+v", s)
	}
}

func TestHealthMirrorBlocking(t *testing.T) {
	now := baseTime
	h := NewHealthMirror(nil)
	h.now = func() time.Time { return now }

	if h.IsBlocked(1, "gpt-4o") {
		t.Error("初始不应被阻塞")
	}
	h.Mark(1, "gpt-4o", 429, false, "rate limited")
	if !h.IsBlocked(1, "gpt-4o") {
		t.Error("429 后应在冷却窗口内被阻塞")
	}
	// 时间推进超过冷却（429 首次 1s）。
	now = baseTime.Add(2 * time.Second)
	if h.IsBlocked(1, "gpt-4o") {
		t.Error("冷却到期后不应再阻塞")
	}
	// 成功重置。
	h.Mark(1, "gpt-4o", 200, false, "")
	if h.IsBlocked(1, "gpt-4o") {
		t.Error("成功后不应被阻塞")
	}
}

func TestHealthMirrorPersistCalled(t *testing.T) {
	var calls int
	h := NewHealthMirror(func(HealthState) error { calls++; return nil })
	h.Mark(1, "m", 500, false, "boom")
	if calls != 1 {
		t.Errorf("Mark 应调用 persist 一次，实际 %d", calls)
	}
}
