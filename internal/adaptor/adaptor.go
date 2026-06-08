// Package adaptor 定义上游适配契约：把入站请求构造为发往上游的 HTTP 请求，并把上游响应
// 回写客户端、解析 usage。具体实现见子包 openai/、claude/，经 Register 注册（relay 不直接 import 实现，避免环）。
package adaptor

import (
	"context"
	"net/http"

	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/credstore"
)

// EndpointFormat 是入站端点方言。
type EndpointFormat string

const (
	FormatOpenAIChat      EndpointFormat = "openai"    // /v1/chat/completions
	FormatClaudeMessages  EndpointFormat = "claude"    // /v1/messages
	FormatOpenAIResponses EndpointFormat = "responses" // /v1/responses
)

// RelayInput 是一次中转的入站上下文（鉴权 + 提取后，路由 + 适配前填充 UpstreamModel）。
type RelayInput struct {
	EndpointFormat EndpointFormat
	RequestedModel string      // 客户端面模型名（含 prefix，保留用于统计）
	UpstreamModel  string      // 解析出的上游模型名（relay 在 BuildRequest 前写入）
	Group          string      // 来自入站 key
	APIKeyID       int64       // 入站 key id
	SessionID      string      // 会话亲和键
	IsStream       bool        // 是否流式
	Body           []byte      // 原始入站请求体（模型名经 gjson 提取/改写）
	Header         http.Header // 入站头（按需透传，绝不透传鉴权头到上游日志）
}

// UsageResult 是适配器从上游响应解析出的 token 用量（成本由 M3 在读取时计算）。
type UsageResult struct {
	InputTokens         int
	OutputTokens        int
	ReasoningTokens     int
	CacheReadTokens     int
	CacheCreationTokens int
	// UsageSource 取值 stream | usage_block | estimated | missing。
	UsageSource string
}

// Adaptor 是单个平台/方言的上游适配器。同方言透传；跨方言由 relay 在调用前经 convert 转换。
type Adaptor interface {
	// Platform 返回本适配器服务的上游平台。
	Platform() channel.Platform
	// BuildRequest 构造发往上游的请求（注入凭证、改写模型名、设置出口代理由 relay 的 client 处理）。
	BuildRequest(ctx context.Context, in *RelayInput, rt *channel.ChannelRuntime, cred credstore.Cred) (*http.Request, error)
	// HandleResponse 把上游响应流式/非流式回写到 w，并把解析到的用量填入 out。
	HandleResponse(ctx context.Context, resp *http.Response, w http.ResponseWriter, out *UsageResult) error
}

// registry 是平台 → 适配器的注册表。实现包在 init 或显式 wiring 中 Register。
var registry = map[channel.Platform]Adaptor{}

// Register 注册一个平台适配器（同平台后注册者覆盖）。
func Register(a Adaptor) {
	registry[a.Platform()] = a
}

// Get 返回指定平台的适配器。
func Get(p channel.Platform) (Adaptor, bool) {
	a, ok := registry[p]
	return a, ok
}
