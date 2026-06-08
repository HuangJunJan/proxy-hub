package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/huangjunjan/proxy-hub/internal/apikey"
)

// bearerPrefix 是 Authorization 头中 Bearer 方案前缀。
const bearerPrefix = "Bearer "

// 注入 gin.Context 的入站身份键（relay handlers 经 GetAPIKeyID/GetGroup 读取）。
const (
	ctxAPIKeyID = "api_key_id"
	ctxGroup    = "group"
)

// Auth 是管理端鉴权中间件（守护 /admin/*）。
//
// adminKey 为空时直接放行（未配置即不强制，便于本地起步）；否则用常量时间比较校验
// `Authorization: Bearer <adminKey>`，不匹配返回 401。/healthz 不挂本中间件。
func Auth(adminKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminKey == "" {
			c.Next()
			return
		}
		token := extractBearer(c.GetHeader("Authorization"))
		if subtle.ConstantTimeCompare([]byte(token), []byte(adminKey)) != 1 {
			abortUnauthorized(c)
			return
		}
		c.Next()
	}
}

// InboundAuth 是中转端点（/v1/*）的入站 key 鉴权中间件。
//
// 从 `Authorization: Bearer` 或 `x-api-key` 取明文 → sha256 → 查缓存（含负缓存）→
// 命中且 enabled 放行并注入 api_key_id/group；否则 401。
func InboundAuth(cache *apikey.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		plain := extractInboundKey(c)
		if plain == "" {
			abortUnauthorized(c)
			return
		}
		meta, ok := cache.Lookup(apikey.HashKey(plain))
		if !ok || !meta.Enabled {
			abortUnauthorized(c)
			return
		}
		c.Set(ctxAPIKeyID, meta.ID)
		c.Set(ctxGroup, meta.Group)
		c.Next()
	}
}

// GetAPIKeyID 读取 InboundAuth 注入的入站 key id（缺失返回 0）。
func GetAPIKeyID(c *gin.Context) int64 {
	v, ok := c.Get(ctxAPIKeyID)
	if !ok {
		return 0
	}
	id, _ := v.(int64)
	return id
}

// GetGroup 读取 InboundAuth 注入的 group（缺失返回空串）。
func GetGroup(c *gin.Context) string {
	v, ok := c.Get(ctxGroup)
	if !ok {
		return ""
	}
	g, _ := v.(string)
	return g
}

// extractBearer 从 Authorization 头中提取 Bearer token；非 Bearer 方案返回空串。
func extractBearer(header string) string {
	if !strings.HasPrefix(header, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
}

// extractInboundKey 依次尝试 Authorization: Bearer 与 x-api-key 头。
func extractInboundKey(c *gin.Context) string {
	if t := extractBearer(c.GetHeader("Authorization")); t != "" {
		return t
	}
	return strings.TrimSpace(c.GetHeader("x-api-key"))
}

// abortUnauthorized 以 401 终止请求。
func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{"message": "未授权：缺失或无效的 API key", "type": "unauthorized"},
	})
}
