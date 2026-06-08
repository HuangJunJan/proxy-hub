package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/huangjunjan/proxy-hub/internal/apikey"
	"github.com/huangjunjan/proxy-hub/internal/channel"
)

// APIKeyHandler 提供入站 key 管理端点（/admin/api-keys），守护于 admin key。
type APIKeyHandler struct {
	mgr   *channel.Manager
	cache *apikey.Cache
}

// NewAPIKeyHandler 创建入站 key 管理处理器。
func NewAPIKeyHandler(mgr *channel.Manager, cache *apikey.Cache) *APIKeyHandler {
	return &APIKeyHandler{mgr: mgr, cache: cache}
}

type createKeyRequest struct {
	Name  string `json:"name"`
	Group string `json:"group"`
}

type setEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// Create 处理 POST /admin/api-keys：生成 key，明文仅此一次返回。
func (h *APIKeyHandler) Create(c *gin.Context) {
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	group := req.Group
	if group == "" {
		group = "default"
	}
	plaintext, id, err := h.mgr.CreateKey(c.Request.Context(), req.Name, group)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.cache.Invalidate()
	c.JSON(http.StatusCreated, gin.H{
		"id":      id,
		"name":    req.Name,
		"group":   group,
		"key":     plaintext,
		"warning": "明文仅此一次显示，请妥善保存",
	})
}

// List 处理 GET /admin/api-keys（不含明文/哈希）。
func (h *APIKeyHandler) List(c *gin.Context) {
	keys, err := h.mgr.ListKeys(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if keys == nil {
		keys = []apikey.KeyInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"data": keys})
}

// SetEnabled 处理 PUT /admin/api-keys/:id：启用/禁用。
func (h *APIKeyHandler) SetEnabled(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	var req setEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	if err := h.mgr.SetKeyEnabled(c.Request.Context(), id, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.cache.Invalidate()
	c.JSON(http.StatusOK, gin.H{"id": id, "enabled": req.Enabled})
}

// Delete 处理 DELETE /admin/api-keys/:id。
func (h *APIKeyHandler) Delete(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	if err := h.mgr.DeleteKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.cache.Invalidate()
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}
