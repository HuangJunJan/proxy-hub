package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// OriginCheck 对改状态请求（非 GET/HEAD/OPTIONS）做同源/白名单校验，防 CSRF（与 bearer 鉴权叠加）。
//
// 规则：
//   - 只读方法（GET/HEAD/OPTIONS）放行。
//   - 有 Origin（或退化用 Referer）时：须与请求 Host 同源，或在 allowed 白名单内，否则 403。
//   - 无 Origin/Referer（如 curl/CLI/服务端调用）：bearer 已强制鉴权，放行（CSRF 是浏览器场景威胁）。
func OriginCheck(allowed []string) gin.HandlerFunc {
	allowSet := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		if t := strings.TrimSpace(a); t != "" {
			allowSet[t] = true
		}
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = refererOrigin(c.GetHeader("Referer"))
		}
		if origin == "" {
			// 非浏览器客户端（无 Origin）：bearer 鉴权已足够，放行。
			c.Next()
			return
		}
		if allowSet[origin] || sameOrigin(origin, c.Request.Host) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "跨域请求被拒绝（origin 校验未通过）"})
	}
}

// sameOrigin 报告 origin 的 host(:port) 是否与请求 Host 相同。
func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Host == host
}

// refererOrigin 从 Referer 提取 scheme://host 形式的 origin（失败返回空）。
func refererOrigin(ref string) string {
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
