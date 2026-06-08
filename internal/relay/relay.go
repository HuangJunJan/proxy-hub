package relay

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/adaptor"
	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/credstore"
	"github.com/huangjunjan/proxy-hub/internal/selector"
	"github.com/huangjunjan/proxy-hub/internal/usage"
)

// Engine 编排一次中转。所有协作者经构造注入，使引擎本身 dbgen 无关、可单测：
// RouteIndex 给候选、selector 选渠道、HealthMirror 管冷却、credstore 取凭证、Emitter 发用量。
type Engine struct {
	index    *channel.RouteIndex
	selector *selector.Selector
	health   *HealthMirror
	emitter  *usage.Emitter
	creds    credGetter

	maxRetries   int
	crossDialect bool

	mu          sync.Mutex
	clientCache map[string]*http.Client // proxy_url（""=直连/随环境）-> 复用的 client

	now   func() time.Time
	newID func() string
}

// credGetter 抽象凭证读取（credstore.Store 满足），便于测试注入。
type credGetter interface {
	Get(channelID int64) (credstore.Cred, bool)
}

// Config 是 Engine 的装配参数。
type Config struct {
	Index              *channel.RouteIndex
	Selector           *selector.Selector
	Health             *HealthMirror
	Emitter            *usage.Emitter
	Creds              credGetter
	MaxRetries         int  // <=0 时取默认 2
	EnableCrossDialect bool // 跨方言转换特性开关（默认 false；见 design §10/§14）
}

// NewEngine 创建中转引擎。
func NewEngine(cfg Config) *Engine {
	mr := cfg.MaxRetries
	if mr <= 0 {
		mr = 2
	}
	return &Engine{
		index:        cfg.Index,
		selector:     cfg.Selector,
		health:       cfg.Health,
		emitter:      cfg.Emitter,
		creds:        cfg.Creds,
		maxRetries:   mr,
		crossDialect: cfg.EnableCrossDialect,
		clientCache:  map[string]*http.Client{},
		now:          time.Now,
		newID:        defaultRequestID,
	}
}

