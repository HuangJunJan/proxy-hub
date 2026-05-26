package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-hub/internal/auth"
	"proxy-hub/internal/config"
	"proxy-hub/internal/store"
	"proxy-hub/internal/upstream"
	"proxy-hub/internal/upstream/openai"
)

type Handler struct {
	config   *config.Manager
	sessions *auth.SessionManager
	logs     store.RequestLogRepo
	stats    store.StatsRepo
	probe    upstream.Adapter
}

type Dependencies struct {
	Logs  store.RequestLogRepo
	Stats store.StatsRepo
	Probe upstream.Adapter
}

func NewHandler(configManager *config.Manager, sessions *auth.SessionManager, deps ...Dependencies) *Handler {
	h := &Handler{config: configManager, sessions: sessions, probe: openai.New(nil)}
	if len(deps) > 0 {
		h.logs = deps[0].Logs
		h.stats = deps[0].Stats
		if deps[0].Probe != nil {
			h.probe = deps[0].Probe
		}
	}
	return h
}

func (h *Handler) Register(r gin.IRouter) {
	r.GET("/setup/status", h.setupStatus)
	r.POST("/setup", h.setup)
	r.POST("/login", h.login)
	protected := r.Group("", h.requireSession())
	protected.POST("/logout", h.logout)
	protected.GET("/me", h.me)
	protected.GET("/channels", h.listChannels)
	protected.POST("/channels/probe-models", h.probeModels)
	protected.POST("/channels/:type", h.createChannel)
	protected.PUT("/channels/:type/:name", h.updateChannel)
	protected.DELETE("/channels/:type/:name", h.deleteChannel)
	protected.POST("/channels/:type/:name/health", h.healthCheck)
	protected.GET("/keys", h.listKeys)
	protected.POST("/keys", h.createKey)
	protected.PATCH("/keys/:id", h.updateKey)
	protected.DELETE("/keys/:id", h.deleteKey)
	protected.GET("/logs", h.listLogs)
	protected.GET("/stats/channels", h.channelStats)
	protected.GET("/stats/series", h.statsSeries)
}

func (h *Handler) RequireSession() gin.HandlerFunc {
	return h.requireSession()
}

func (h *Handler) setupStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"needed": h.config.SetupNeeded()})
}

type setupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) setup(c *gin.Context) {
	if !h.config.SetupNeeded() {
		c.JSON(http.StatusConflict, gin.H{"error": "setup already completed"})
		return
	}
	var req setupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password with at least 6 characters are required"})
		return
	}
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	token, err := auth.GenerateProxyAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}
	if err := h.config.Save(func(cfg *config.Config) error {
		cfg.Admin = &config.AdminConfig{Username: username, PasswordHash: passwordHash}
		cfg.APIKeys = append(cfg.APIKeys, config.APIKeyConfig{Token: token, Name: "default"})
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	session, err := h.sessions.Issue(username, 30*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue session"})
		return
	}
	h.sessions.SetCookie(c.Writer, session)
	c.JSON(http.StatusOK, gin.H{"token": token})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) login(c *gin.Context) {
	cfg := h.config.Snapshot()
	if cfg.Admin == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "setup required"})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Username) != cfg.Admin.Username {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	ok, err := auth.VerifyPassword(cfg.Admin.PasswordHash, req.Password)
	if err != nil || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	token, err := h.sessions.Issue(cfg.Admin.Username, 30*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue session"})
		return
	}
	h.sessions.SetCookie(c.Writer, token)
	c.JSON(http.StatusOK, gin.H{"username": cfg.Admin.Username})
}

func (h *Handler) logout(c *gin.Context) {
	auth.ClearSessionCookie(c.Writer)
	c.Status(http.StatusNoContent)
}

func (h *Handler) me(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{"username": username})
}

func (h *Handler) listChannels(c *gin.Context) {
	cfg := h.config.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		config.ChannelTypeOpenAIAPI:    openAIChannelsOrEmpty(cfg.OpenAIAPI),
		config.ChannelTypeChatGPTOAuth: oauthChannelsOrEmpty(cfg.ChatGPTOAuth),
	})
}

func (h *Handler) createChannel(c *gin.Context) {
	channelType := c.Param("type")
	switch channelType {
	case config.ChannelTypeOpenAIAPI:
		var req config.OpenAIAPIChannel
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel body"})
			return
		}
		if err := h.config.Save(func(cfg *config.Config) error {
			if openAIChannelIndex(cfg.OpenAIAPI, req.Name) >= 0 {
				return errConflict("channel already exists")
			}
			cfg.OpenAIAPI = append(cfg.OpenAIAPI, req)
			return nil
		}); err != nil {
			writeSaveError(c, err)
			return
		}
	case config.ChannelTypeChatGPTOAuth:
		var req config.ChatGPTOAuthChannel
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel body"})
			return
		}
		if err := h.config.Save(func(cfg *config.Config) error {
			if oauthChannelIndex(cfg.ChatGPTOAuth, req.Name) >= 0 {
				return errConflict("channel already exists")
			}
			cfg.ChatGPTOAuth = append(cfg.ChatGPTOAuth, req)
			return nil
		}); err != nil {
			writeSaveError(c, err)
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported channel type"})
		return
	}
	c.Status(http.StatusCreated)
}

