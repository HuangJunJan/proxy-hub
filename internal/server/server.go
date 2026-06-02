package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"proxy-hub/internal/admin"
	"proxy-hub/internal/auth"
	"proxy-hub/internal/config"
	"proxy-hub/internal/monitor"
	"proxy-hub/internal/proxy"
	"proxy-hub/internal/store"
	webui "proxy-hub/internal/web"
)

type Options struct {
	Logger        *slog.Logger
	ConfigManager *config.Manager
	Sessions      *auth.SessionManager
	Monitor       *monitor.Service
	Logs          store.RequestLogRepo
	Stats         store.StatsRepo
}

func NewRouter(opts Options) http.Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())
	if opts.ConfigManager != nil {
		r.Use(corsMiddleware(opts.ConfigManager))
	}
	r.Use(requestLogger(opts.Logger))

	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	if opts.ConfigManager != nil && opts.Sessions != nil {
		adminHandler := admin.NewHandler(opts.ConfigManager, opts.Sessions, admin.Dependencies{Logs: opts.Logs, Stats: opts.Stats})
		adminHandler.Register(r.Group("/api/admin"))
		proxyHandler := proxy.NewHandler(opts.ConfigManager, nil, opts.Monitor, opts.Logger)
		v1 := r.Group("/v1")
		v1.Use(requireBearer(opts.ConfigManager))
		proxyHandler.Register(v1)

		openAICompat := r.Group("")
		openAICompat.Use(requireBearer(opts.ConfigManager))
		proxyHandler.Register(openAICompat)
	}

	spa := webui.Handler()
	r.NoRoute(func(c *gin.Context) {
		if isAPIRoute(c.Request.URL.Path) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		spa.ServeHTTP(c.Writer, c.Request)
	})

	return r
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		logger.Info(
			"http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"client_ip", c.ClientIP(),
		)
	}
}

func requireBearer(configManager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := auth.AuthenticateBearer(c.Request, configManager.Snapshot())
		if !ok {
			auth.AbortOpenAIError(
				c,
				http.StatusUnauthorized,
				"Invalid API key provided.",
				"invalid_request_error",
				"invalid_api_key",
			)
			return
		}
		c.Set("api_key_name", principal.Name)
		c.Set("api_key_mask", auth.MaskToken(principal.Token))
		c.Next()
	}
}

func corsMiddleware(configManager *config.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}
		if originAllowed(configManager.Snapshot().CORS.AllowedOrigins, origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func originAllowed(allowed []string, origin string) bool {
	for _, item := range allowed {
		if item == origin {
			return true
		}
	}
	return false
}

func isAPIRoute(path string) bool {
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/")
}