// Serve 编排一次中转：路由解析 → 选择器选渠道 → 适配器构造上游请求 → 调用（带跨渠道重试）→
// 回写客户端 → 发 UsageEvent + Mark 健康。函数返回时必定发出一条 UsageEvent（无密钥/请求体）。
func (e *Engine) Serve(ctx context.Context, w http.ResponseWriter, in *adaptor.RelayInput) {
	start := e.now()
	ev := usage.Event{
		RequestID:      e.newID(),
		RequestedModel: in.RequestedModel,
		Group:          in.Group,
		EndpointFormat: string(in.EndpointFormat),
		APIKeyID:       in.APIKeyID,
		IsStream:       in.IsStream,
		SessionID:      in.SessionID,
		CreatedAt:      start,
	}
	defer func() {
		ev.LatencyMS = e.now().Sub(start).Milliseconds()
		ev.TotalTokens = ev.InputTokens + ev.OutputTokens
		e.emitter.Emit(ev)
	}()

	// 1. 路由解析：先用客户端原名（含 prefix）；未命中再用剥离 [1M] 长上下文后缀的名兜底。
	candidates, ok := e.index.Candidates(in.Group, in.RequestedModel)
	if !ok {
		if stripped := channel.StripContextSuffix(in.RequestedModel); stripped != in.RequestedModel {
			candidates, ok = e.index.Candidates(in.Group, stripped)
		}
	}
	if !ok || len(candidates) == 0 {
		ev.IsError, ev.StatusCode, ev.ErrorType = true, http.StatusNotFound, "model_not_found"
		writeError(w, in.EndpointFormat, http.StatusNotFound, "model_not_found", "无可用渠道服务该模型")
		return
	}

	// 1b. 跨方言守门：转换器特性未启用时，仅保留与入站方言同方言的候选（避免把不匹配的请求体
	// 透传给上游导致 4xx）。候选全为跨方言 ⇒ 干净拒绝（design §10/§14：套件未过则只发布同方言路由）。
	if !e.crossDialect {
		same := make([]*channel.ChannelRuntime, 0, len(candidates))
		for _, c := range candidates {
			if !isCrossDialect(in.EndpointFormat, c.Platform) {
				same = append(same, c)
			}
		}
		if len(same) == 0 {
			ev.IsError, ev.StatusCode, ev.ErrorType = true, http.StatusNotImplemented, "cross_dialect_disabled"
			writeError(w, in.EndpointFormat, http.StatusNotImplemented, "cross_dialect_disabled",
				"请求方言与渠道平台不一致，且跨方言转换未启用（relay.enable_cross_dialect=false）")
			return
		}
		candidates = same
	}

	// 2. 重试循环：每轮选一个候选；可重试失败则排除该渠道后重选，至多 maxRetries 次。
	tried := map[int64]bool{}
	var lastStatus int
	var lastErrType string

	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		pick, err := e.pickExcluding(candidates, in.SessionID, tried)
		if err != nil {
			break // 无更多可用候选
		}
		tried[pick.ChannelID] = true

		cred, credOK := e.creds.Get(pick.ChannelID)
		if !credOK {
			e.health.Mark(pick.ChannelID, pick.UpstreamModel, http.StatusUnauthorized, false, "缺少凭证")
			lastStatus, lastErrType = http.StatusUnauthorized, "missing_credential"
			continue
		}

		ad, adOK := adaptor.Get(pick.Platform)
		if !adOK {
			lastStatus, lastErrType = http.StatusInternalServerError, "no_adaptor"
			slog.Error("缺少平台适配器", "platform", pick.Platform, "channel_id", pick.ChannelID)
			continue
		}

		in.UpstreamModel = pick.UpstreamModel
		req, err := ad.BuildRequest(ctx, in, pick, cred)
		if err != nil {
			lastStatus, lastErrType = http.StatusInternalServerError, "build_request"
			slog.Error("构造上游请求失败", "channel_id", pick.ChannelID, "error", err)
			continue
		}

		resp, err := e.clientFor(pick.ProxyURL).Do(req)
		if err != nil {
			// 连接错误：可重试。
			e.health.Mark(pick.ChannelID, pick.UpstreamModel, 0, true, truncErr(err))
			lastStatus, lastErrType = http.StatusBadGateway, "upstream_connection"
			slog.Warn("上游连接失败", "channel_id", pick.ChannelID, "error", truncErr(err))
			continue
		}

		oc := Classify(resp.StatusCode, false)
		var errMsg string
		if oc != OutcomeSuccess {
			errMsg = "上游返回 " + strconv.Itoa(resp.StatusCode)
		}
		e.health.Mark(pick.ChannelID, pick.UpstreamModel, resp.StatusCode, false, errMsg)

		if oc.Retryable() && attempt < e.maxRetries {
			// 可重试且仍有重试次数：丢弃响应体并换渠道。
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastStatus, lastErrType = resp.StatusCode, "upstream_retryable"
			continue
		}

		// 终态：成功 / 不可重试 / 重试用尽 → 把（最后这个）上游响应回写客户端。
		ev.ChannelID = pick.ChannelID
		ev.UpstreamModel = pick.UpstreamModel
		ev.StatusCode = resp.StatusCode

		var ur adaptor.UsageResult
		herr := ad.HandleResponse(ctx, resp, w, &ur)
		_ = resp.Body.Close()
		fillUsage(&ev, ur)

		if oc == OutcomeSuccess {
			e.selector.RecordAffinity(in.SessionID, pick.ChannelID)
		} else {
			ev.IsError, ev.ErrorType = true, "upstream_error"
		}
		if herr != nil {
			slog.Warn("回写上游响应失败", "request_id", ev.RequestID, "channel_id", pick.ChannelID, "error", herr)
		}
		return
	}

	// 重试循环未产生任何终态响应（全部连接失败 / 无凭证 / 候选耗尽）。
	status := lastStatus
	if status == 0 {
		status = http.StatusServiceUnavailable
	}
	if lastErrType == "" {
		lastErrType = "no_candidate"
	}
	ev.IsError, ev.StatusCode, ev.ErrorType = true, status, lastErrType
	writeError(w, in.EndpointFormat, status, lastErrType, "所有候选渠道均不可用")
}

