package relay

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/huangjunjan/proxy-hub/internal/adaptor"
	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/credstore"
	"github.com/huangjunjan/proxy-hub/internal/selector"
)

// fakeAdaptor 是测试用适配器：把请求打到 rt.BaseURL，回写响应体并填固定 usage。
type fakeAdaptor struct{}

func (fakeAdaptor) Platform() channel.Platform { return channel.Platform("fake") }

func (fakeAdaptor) BuildRequest(ctx context.Context, in *adaptor.RelayInput, rt *channel.ChannelRuntime, cred credstore.Cred) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodPost, rt.BaseURL, nil)
}

func (fakeAdaptor) HandleResponse(ctx context.Context, resp *http.Response, w http.ResponseWriter, out *adaptor.UsageResult) error {
	adaptor.CopyStatusAndHeaders(w, resp)
	if _, err := adaptor.BufferCopy(w, resp); err != nil {
		return err
	}
	out.InputTokens = 10
	out.OutputTokens = 20
	out.UsageSource = "usage_block"
	return nil
}

// fakeCreds 满足 credGetter。
type fakeCreds map[int64]credstore.Cred

func (f fakeCreds) Get(id int64) (credstore.Cred, bool) {
	c, ok := f[id]
	return c, ok
}

func init() { adaptor.Register(fakeAdaptor{}) }

// newTestEngine 构造一个候选为指定渠道的引擎。
func newTestEngine(t *testing.T, chans []channel.ChannelAbilities, creds fakeCreds) (*Engine, *Emitter) {
	t.Helper()
	idx := channel.NewRouteIndex()
	idx.Rebuild(chans)
	em := NewEmitter(16)
	eng := NewEngine(Config{
		Index:      idx,
		Selector:   selector.New(),
		Health:     NewHealthMirror(nil),
		Emitter:    em,
		Creds:      creds,
		MaxRetries: 2,
	})
	return eng, em
}

func chanWith(id int64, group, alias, upstream, baseURL string, priority int) channel.ChannelAbilities {
	return channel.ChannelAbilities{
		Channel: channel.Channel{
			ID: id, Enabled: true, Platform: channel.Platform("fake"),
			Type: channel.TypeAPIKey, Group: group, BaseURL: baseURL, Priority: priority, Weight: 1,
		},
		Abilities: []channel.Ability{{
			Group: group, AliasModel: alias, ChannelID: id, UpstreamModel: upstream,
			Priority: priority, Weight: 1, Enabled: true,
		}},
	}
}

func TestServeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	eng, em := newTestEngine(t,
		[]channel.ChannelAbilities{chanWith(1, "default", "gpt-4o", "gpt-4o", srv.URL, 50)},
		fakeCreds{1: {APIKey: "x"}},
	)

	rec := httptest.NewRecorder()
	in := &adaptor.RelayInput{
		EndpointFormat: adaptor.FormatOpenAIChat, RequestedModel: "gpt-4o",
		Group: "default", Body: []byte(`{"model":"gpt-4o"}`),
	}
	eng.Serve(context.Background(), rec, in)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200", rec.Code)
	}
	ev := <-em.Events()
	if ev.ChannelID != 1 || ev.InputTokens != 10 || ev.OutputTokens != 20 || ev.TotalTokens != 30 {
		t.Fatalf("用量事件不符: %+v", ev)
	}
	if ev.IsError {
		t.Fatalf("不应标记错误")
	}
}

func TestServeFailover(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer good.Close()

	// bad 优先级更高 → 先被选中并失败，随后排除转到 good。
	eng, em := newTestEngine(t, []channel.ChannelAbilities{
		chanWith(1, "default", "gpt-4o", "gpt-4o", bad.URL, 100),
		chanWith(2, "default", "gpt-4o", "gpt-4o", good.URL, 50),
	}, fakeCreds{1: {APIKey: "x"}, 2: {APIKey: "y"}})

	rec := httptest.NewRecorder()
	in := &adaptor.RelayInput{
		EndpointFormat: adaptor.FormatOpenAIChat, RequestedModel: "gpt-4o",
		Group: "default", Body: []byte(`{"model":"gpt-4o"}`),
	}
	eng.Serve(context.Background(), rec, in)

	if rec.Code != http.StatusOK {
		t.Fatalf("故障转移后状态码 = %d，期望 200", rec.Code)
	}
	ev := <-em.Events()
	if ev.ChannelID != 2 {
		t.Fatalf("应转移到渠道 2，实际 %d", ev.ChannelID)
	}
}

func TestServeModelNotFound(t *testing.T) {
	eng, em := newTestEngine(t, nil, fakeCreds{})
	rec := httptest.NewRecorder()
	in := &adaptor.RelayInput{
		EndpointFormat: adaptor.FormatOpenAIChat, RequestedModel: "nope",
		Group: "default", Body: []byte(`{"model":"nope"}`),
	}
	eng.Serve(context.Background(), rec, in)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d，期望 404", rec.Code)
	}
	ev := <-em.Events()
	if !ev.IsError || ev.ErrorType != "model_not_found" {
		t.Fatalf("应为 model_not_found 错误事件: %+v", ev)
	}
}

func TestServeCrossDialectDisabled(t *testing.T) {
	// 入站 OpenAI 方言，唯一候选为 anthropic 平台渠道；跨方言开关默认关 ⇒ 干净拒绝（501）。
	ch := channel.ChannelAbilities{
		Channel: channel.Channel{
			ID: 1, Enabled: true, Platform: channel.PlatformAnthropic,
			Type: channel.TypeAPIKey, Group: "default", BaseURL: "http://unused.invalid", Priority: 50, Weight: 1,
		},
		Abilities: []channel.Ability{{
			Group: "default", AliasModel: "gpt-4o", ChannelID: 1, UpstreamModel: "gpt-4o",
			Priority: 50, Weight: 1, Enabled: true,
		}},
	}
	eng, em := newTestEngine(t, []channel.ChannelAbilities{ch}, fakeCreds{1: {APIKey: "x"}})

	rec := httptest.NewRecorder()
	in := &adaptor.RelayInput{
		EndpointFormat: adaptor.FormatOpenAIChat, RequestedModel: "gpt-4o",
		Group: "default", Body: []byte(`{"model":"gpt-4o"}`),
	}
	eng.Serve(context.Background(), rec, in)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("状态码 = %d，期望 501（跨方言未启用应干净拒绝）", rec.Code)
	}
	ev := <-em.Events()
	if !ev.IsError || ev.ErrorType != "cross_dialect_disabled" {
		t.Fatalf("应为 cross_dialect_disabled 错误事件: %+v", ev)
	}
}

func TestServeContextSuffixFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	// 别名按无后缀注册；请求带 [1M]，应剥离后兜底命中。
	eng, em := newTestEngine(t,
		[]channel.ChannelAbilities{chanWith(1, "default", "claude-sonnet-4", "claude-sonnet-4", srv.URL, 50)},
		fakeCreds{1: {APIKey: "x"}},
	)
	rec := httptest.NewRecorder()
	in := &adaptor.RelayInput{
		EndpointFormat: adaptor.FormatClaudeMessages, RequestedModel: "claude-sonnet-4[1M]",
		Group: "default", Body: []byte(`{"model":"claude-sonnet-4[1M]"}`),
	}
	eng.Serve(context.Background(), rec, in)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200（剥离 [1M] 兜底命中）", rec.Code)
	}
	ev := <-em.Events()
	if ev.RequestedModel != "claude-sonnet-4[1M]" {
		t.Fatalf("requested_model 应保留客户端原名（含 [1M]）: %q", ev.RequestedModel)
	}
}
