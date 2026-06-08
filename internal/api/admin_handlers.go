package api

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/huangjunjan/proxy-hub/internal/adaptor"
	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/credstore"
)

// channelTestTimeout 是渠道连通性测试的上限。
const channelTestTimeout = 20 * time.Second

// AdminHandler 提供渠道管理端点（/admin/channels），守护于 admin key。
type AdminHandler struct {
	mgr *channel.Manager
}

// NewAdminHandler 创建渠道管理处理器。
func NewAdminHandler(mgr *channel.Manager) *AdminHandler {
	return &AdminHandler{mgr: mgr}
}

// channelRequest 是创建/更新渠道的入参。api_key 仅写入 credstore，绝不回显。
// enabled/priority/weight 用指针区分"未提供"（创建时给默认值）。
type channelRequest struct {
	Name         string            `json:"name"`
	Enabled      *bool             `json:"enabled"`
	Platform     string            `json:"platform"`
	Type         string            `json:"type"`
	BaseURL      string            `json:"base_url"`
	Group        string            `json:"group"`
	Priority     *int              `json:"priority"`
	Weight       *int              `json:"weight"`
	Models       []string          `json:"models"`
	ModelMapping map[string]string `json:"model_mapping"`
	Prefix       string            `json:"prefix"`
	ProxyURL     string            `json:"proxy_url"`
	APIKey       string            `json:"api_key"`
}

// channelResponse 是渠道的对外表示（无任何密钥；以 has_credential 标识是否已配置凭证）。
type channelResponse struct {
	ID            int64             `json:"id"`
	Name          string            `json:"name"`
	Enabled       bool              `json:"enabled"`
	Platform      string            `json:"platform"`
	Type          string            `json:"type"`
	BaseURL       string            `json:"base_url"`
	Group         string            `json:"group"`
	Priority      int               `json:"priority"`
	Weight        int               `json:"weight"`
	Models        []string          `json:"models"`
	ModelMapping  map[string]string `json:"model_mapping"`
	Prefix        string            `json:"prefix"`
	ProxyURL      string            `json:"proxy_url"`
	Status        string            `json:"status"`
	ErrorMessage  string            `json:"error_message"`
	HasCredential bool              `json:"has_credential"`
}

// toChannelResponse 把领域渠道转为对外 DTO（附带凭证存在性）。
func (h *AdminHandler) toChannelResponse(c channel.Channel) channelResponse {
	_, has := h.mgr.Cred(c.ID)
	models := c.Models
	if models == nil {
		models = []string{}
	}
	mapping := c.ModelMapping
	if mapping == nil {
		mapping = map[string]string{}
	}
	return channelResponse{
		ID: c.ID, Name: c.Name, Enabled: c.Enabled, Platform: string(c.Platform), Type: string(c.Type),
		BaseURL: c.BaseURL, Group: c.Group, Priority: c.Priority, Weight: c.Weight,
		Models: models, ModelMapping: mapping, Prefix: c.Prefix, ProxyURL: c.ProxyURL,
		Status: c.Status, ErrorMessage: c.ErrorMessage, HasCredential: has,
	}
}

// toDomain 把请求 DTO 转为领域渠道（带 id 用于更新）+ 可选凭证。
func (r channelRequest) toDomain(id int64) (channel.Channel, *credstore.Cred) {
	c := channel.Channel{
		ID: id, Name: r.Name, Platform: channel.Platform(r.Platform), Type: channel.ChannelType(r.Type),
		BaseURL: r.BaseURL, Group: r.Group, Models: r.Models, ModelMapping: r.ModelMapping,
		Prefix: r.Prefix, ProxyURL: r.ProxyURL,
	}
	// 默认值：enabled 默认 true，priority 默认 50，weight 默认 1。
	c.Enabled = true
	if r.Enabled != nil {
		c.Enabled = *r.Enabled
	}
	c.Priority = 50
	if r.Priority != nil {
		c.Priority = *r.Priority
	}
	c.Weight = 1
	if r.Weight != nil {
		c.Weight = *r.Weight
	}
	var cred *credstore.Cred
	if r.APIKey != "" {
		cred = &credstore.Cred{APIKey: r.APIKey}
	}
	return c, cred
}

// List 处理 GET /admin/channels。
func (h *AdminHandler) List(c *gin.Context) {
	chans, err := h.mgr.ListChannels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]channelResponse, 0, len(chans))
	for _, ch := range chans {
		out = append(out, h.toChannelResponse(ch))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Get 处理 GET /admin/channels/:id。
func (h *AdminHandler) Get(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	ch, found, err := h.mgr.GetChannel(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "渠道不存在"})
		return
	}
	c.JSON(http.StatusOK, h.toChannelResponse(ch))
}

