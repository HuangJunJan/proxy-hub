// Package health 提供可选的主动健康探测：按渠道×模型定时发最小探针，记 health_check_logs，
// 并复用 relay 的健康标记（成功重置/失败冷却）。默认关闭（仅 health.enabled=true 时由 main 启动）。
// 独立 goroutine + 独立 http client，绝不阻塞中转热路径。
package health

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/adaptor"
	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/credstore"
)

// credGetter 抽象凭证读取（credstore.Store 满足）。
type credGetter interface {
	Get(channelID int64) (credstore.Cred, bool)
}

// MarkFunc 记录一次探测结果到健康镜像（relay.HealthMirror.Mark 满足）。
type MarkFunc func(channelID int64, model string, statusCode int, connErr bool, errMsg string)

// probeResult 是单次探测结论。
type probeResult struct {
	success bool
	status  int
	ms      int64
	msg     string
	connErr bool
}

// Prober 是主动健康探测器。
type Prober struct {
	dao      *channel.DAO
	creds    credGetter
	mark     MarkFunc
	interval time.Duration
	timeout  time.Duration
	now      func() time.Time
	probeFn  func(ctx context.Context, ch channel.Channel, model string) probeResult // 可注入（测试）
}

// Config 是探测器装配参数。
type Config struct {
	DAO      *channel.DAO
	Creds    credGetter
	Mark     MarkFunc
	Interval time.Duration
	Timeout  time.Duration
}

// NewProber 创建探测器（interval<=0 取 5m，timeout<=0 取 20s）。
func NewProber(cfg Config) *Prober {
	p := &Prober{
		dao: cfg.DAO, creds: cfg.Creds, mark: cfg.Mark,
		interval: cfg.Interval, timeout: cfg.Timeout, now: time.Now,
	}
	if p.interval <= 0 {
		p.interval = 5 * time.Minute
	}
	if p.timeout <= 0 {
		p.timeout = 20 * time.Second
	}
	p.probeFn = p.doProbe
	return p
}

// Run 启动周期探测，返回 done（goroutine 退出后 close）。ctx 取消即停。
func (p *Prober) Run(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		p.ProbeOnce(ctx) // 启动即探一轮
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.ProbeOnce(ctx)
			}
		}
	}()
	return done
}

// ProbeOnce 对所有启用渠道 × 其模型探一轮。
func (p *Prober) ProbeOnce(ctx context.Context) {
	chans, err := p.dao.ListChannels(ctx)
	if err != nil {
		slog.Warn("健康探测：列出渠道失败", "error", err)
		return
	}
	for _, ch := range chans {
		if !ch.Enabled {
			continue
		}
		for _, model := range probeModels(ch) {
			p.probe(ctx, ch, model)
		}
	}
}

func (p *Prober) probe(ctx context.Context, ch channel.Channel, model string) {
	res := p.probeFn(ctx, ch, model)
	if err := p.dao.InsertHealthCheck(ctx, channel.HealthCheck{
		ChannelID: ch.ID, Model: model, Success: res.success, HTTPStatus: res.status,
		ResponseTimeMS: res.ms, Message: res.msg, CheckedAt: p.now().UTC().Format(time.RFC3339),
	}); err != nil {
		slog.Warn("健康探测：写入记录失败", "channel_id", ch.ID, "model", model, "error", err)
	}
	if p.mark != nil {
		p.mark(ch.ID, model, res.status, res.connErr, res.msg)
	}
}

// doProbe 发一个 max_tokens=1 的最小探针（默认实现）。
func (p *Prober) doProbe(ctx context.Context, ch channel.Channel, model string) probeResult {
	cred, has := p.creds.Get(ch.ID)
	if !has {
		return probeResult{msg: "缺少凭证"}
	}
	ad, ok := adaptor.Get(ch.Platform)
	if !ok {
		return probeResult{msg: "无平台适配器"}
	}
	in := &adaptor.RelayInput{
		EndpointFormat: formatForPlatform(ch.Platform),
		UpstreamModel:  model,
		IsStream:       false,
		Body:           probeBody(model),
	}
	rt := &channel.ChannelRuntime{
		ChannelID: ch.ID, UpstreamModel: model, Platform: ch.Platform, Type: ch.Type,
		BaseURL: ch.BaseURL, ProxyURL: ch.ProxyURL,
	}
	cctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	req, err := ad.BuildRequest(cctx, in, rt, cred)
	if err != nil {
		return probeResult{msg: "构造请求失败: " + err.Error()}
	}
	start := p.now()
	resp, err := probeClient(ch.ProxyURL, p.timeout).Do(req)
	ms := p.now().Sub(start).Milliseconds()
	if err != nil {
		return probeResult{ms: ms, msg: truncate(err.Error(), 200), connErr: true}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return probeResult{success: true, status: resp.StatusCode, ms: ms}
	}
	return probeResult{status: resp.StatusCode, ms: ms, msg: "上游返回 " + strconv.Itoa(resp.StatusCode)}
}

func probeModels(ch channel.Channel) []string {
	if len(ch.Models) > 0 {
		return ch.Models
	}
	seen := map[string]bool{}
	var out []string
	for _, up := range ch.ModelMapping {
		m := channel.StripContextSuffix(up)
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

func formatForPlatform(p channel.Platform) adaptor.EndpointFormat {
	if p == channel.PlatformAnthropic {
		return adaptor.FormatClaudeMessages
	}
	return adaptor.FormatOpenAIChat
}

func probeBody(model string) []byte {
	return []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"ping"}],"max_tokens":1}`)
}

func probeClient(proxyURL string, timeout time.Duration) *http.Client {
	tr := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