func (h *Handler) updateChannel(c *gin.Context) {
	channelType := c.Param("type")
	name := c.Param("name")
	switch channelType {
	case config.ChannelTypeOpenAIAPI:
		var req config.OpenAIAPIChannel
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel body"})
			return
		}
		if err := h.config.Save(func(cfg *config.Config) error {
			idx := openAIChannelIndex(cfg.OpenAIAPI, name)
			if idx < 0 {
				return errNotFound("channel not found")
			}
			cfg.OpenAIAPI[idx] = req
			return nil
		}); err != nil {
			writeSaveError(c, err)
			return
		}
	case config.ChannelTypeChatGPTOAuth:
		var req config.ChatGPTOAuthChannel
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel body"})
			return
		}
		if err := h.config.Save(func(cfg *config.Config) error {
			idx := oauthChannelIndex(cfg.ChatGPTOAuth, name)
			if idx < 0 {
				return errNotFound("channel not found")
			}
			cfg.ChatGPTOAuth[idx] = req
			return nil
		}); err != nil {
			writeSaveError(c, err)
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported channel type"})
		return
	}
	c.Status(http.StatusOK)
}

func (h *Handler) deleteChannel(c *gin.Context) {
	channelType := c.Param("type")
	name := c.Param("name")
	if err := h.config.Save(func(cfg *config.Config) error {
		switch channelType {
		case config.ChannelTypeOpenAIAPI:
			idx := openAIChannelIndex(cfg.OpenAIAPI, name)
			if idx < 0 {
				return errNotFound("channel not found")
			}
			cfg.OpenAIAPI = append(cfg.OpenAIAPI[:idx], cfg.OpenAIAPI[idx+1:]...)
		case config.ChannelTypeChatGPTOAuth:
			idx := oauthChannelIndex(cfg.ChatGPTOAuth, name)
			if idx < 0 {
				return errNotFound("channel not found")
			}
			cfg.ChatGPTOAuth = append(cfg.ChatGPTOAuth[:idx], cfg.ChatGPTOAuth[idx+1:]...)
		default:
			return errBadRequest("unsupported channel type")
		}
		return nil
	}); err != nil {
		writeSaveError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type probeModelsRequest struct {
	BaseURL string `json:"base-url"`
	APIKey  string `json:"api-key"`
	Timeout int    `json:"timeout-sec"`
}

func (h *Handler) probeModels(c *gin.Context) {
	var req probeModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	models, err := h.probe.Models(ctx, upstream.ModelsRequest{BaseURL: req.BaseURL, APIKey: req.APIKey, Timeout: timeout})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h *Handler) healthCheck(c *gin.Context) {
	channelType := c.Param("type")
	name := c.Param("name")
	if channelType != config.ChannelTypeOpenAIAPI {
		c.JSON(http.StatusNotImplemented, gin.H{"ok": false, "error": "health check for this channel type is not implemented yet"})
		return
	}
	cfg := h.config.Snapshot()
	idx := openAIChannelIndex(cfg.OpenAIAPI, name)
	if idx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "channel not found"})
		return
	}
	ch := cfg.OpenAIAPI[idx]
	if len(ch.APIKeyEntries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "channel has no api key entries"})
		return
	}
	start := time.Now()
	err := h.probe.HealthCheck(c.Request.Context(), upstream.HealthCheckRequest{
		BaseURL: ch.BaseURL,
		APIKey:  ch.APIKeyEntries[0].APIKey,
		Timeout: time.Duration(ch.EffectiveTimeoutSec()) * time.Second,
	})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "latencyMs": latency, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "latencyMs": latency})
}

func (h *Handler) listKeys(c *gin.Context) {
	cfg := h.config.Snapshot()
	keys := make([]gin.H, 0, len(cfg.APIKeys))
	for _, key := range cfg.APIKeys {
		keys = append(keys, gin.H{
			"name":      key.Name,
			"notes":     key.Notes,
			"token":     key.Token,
			"tokenMask": auth.MaskToken(key.Token),
			"disabled":  key.Disabled,
		})
	}
	c.JSON(http.StatusOK, keys)
}

type createKeyRequest struct {
	Name  string `json:"name"`
	Notes string `json:"notes"`
}

func (h *Handler) createKey(c *gin.Context) {
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	token, err := auth.GenerateProxyAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}
	if err := h.config.Save(func(cfg *config.Config) error {
		cfg.APIKeys = append(cfg.APIKeys, config.APIKeyConfig{
			Token: token,
			Name:  strings.TrimSpace(req.Name),
			Notes: strings.TrimSpace(req.Notes),
		})
		return nil
	}); err != nil {
		writeSaveError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token})
}

type updateKeyRequest struct {
	Name     *string `json:"name"`
	Notes    *string `json:"notes"`
	Disabled *bool   `json:"disabled"`
}

