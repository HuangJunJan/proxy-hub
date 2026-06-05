package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// bearerPrefix 是 Authorization 头中 Bearer 方案前缀。
const bearerPrefix = "Bearer "

// Auth 是管理端鉴权中间件雏形。
//
// M1 行为：仅作为可插拔骨架供 M2 扩展。若 adminKey 已设置，则对受保护路由组校验
// `Authorization: Bearer <adminKey>`；不匹配返回 401。adminKey 为空时直接放行
// （M1 尚未注册 /admin/*、/v0/* 路由，故等价于仅 /healthz 可用）。
//
// 注意：/healthz 不应挂载本中间件——它在中间件链之外或单独放行。
func Auth(adminKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// M1 雏形：未配置 adminKey 时不做强制鉴权（真实鉴权 M2 接管）。
		if adminKey == "" {
			c.Next()
			return
		}
		token := extractBearer(c.GetHeader("Authorization"))
		if token == "" || token != adminKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			return
		}
		c.Next()
	}
}

// extractBearer 从 Authorization 头中提取 Bearer token；非 Bearer 方案返回空串。
func extractBearer(header string) string {
	if !strings.HasPrefix(header, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
}
