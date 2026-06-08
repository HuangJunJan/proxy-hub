// Package claude 实现 Anthropic Claude 方言（/v1/messages）的同方言透传适配器。
// 仅注入凭证、改写请求体中的 model 名，并把上游响应原样回写客户端，顺带嗅探 usage。
// 跨方言（如 OpenAI 入站 → Claude 上游）由 relay 在调用前经 convert 转换，不在此处处理。
package claude

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/huangjunjan/proxy-hub/internal/adaptor"
	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/credstore"
)

// defaultBaseURL 是官方 Anthropic API 根（base_url 为空时使用）。
const defaultBaseURL = "https://api.anthropic.com"

// anthropicVersion 是必带的 API 版本头。
const anthropicVersion = "2023-06-01"

// Adaptor 是 Anthropic 平台适配器。
type Adaptor struct{}

func init() { adaptor.Register(Adaptor{}) }

// Platform 返回 anthropic。
func (Adaptor) Platform() channel.Platform { return channel.PlatformAnthropic }

// BuildRequest 构造发往 Claude 兼容上游的请求：拼 URL、改写 model、注入 x-api-key 凭证。
func (Adaptor) BuildRequest(ctx context.Context, in *adaptor.RelayInput, rt *channel.ChannelRuntime, cred credstore.Cred) (*http.Request, error) {
	base := rt.BaseURL
	if base == "" {
		base = cred.BaseURL
	}
	if base == "" {
		base = defaultBaseURL
	}
	url := joinURL(base, "/v1/messages")

	body := in.Body
	if in.UpstreamModel != "" {
		if nb, err := sjson.SetBytes(body, "model", in.UpstreamModel); err == nil {
			body = nb
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cred.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	if in.IsStream {
		req.Header.Set("Accept", "text/event-stream")
	}
	return req, nil
}

// HandleResponse 流式/非流式回写上游响应并解析 usage。
func (Adaptor) HandleResponse(ctx context.Context, resp *http.Response, w http.ResponseWriter, out *adaptor.UsageResult) error {
	adaptor.CopyStatusAndHeaders(w, resp)
	if adaptor.IsEventStream(resp) {
		err := adaptor.StreamCopy(w, resp, func(p []byte) { parseClaudeStream(p, out) })
		if out.InputTokens == 0 && out.OutputTokens == 0 {
			out.UsageSource = "missing"
		} else {
			out.UsageSource = "stream"
		}
		return err
	}
	body, err := adaptor.BufferCopy(w, resp)
	if err != nil {
		return err
	}
	if gjson.GetBytes(body, "usage").Exists() {
		out.UsageSource = "usage_block"
		parseClaudeFull(body, out)
	} else {
		out.UsageSource = "missing"
	}
	return nil
}

// parseClaudeFull 从非流式完整响应体解析 token 用量。
func parseClaudeFull(b []byte, out *adaptor.UsageResult) {
	u := gjson.GetBytes(b, "usage")
	if !u.Exists() {
		return
	}
	out.InputTokens = int(u.Get("input_tokens").Int())
	out.OutputTokens = int(u.Get("output_tokens").Int())
	out.CacheReadTokens = int(u.Get("cache_read_input_tokens").Int())
	out.CacheCreationTokens = int(u.Get("cache_creation_input_tokens").Int())
}

// parseClaudeStream 从 SSE 事件负载解析用量：message_start 带输入侧，message_delta 带输出侧。
func parseClaudeStream(p []byte, out *adaptor.UsageResult) {
	switch gjson.GetBytes(p, "type").String() {
	case "message_start":
		u := gjson.GetBytes(p, "message.usage")
		if u.Exists() {
			out.InputTokens = int(u.Get("input_tokens").Int())
			out.CacheReadTokens = int(u.Get("cache_read_input_tokens").Int())
			out.CacheCreationTokens = int(u.Get("cache_creation_input_tokens").Int())
		}
	case "message_delta":
		u := gjson.GetBytes(p, "usage")
		if u.Exists() {
			out.OutputTokens = int(u.Get("output_tokens").Int())
		}
	}
}

// joinURL 拼接 base 与标准子路径；当 base 已含尾部 /v1 而子路径也以 /v1 开头时，去重避免 /v1/v1。
func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if strings.HasPrefix(path, "/v1") && strings.HasSuffix(base, "/v1") {
		base = strings.TrimSuffix(base, "/v1")
	}
	return base + path
}