// pickExcluding 用 selector 选一个候选，跳过已尝试的渠道与正在冷却的 (渠道,模型)。
func (e *Engine) pickExcluding(candidates []*channel.ChannelRuntime, sessionID string, tried map[int64]bool) (*channel.ChannelRuntime, error) {
	block := func(channelID int64, upstreamModel string) bool {
		if tried[channelID] {
			return true
		}
		return e.health.IsBlocked(channelID, upstreamModel)
	}
	return e.selector.Pick(candidates, sessionID, block)
}

// isCrossDialect 报告入站方言与渠道平台是否跨方言。仅对已知平台（openai/anthropic）判定；
// 未知/兼容平台一律视为同方言，交由透传适配器处理（也使测试用 fake 平台不被误拦）。
func isCrossDialect(format adaptor.EndpointFormat, platform channel.Platform) bool {
	switch platform {
	case channel.PlatformOpenAI:
		return format == adaptor.FormatClaudeMessages
	case channel.PlatformAnthropic:
		return format == adaptor.FormatOpenAIChat || format == adaptor.FormatOpenAIResponses
	default:
		return false
	}
}

// clientFor 返回（并缓存）某出口代理对应的 http.Client。proxyURL 为空时随环境代理（标准 Go 行为）。
func (e *Engine) clientFor(proxyURL string) *http.Client {
	e.mu.Lock()
	defer e.mu.Unlock()
	if c, ok := e.clientCache[proxyURL]; ok {
		return c
	}
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		Proxy:               http.ProxyFromEnvironment,
	}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		} else {
			slog.Warn("解析渠道出口代理失败，回退随环境", "proxy_url", proxyURL, "error", err)
		}
	}
	// Timeout 置 0：流式响应可长时间持续，由调用方 ctx（客户端断开即取消）控制生命周期。
	c := &http.Client{Transport: transport}
	e.clientCache[proxyURL] = c
	return c
}

// writeError 按入站方言写出错误响应体（OpenAI 风格 vs Claude 风格）。仅在尚未写出响应时调用。
func writeError(w http.ResponseWriter, format adaptor.EndpointFormat, status int, errType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	var payload any
	if format == adaptor.FormatClaudeMessages {
		payload = map[string]any{
			"type":  "error",
			"error": map[string]string{"type": errType, "message": msg},
		}
	} else {
		payload = map[string]any{
			"error": map[string]any{"message": msg, "type": errType, "code": nil},
		}
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// fillUsage 把适配器解析出的用量拷入事件。
func fillUsage(ev *usage.Event, u adaptor.UsageResult) {
	ev.InputTokens = u.InputTokens
	ev.OutputTokens = u.OutputTokens
	ev.ReasoningTokens = u.ReasoningTokens
	ev.CacheReadTokens = u.CacheReadTokens
	ev.CacheCreationTokens = u.CacheCreationTokens
	ev.UsageSource = u.UsageSource
}

// truncErr 把错误转为可存储的短串（连接错误信息不含密钥——key 在请求头而非 URL）。
func truncErr(err error) string {
	if err == nil {
		return ""
	}
	const max = 256
	s := err.Error()
	if len(s) > max {
		s = s[:max]
	}
	return s
}

// defaultRequestID 生成 16 字节随机十六进制请求 ID。
func defaultRequestID() string {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		return "req-unknown"
	}
	return hex.EncodeToString(b[:])
}
