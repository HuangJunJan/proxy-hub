// Package middleware 提供 Gin 中间件：recover、request-id、body-limit、auth 雏形。
package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Recover 捕获 handler 中的 panic，记录结构化日志并返回 500，避免进程崩溃。
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("处理请求时发生 panic",
					"panic", rec,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"request_id", RequestIDFromContext(c),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			}
		}()
		c.Next()
	}
}