func (h *Handler) updateKey(c *gin.Context) {
	id := c.Param("id")
	var req updateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.config.Save(func(cfg *config.Config) error {
		idx, err := apiKeyIndex(cfg.APIKeys, id)
		if err != nil {
			return err
		}
		if req.Name != nil {
			cfg.APIKeys[idx].Name = strings.TrimSpace(*req.Name)
		}
		if req.Notes != nil {
			cfg.APIKeys[idx].Notes = strings.TrimSpace(*req.Notes)
		}
		if req.Disabled != nil {
			cfg.APIKeys[idx].Disabled = *req.Disabled
		}
		return nil
	}); err != nil {
		writeSaveError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *Handler) deleteKey(c *gin.Context) {
	id := c.Param("id")
	if err := h.config.Save(func(cfg *config.Config) error {
		idx, err := apiKeyIndex(cfg.APIKeys, id)
		if err != nil {
			return err
		}
		cfg.APIKeys = append(cfg.APIKeys[:idx], cfg.APIKeys[idx+1:]...)
		return nil
	}); err != nil {
		writeSaveError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listLogs(c *gin.Context) {
	if h.logs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "logs repository is not configured"})
		return
	}
	filter := store.QueryFilter{
		ChannelName: c.Query("channel"),
		StatusCode:  queryInt(c, "status"),
		StartMS:     queryInt64(c, "from"),
		EndMS:       queryInt64(c, "to"),
		Limit:       queryInt(c, "limit"),
	}
	page := queryInt(c, "page")
	if page <= 0 {
		page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	filter.Offset = (page - 1) * filter.Limit
	logs, err := h.logs.Query(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": logs, "page": page, "limit": filter.Limit})
}

func (h *Handler) channelStats(c *gin.Context) {
	if h.stats == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stats repository is not configured"})
		return
	}
	summaries, err := h.stats.QueryChannelSummary(c.Request.Context(), parseWindow(c.Query("window")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summaries)
}

func (h *Handler) statsSeries(c *gin.Context) {
	if h.stats == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "stats repository is not configured"})
		return
	}
	channel := c.Query("channel")
	if channel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel is required"})
		return
	}
	points, err := h.stats.QuerySeries(c.Request.Context(), channel, store.Metric(c.DefaultQuery("metric", string(store.MetricRequests))), parseWindow(c.Query("window")))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, points)
}

func (h *Handler) requireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(auth.SessionCookieName)
		if errors.Is(err, http.ErrNoCookie) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "login required"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}
		username, err := h.sessions.Verify(cookie.Value)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}
		c.Set("username", username)
		c.Next()
	}
}

type apiError struct {
	status  int
	message string
}

func (e apiError) Error() string {
	return e.message
}

func errConflict(message string) error {
	return apiError{status: http.StatusConflict, message: message}
}

func errNotFound(message string) error {
	return apiError{status: http.StatusNotFound, message: message}
}

func errBadRequest(message string) error {
	return apiError{status: http.StatusBadRequest, message: message}
}

func writeSaveError(c *gin.Context, err error) {
	var apiErr apiError
	if errors.As(err, &apiErr) {
		c.JSON(apiErr.status, gin.H{"error": apiErr.message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func openAIChannelIndex(channels []config.OpenAIAPIChannel, name string) int {
	for i, ch := range channels {
		if ch.Name == name {
			return i
		}
	}
	return -1
}

func openAIChannelsOrEmpty(channels []config.OpenAIAPIChannel) []config.OpenAIAPIChannel {
	if channels == nil {
		return []config.OpenAIAPIChannel{}
	}
	return channels
}

func oauthChannelsOrEmpty(channels []config.ChatGPTOAuthChannel) []config.ChatGPTOAuthChannel {
	if channels == nil {
		return []config.ChatGPTOAuthChannel{}
	}
	return channels
}

func oauthChannelIndex(channels []config.ChatGPTOAuthChannel, name string) int {
	for i, ch := range channels {
		if ch.Name == name {
			return i
		}
	}
	return -1
}

func apiKeyIndex(keys []config.APIKeyConfig, id string) (int, error) {
	var matches []int
	for i, key := range keys {
		if key.Token == id {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		for i, key := range keys {
			if key.Name != "" && key.Name == id {
				matches = append(matches, i)
			}
		}
	}
	if len(matches) == 0 {
		for i, key := range keys {
			if auth.MaskToken(key.Token) == id {
				matches = append(matches, i)
			}
		}
	}
	if len(matches) == 0 {
		return -1, errNotFound("api key not found")
	}
	if len(matches) > 1 {
		return -1, errConflict("api key identifier is ambiguous")
	}
	return matches[0], nil
}

func queryInt(c *gin.Context, name string) int {
	value, _ := strconv.Atoi(c.Query(name))
	return value
}

func queryInt64(c *gin.Context, name string) int64 {
	value, _ := strconv.ParseInt(c.Query(name), 10, 64)
	return value
}

func parseWindow(raw string) store.TimeWindow {
	now := time.Now().UnixMilli()
	var duration time.Duration
	switch raw {
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	default:
		duration = 24 * time.Hour
	}
	return store.TimeWindow{StartMS: now - duration.Milliseconds(), EndMS: now}
}