// Create 处理 POST /admin/channels。
func (h *AdminHandler) Create(c *gin.Context) {
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	dom, cred := req.toDomain(0)
	saved, err := h.mgr.SaveChannel(c.Request.Context(), dom, cred)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, h.toChannelResponse(saved))
}

// Update 处理 PUT /admin/channels/:id。
func (h *AdminHandler) Update(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	if _, found, err := h.mgr.GetChannel(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "渠道不存在"})
		return
	}
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	dom, cred := req.toDomain(id)
	saved, err := h.mgr.SaveChannel(c.Request.Context(), dom, cred)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, h.toChannelResponse(saved))
}

// Delete 处理 DELETE /admin/channels/:id。
func (h *AdminHandler) Delete(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	if err := h.mgr.DeleteChannel(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// Test 处理 POST /admin/channels/:id/test：向上游发一个最小探针请求并回写渠道状态。
func (h *AdminHandler) Test(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 id"})
		return
	}
	ch, found, err := h.mgr.GetChannel(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "渠道不存在"})
		return
	}

	ok, status, msg := h.probe(c.Request.Context(), ch)

	newStatus := "active"
	if !ok {
		newStatus = "error"
	}
	if err := h.mgr.SetChannelStatus(c.Request.Context(), id, newStatus, msg); err != nil {
		// 状态回写失败不影响测试结论返回。
		c.Header("X-Status-Write-Error", err.Error())
	}
	c.JSON(http.StatusOK, gin.H{"ok": ok, "upstream_status": status, "message": msg})
}

// probe 用渠道适配器发一个 max_tokens=1 的最小请求，返回是否成功 + 上游状态 + 信息。
func (h *AdminHandler) probe(ctx context.Context, ch channel.Channel) (ok bool, upstreamStatus int, msg string) {
	cred, has := h.mgr.Cred(ch.ID)
	if !has {
		return false, 0, "缺少凭证"
	}
	ad, okAd := adaptor.Get(ch.Platform)
	if !okAd {
		return false, 0, "无平台适配器"
	}
	model := probeModel(ch)
	if model == "" {
		return false, 0, "渠道未配置任何模型"
	}

	in := &adaptor.RelayInput{
		EndpointFormat: formatForPlatform(ch.Platform),
		UpstreamModel:  model,
		IsStream:       false,
		Body:           probeBody(model),
	}
	rt := &channel.ChannelRuntime{
		ChannelID: ch.ID, UpstreamModel: model, Platform: ch.Platform, Type: ch.Type,
		BaseURL: ch.BaseURL, ProxyURL: ch.ProxyURL,
	}
	cctx, cancel := context.WithTimeout(ctx, channelTestTimeout)
	defer cancel()
	req, err := ad.BuildRequest(cctx, in, rt, cred)
	if err != nil {
		return false, 0, "构造请求失败: " + err.Error()
	}

	resp, err := probeClient(ch.ProxyURL).Do(req)
	if err != nil {
		return false, 0, truncate(err.Error(), 200)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, resp.StatusCode, ""
	}
	return false, resp.StatusCode, "上游返回 " + strconv.Itoa(resp.StatusCode)
}

// probeModel 取渠道用于探测的模型：优先 models[0]，否则任一 mapping 的 upstream 值。
func probeModel(ch channel.Channel) string {
	if len(ch.Models) > 0 {
		return ch.Models[0]
	}
	for _, up := range ch.ModelMapping {
		return channel.StripContextSuffix(up)
	}
	return ""
}

// probeBody 构造最小聊天请求体（OpenAI 与 Claude 同形可用：messages + max_tokens）。
func probeBody(model string) []byte {
	return []byte(`{"model":"` + model + `","messages":[{"role":"user","content":"ping"}],"max_tokens":1}`)
}

// formatForPlatform 把平台映射到入站方言（用于探测时选上游路径）。
func formatForPlatform(p channel.Platform) adaptor.EndpointFormat {
	if p == channel.PlatformAnthropic {
		return adaptor.FormatClaudeMessages
	}
	return adaptor.FormatOpenAIChat
}

// probeClient 构造一个（可选带出口代理的）测试用 http.Client。
func probeClient(proxyURL string) *http.Client {
	tr := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Transport: tr, Timeout: channelTestTimeout}
}

// pathID 解析路由参数 :id。
func pathID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

// truncate 截断字符串到最多 n 字节。
func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
