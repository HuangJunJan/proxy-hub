package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/huangjunjan/proxy-hub/internal/adaptor"
	"github.com/huangjunjan/proxy-hub/internal/api/middleware"
	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/relay"
)

// RelayHandler 持有中转引擎与路由索引，提供统一中转端点（/v1/*）。
type RelayHandler struct {
	engine *relay.Engine
	index  *channel.RouteIndex
}

// NewRelayHandler 创建中转处理器。
func NewRelayHandler(engine *relay.Engine, index *channel.RouteIndex) *RelayHandler {
	return &RelayHandler{engine: engine, index: index}
}

// ChatCompletions 处理 POST /v1/chat/completions（OpenAI chat 方言）。
func (h *RelayHandler) ChatCompletions(c *gin.Context) { h.serve(c, adaptor.FormatOpenAIChat) }

// Messages 处理 POST /v1/messages（Claude messages 方言）。
func (h *RelayHandler) Messages(c *gin.Context) { h.serve(c, adaptor.FormatClaudeMessages) }

// Responses 处理 POST /v1/responses（OpenAI Responses 方言）。
func (h *RelayHandler) Responses(c *gin.Context) { h.serve(c, adaptor.FormatOpenAIResponses) }

// serve 是三个 POST 端点的公共流程：读体 → 提取 model/stream → 构造 RelayInput → 交引擎编排。
func (h *RelayHandler) serve(c *gin.Context, format adaptor.EndpointFormat) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "读取请求体失败", "type": "invalid_request"}})
		return
	}
	model := gjson.GetBytes(body, "model").String()
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "缺少 model 字段", "type": "invalid_request"}})
		return
	}
	in := &adaptor.RelayInput{
		EndpointFormat: format,
		RequestedModel: model,
		Group:          middleware.GetGroup(c),
		APIKeyID:       middleware.GetAPIKeyID(c),
		SessionID:      sessionID(c),
		IsStream:       gjson.GetBytes(body, "stream").Bool(),
		Body:           body,
		Header:         c.Request.Header,
	}
	h.engine.Serve(c.Request.Context(), c.Writer, in)
}

// Models 处理 GET /v1/models：从 RouteIndex 按入站 key 的 group 列举可用别名（OpenAI 形态）。
func (h *RelayHandler) Models(c *gin.Context) {
	models := h.index.Models(middleware.GetGroup(c))
	data := make([]gin.H, 0, len(models))
	for _, m := range models {
		data = append(data, gin.H{"id": m, "object": "model", "owned_by": "proxy-hub"})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

// sessionID 取会话亲和键：优先 X-Session-Id 头；缺失返回空（内容哈希兜底留待 M5）。
func sessionID(c *gin.Context) string {
	return strings.TrimSpace(c.GetHeader("X-Session-Id"))
}
