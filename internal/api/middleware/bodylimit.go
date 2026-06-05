package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// defaultBodyLimit 是请求体大小默认上限：32MB。
const defaultBodyLimit int64 = 32 << 20

// BodyLimit 限制请求体大小；超限返回 413。limit <= 0 时使用默认 32MB。
//
// 实现用 http.MaxBytesReader 包裹 Body：读取超限时返回错误，由后续 handler 感知。
// 对已声明 Content-Length 且超限的请求，提前拒绝。
func BodyLimit(limit int64) gin.HandlerFunc {
	if limit <= 0 {
		limit = defaultBodyLimit
	}
	return func(c *gin.Context) {
		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "request body too large",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
