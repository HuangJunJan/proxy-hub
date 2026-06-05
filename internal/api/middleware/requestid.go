package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// requestIDHeader 是请求/响应中携带请求 ID 的 HTTP 头。
const requestIDHeader = "X-Request-Id"

// contextKeyRequestID 是 gin.Context 中存放请求 ID 的键。
const contextKeyRequestID = "request_id"

// RequestID 读取入站 X-Request-Id，缺失则生成一个，注入 gin 上下文与响应头。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(requestIDHeader)
		if rid == "" {
			rid = generateRequestID()
		}
		c.Set(contextKeyRequestID, rid)
		c.Header(requestIDHeader, rid)
		c.Next()
	}
}

// RequestIDFromContext 从 gin 上下文取出请求 ID；不存在时返回空串。
func RequestIDFromContext(c *gin.Context) string {
	if v, ok := c.Get(contextKeyRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// generateRequestID 生成 16 字节随机十六进制请求 ID。
func generateRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// rand 失败极罕见；退化为固定占位，不阻塞请求。
		return "req-unknown"
	}
	return hex.EncodeToString(buf)
}
