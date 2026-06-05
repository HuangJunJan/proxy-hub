// Package api 提供 Gin HTTP 服务器、中间件装配与基础路由（healthz、静态壳）。
package api

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/huangjunjan/proxy-hub/internal/api/middleware"
	"github.com/huangjunjan/proxy-hub/internal/buildinfo"
	"github.com/huangjunjan/proxy-hub/internal/config"
	"github.com/huangjunjan/proxy-hub/internal/store"
	"github.com/huangjunjan/proxy-hub/web"
)

// healthCheckTimeout 是 /healthz 中 DB ping 的超时。
const healthCheckTimeout = 3 * time.Second

// NewServer 构建配置好中间件与路由的 *gin.Engine。
//
// 中间件链顺序：recover → request-id → body-limit。auth 雏形挂在受保护路由组上
// （M1 暂未注册 /admin/*、/v0/*，故此处仅保留骨架）。/healthz 不受 auth 约束。
func NewServer(cfg *config.Config, st *store.Store) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	// 用 gin.New() 而非 gin.Default()，避免默认 logger 噪音；日志走 slog。
	r := gin.New()

	r.Use(middleware.Recover())
	r.Use(middleware.RequestID())
	r.Use(middleware.BodyLimit(cfg.Server.BodyLimit))

	// 健康检查：进程存活 + DB ping。
	r.GET("/healthz", healthzHandler(st))

	// 受保护路由组骨架（M2 起注册真实路由）。M1 仅装配 auth 中间件占位。
	protected := r.Group("/")
	protected.Use(middleware.Auth(cfg.AdminKey))
	// M1 此处无子路由：/admin/*、/v0/* 由后续里程碑注册。
	_ = protected

	// 静态 SPA 壳 + history 模式回退。
	if err := registerStatic(r); err != nil {
		return nil, err
	}

	return r, nil
}

// healthzHandler 返回 /healthz 的处理函数。DB 正常 → 200；DB 异常 → 503。
func healthzHandler(st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
		defer cancel()
		if err := st.HealthCheck(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "error",
				"version": buildinfo.Version,
				"db":      "error",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": buildinfo.Version,
			"db":      "ok",
		})
	}
}

// registerStatic 注册内嵌 SPA 的静态文件服务，并把未命中的非 API 路由回退到 index.html。
func registerStatic(r *gin.Engine) error {
	// 取 dist 子树作为静态资源根。
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(distFS))

	// NoRoute：API 路由优先匹配；其余交给静态服务，缺失文件回退到 index.html（SPA history 模式）。
	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		// 若请求的静态文件存在则直接服务，否则回退到 index.html。
		if reqPath != "/" {
			if f, err := distFS.Open(trimLeadingSlash(reqPath)); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		// 回退：渲染 index.html。
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
	return nil
}

// trimLeadingSlash 去掉路径前导斜杠，适配 fs.FS 的相对路径要求。
func trimLeadingSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		return p[1:]
	}
	return p
}
