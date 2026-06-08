package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/huangjunjan/proxy-hub/internal/mcp"
)

// MCPHandler 提供 MCP 共享管理端点（/v0/mcp/*），守护于 admin key。
type MCPHandler struct {
	svc *mcp.Service
}

// NewMCPHandler 创建 MCP 处理器。
func NewMCPHandler(svc *mcp.Service) *MCPHandler { return &MCPHandler{svc: svc} }

type mcpServerRequest struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Spec        map[string]any `json:"spec"`
	Description string         `json:"description"`
	Homepage    string         `json:"homepage"`
	Docs        string         `json:"docs"`
	Tags        []string       `json:"tags"`
}

type mcpServerResponse struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Spec          map[string]any `json:"spec"`
	Description   string         `json:"description"`
	Homepage      string         `json:"homepage"`
	Docs          string         `json:"docs"`
	Tags          []string       `json:"tags"`
	EnabledCodex  bool           `json:"enabled_codex"`
	EnabledClaude bool           `json:"enabled_claude"`
}

func toMCPServerResponse(s mcp.Server) mcpServerResponse {
	tags := s.Tags
	if tags == nil {
		tags = []string{}
	}
	spec := s.Spec
	if spec == nil {
		spec = map[string]any{}
	}
	return mcpServerResponse{
		ID: s.ID, Name: s.Name, Spec: spec, Description: s.Description, Homepage: s.Homepage,
		Docs: s.Docs, Tags: tags, EnabledCodex: s.EnabledCodex, EnabledClaude: s.EnabledClaude,
	}
}

func (r mcpServerRequest) toDomain() mcp.Server {
	return mcp.Server{
		ID: r.ID, Name: r.Name, Spec: r.Spec, Description: r.Description,
		Homepage: r.Homepage, Docs: r.Docs, Tags: r.Tags,
	}
}

// ListServers 处理 GET /v0/mcp/servers。
func (h *MCPHandler) ListServers(c *gin.Context) {
	servers, err := h.svc.ListServers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]mcpServerResponse, 0, len(servers))
	for _, s := range servers {
		out = append(out, toMCPServerResponse(s))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// GetServer 处理 GET /v0/mcp/servers/:id。
func (h *MCPHandler) GetServer(c *gin.Context) {
	s, found, err := h.svc.GetServer(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "server 不存在"})
		return
	}
	c.JSON(http.StatusOK, toMCPServerResponse(s))
}

// CreateServer 处理 POST /v0/mcp/servers。
func (h *MCPHandler) CreateServer(c *gin.Context) {
	var req mcpServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	if err := h.svc.UpsertServer(c.Request.Context(), req.toDomain()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondServer(c, http.StatusCreated, req.ID)
}

// UpdateServer 处理 PUT /v0/mcp/servers/:id。
func (h *MCPHandler) UpdateServer(c *gin.Context) {
	var req mcpServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	req.ID = c.Param("id")
	if err := h.svc.UpsertServer(c.Request.Context(), req.toDomain()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondServer(c, http.StatusOK, req.ID)
}

// DeleteServer 处理 DELETE /v0/mcp/servers/:id。
func (h *MCPHandler) DeleteServer(c *gin.Context) {
	if err := h.svc.DeleteServer(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": c.Param("id")})
}

type mcpToggleRequest struct {
	Client  string `json:"client"`
	Enabled bool   `json:"enabled"`
}

// ToggleServer 处理 PUT /v0/mcp/servers/:id/toggle。
func (h *MCPHandler) ToggleServer(c *gin.Context) {
	var req mcpToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	if err := h.svc.ToggleClient(c.Request.Context(), c.Param("id"), req.Client, req.Enabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.respondServer(c, http.StatusOK, c.Param("id"))
}

func (h *MCPHandler) respondServer(c *gin.Context, code int, id string) {
	s, found, err := h.svc.GetServer(c.Request.Context(), id)
	if err != nil || !found {
		c.JSON(code, gin.H{"id": id})
		return
	}
	c.JSON(code, toMCPServerResponse(s))
}

type mcpTargetRequest struct {
	Client     string `json:"client"`
	ConfigPath string `json:"config_path"`
	Label      string `json:"label"`
	Enabled    *bool  `json:"enabled"`
}

type mcpTargetResponse struct {
	ID             int64  `json:"id"`
	Client         string `json:"client"`
	ConfigPath     string `json:"config_path"`
	Label          string `json:"label"`
	Enabled        bool   `json:"enabled"`
	LastSyncedAt   string `json:"last_synced_at"`
	LastSyncStatus string `json:"last_sync_status"`
}

func toMCPTargetResponse(t mcp.Target) mcpTargetResponse {
	return mcpTargetResponse{
		ID: t.ID, Client: t.Client, ConfigPath: t.ConfigPath, Label: t.Label,
		Enabled: t.Enabled, LastSyncedAt: t.LastSyncedAt, LastSyncStatus: t.LastSyncStatus,
	}
}

// ListTargets 处理 GET /v0/mcp/targets。
func (h *MCPHandler) ListTargets(c *gin.Context) {
	targets, err := h.svc.ListTargets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]mcpTargetResponse, 0, len(targets))
	for _, t := range targets {
		out = append(out, toMCPTargetResponse(t))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// CreateTarget 处理 POST /v0/mcp/targets。
func (h *MCPHandler) CreateTarget(c *gin.Context) {
	var req mcpTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	t, err := h.svc.CreateTarget(c.Request.Context(), mcp.Target{
		Client: req.Client, ConfigPath: req.ConfigPath, Label: req.Label, Enabled: enabled,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, toMCPTargetResponse(t))
}

// UpdateTarget 处理 PUT /v0/mcp/targets/:id。
func (h *MCPHandler) UpdateTarget(c *gin.Context) {
	id, err := mcpPathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	var req mcpTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := h.svc.UpdateTarget(c.Request.Context(), mcp.Target{
		ID: id, Client: req.Client, ConfigPath: req.ConfigPath, Label: req.Label, Enabled: enabled,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// DeleteTarget 处理 DELETE /v0/mcp/targets/:id。
func (h *MCPHandler) DeleteTarget(c *gin.Context) {
	id, err := mcpPathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	if err := h.svc.DeleteTarget(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// SyncAll 处理 POST /v0/mcp/sync。
func (h *MCPHandler) SyncAll(c *gin.Context) {
	if err := h.svc.SyncAll(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SyncTarget 处理 POST /v0/mcp/sync/:id。
func (h *MCPHandler) SyncTarget(c *gin.Context) {
	id, err := mcpPathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	if err := h.svc.SyncTarget(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "target": id})
}

// ImportTarget 处理 POST /v0/mcp/import/:id（从目标现有配置导入注册表，只读）。
func (h *MCPHandler) ImportTarget(c *gin.Context) {
	id, err := mcpPathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	if err := h.svc.Import(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "target": id})
}

type mcpImportBundleRequest struct {
	McpServers map[string]map[string]any `json:"mcpServers"`
	Apps       []string                  `json:"apps"`
}

// ImportBundle 处理 POST /v0/mcp/import-bundle（朴素 {mcpServers} + apps）。
func (h *MCPHandler) ImportBundle(c *gin.Context) {
	var req mcpImportBundleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	if err := h.svc.ImportBundle(c.Request.Context(), req.McpServers, req.Apps); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(req.McpServers)})
}

func mcpPathID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}
