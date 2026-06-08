// Package openai 实现 OpenAI 方言（/v1/chat/completions、/v1/responses）的同方言透传适配器。
// 仅注入凭证、改写请求体中的 model 名，并把上游响应原样回写客户端，顺带嗅探 usage。
// 跨方言（如 Claude 入站 → OpenAI 上游）由 relay 在调用前经 convert 转换，不在此处处理。
package openai

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

// defaultBaseURL 是官方 OpenAI API 根（base_url 为空时使用）。
const defaultBaseURL = "https://api.openai.com"

// Adaptor 是 OpenAI 平台适配器。
type Adaptor struct{}

func init() { adaptor.Register(Adaptor{}) }

// Platform 返回 openai。
func (Adaptor) Platform() channel.Platform { return channel.PlatformOpenAI }

// BuildRequest 构造发往 OpenAI 兼容上游的请求：拼 URL、改写 model、注入 Bearer 凭证。
// 出口代理与 http.Client 由 relay 持有，本函数只产出 *http.Request。
func (Adaptor) BuildRequest(ctx context.Context, in *adaptor.RelayInput, rt *channel.ChannelRuntime, cred credstore.Cred) (*http.Request, error) {
	base := rt.BaseURL
	if base == "" {
		base = cred.BaseURL
	}
	if base == "" {
		base = defaultBaseURL
	}
	url := joinURL(base, pathFor(in.EndpointFormat))

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
	req.Header.Set("Authorization", "Bearer "+cred.APIKey)
	if in.IsStream {
		req.Header.Set("Accept", "text/event-stream")
	}
	return req, nil
}

// HandleResponse 流式/非流式回写上游响应并解析 usage。
func (Adaptor) HandleResponse(ctx context.Context, resp *http.Response, w http.ResponseWriter, out *adaptor.UsageResult) error {
	adaptor.CopyStatusAndHeaders(w, resp)
	if adaptor.IsEventStream(resp) {
		err := adaptor.StreamCopy(w, resp, func(p []byte) { parseOpenAIUsage(p, out) })
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
		parseOpenAIUsage(body, out)
	} else {
		out.UsageSource = "missing"
	}
	return nil
}

// parseOpenAIUsage 从一段 JSON（完整响应体或带 usage 的流末块）解析 token 用量。
// 仅在 usage 存在时写入，避免后续无 usage 的流块把已解析值清零。
func parseOpenAIUsage(b []byte, out *adaptor.UsageResult) {
	u := gjson.GetBytes(b, "usage")
	if !u.Exists() {
		return
	}
	out.InputTokens = int(u.Get("prompt_tokens").Int())
	out.OutputTokens = int(u.Get("completion_tokens").Int())
	out.CacheReadTokens = int(u.Get("prompt_tokens_details.cached_tokens").Int())
	out.ReasoningTokens = int(u.Get("completion_tokens_details.reasoning_tokens").Int())
}

// pathFor 把入站方言映射到 OpenAI 上游路径。
func pathFor(f adaptor.EndpointFormat) string {
	switch f {
	case adaptor.FormatOpenAIResponses:
		return "/v1/responses"
	default:
		return "/v1/chat/completions"
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
