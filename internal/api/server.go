// Package api 提供 Gin HTTP 服务器、中间件装配与基础路由（healthz、静态壳）。
package api

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/huangjunjan/proxy-hub/internal/api/middleware"
	"github.com/huangjunjan/proxy-hub/internal/apikey"
	"github.com/huangjunjan/proxy-hub/internal/buildinfo"
	"github.com/huangjunjan/proxy-hub/internal/config"
	"github.com/huangjunjan/proxy-hub/internal/store"
	"github.com/huangjunjan/proxy-hub/web"
)

// healthCheckTimeout 是 /healthz 中 DB ping 的超时。
const healthCheckTimeout = 3 * time.Second

// Deps 是 HTTP 路由所需的处理器与依赖（由 main 装配后注入）。
type Deps struct {
	Relay    *RelayHandler
	Admin    *AdminHandler
	APIKey   *APIKeyHandler
	Stats    *StatsHandler
	MCP      *MCPHandler
	KeyCache *apikey.Cache
}

// NewServer 构建配置好中间件与路由的 *gin.Engine。
//
// 中间件链顺序：recover → request-id → body-limit。/healthz 不受 auth 约束。
// 路由分两组：/v1/*（中转，入站 key 鉴权）与 /admin/*（管理，admin key 鉴权）。
func NewServer(cfg *config.Config, st *store.Store, deps Deps) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	// 用 gin.New() 而非 gin.Default()，避免默认 logger 噪音；日志走 slog。
	r := gin.New()

	r.Use(middleware.Recover())
	r.Use(middleware.RequestID())
	r.Use(middleware.BodyLimit(cfg.Server.BodyLimit))

	// 健康检查：进程存活 + DB ping。
	r.GET("/healthz", healthzHandler(st))

	// 中转端点：入站 key 鉴权（OpenAI/Claude/Responses 方言 + 模型列表）。
	if deps.Relay != nil {
		v1 := r.Group("/v1")
		v1.Use(middleware.InboundAuth(deps.KeyCache))
		v1.POST("/chat/completions", deps.Relay.ChatCompletions)
		v1.POST("/messages", deps.Relay.Messages)
		v1.POST("/responses", deps.Relay.Responses)
		v1.GET("/models", deps.Relay.Models)
	}

	// 管理端点：admin key 鉴权（渠道 CRUD + 测试，入站 key CRUD）。
	admin := r.Group("/admin")
	admin.Use(middleware.Auth(cfg.AdminKey))
	if deps.Admin != nil {
		admin.GET("/channels", deps.Admin.List)
		admin.POST("/channels", deps.Admin.Create)
		admin.GET("/channels/:id", deps.Admin.Get)
		admin.PUT("/channels/:id", deps.Admin.Update)
		admin.DELETE("/channels/:id", deps.Admin.Delete)
		admin.POST("/channels/:id/test", deps.Admin.Test)
	}
	if deps.APIKey != nil {
		admin.GET("/api-keys", deps.APIKey.List)
		admin.POST("/api-keys", deps.APIKey.Create)
		admin.PUT("/api-keys/:id", deps.APIKey.SetEnabled)
		admin.DELETE("/api-keys/:id", deps.APIKey.Delete)
	}
	if deps.Stats != nil {
		admin.GET("/stats/overview", deps.Stats.Overview)
		admin.GET("/stats/timeseries", deps.Stats.Timeseries)
		admin.GET("/stats/breakdown", deps.Stats.Breakdown)
		admin.GET("/stats/logs", deps.Stats.Logs)
		admin.GET("/stats/health", deps.Stats.Health)
		admin.GET("/pricing", deps.Stats.ListPricing)
		admin.PUT("/pricing/:model", deps.Stats.UpsertPricing)
		admin.DELETE("/pricing/:model", deps.Stats.DeletePricing)
	}

	// MCP 共享管理端点（admin key 鉴权；与 /admin/* 同守护，见父设计 §5）。
	if deps.MCP != nil {
		m := r.Group("/v0/mcp")
		m.Use(middleware.Auth(cfg.AdminKey))
		m.GET("/servers", deps.MCP.ListServers)
		m.POST("/servers", deps.MCP.CreateServer)
		m.GET("/servers/:id", deps.MCP.GetServer)
		m.PUT("/servers/:id", deps.MCP.UpdateServer)
		m.DELETE("/servers/:id", deps.MCP.DeleteServer)
		m.PUT("/servers/:id/toggle", deps.MCP.ToggleServer)
		m.GET("/targets", deps.MCP.ListTargets)
		m.POST("/targets", deps.MCP.CreateTarget)
		m.PUT("/targets/:id", deps.MCP.UpdateTarget)
		m.DELETE("/targets/:id", deps.MCP.DeleteTarget)
		m.POST("/sync", deps.MCP.SyncAll)
		m.POST("/sync/:id", deps.MCP.SyncTarget)
		m.POST("/import/:id", deps.MCP.ImportTarget)
		m.POST("/import-bundle", deps.MCP.ImportBundle)
	}

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
