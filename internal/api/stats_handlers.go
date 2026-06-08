package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/huangjunjan/proxy-hub/internal/channel"
	"github.com/huangjunjan/proxy-hub/internal/stats"
	"github.com/huangjunjan/proxy-hub/internal/store/dbgen"
	"github.com/huangjunjan/proxy-hub/internal/usage"
)

// healthLister 抽象渠道健康读取（channel.DAO 满足），供 /admin/stats/health 与 /admin/health/checks 用。
type healthLister interface {
	ListHealth(ctx context.Context) ([]channel.HealthSnapshot, error)
	ListRecentHealthChecks(ctx context.Context, limit int) ([]channel.HealthCheck, error)
}

// StatsHandler 提供仪表盘读取端点（/admin/stats/*）与定价 CRUD（/admin/pricing），守护于 admin key。
type StatsHandler struct {
	dao     *stats.DAO
	pricing *stats.Table
	emitter *usage.Emitter
	health  healthLister
}

// NewStatsHandler 创建统计处理器。
func NewStatsHandler(dao *stats.DAO, pricing *stats.Table, emitter *usage.Emitter, health healthLister) *StatsHandler {
	return &StatsHandler{dao: dao, pricing: pricing, emitter: emitter, health: health}
}

// Overview 处理 GET /admin/stats/overview：区间内 token/请求/错误率/成本/延迟/TTFT + 溢出与缺价。
func (h *StatsHandler) Overview(c *gin.Context) {
	ctx := c.Request.Context()
	sinceHour := stats.HourBucket(time.Now().Add(-parseRange(c)))
	sum, err := h.dao.SumHourlyRange(ctx, sinceHour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	models, err := h.dao.BreakdownByModel(ctx, sinceHour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	totalCost := decimal.Zero
	missing := []string{}
	for _, m := range models {
		cost, miss := h.pricing.Compute(m.Dim, m.InputTokens, m.OutputTokens, m.CacheReadTokens, m.CacheCreationTokens)
		totalCost = totalCost.Add(cost.Total)
		if miss && (m.InputTokens > 0 || m.OutputTokens > 0) {
			missing = append(missing, m.Dim)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"range":                 rangeLabel(c),
		"request_count":         sum.RequestCount,
		"success_count":         sum.SuccessCount,
		"error_count":           sum.ErrorCount,
		"error_rate":            ratio(sum.ErrorCount, sum.RequestCount),
		"input_tokens":          sum.InputTokens,
		"output_tokens":         sum.OutputTokens,
		"cache_read_tokens":     sum.CacheReadTokens,
		"cache_creation_tokens": sum.CacheCreationTokens,
		"reasoning_tokens":      sum.ReasoningTokens,
		"total_cost":            totalCost.StringFixed(6),
		"avg_latency_ms":        avg(sum.SumLatencyMs, sum.RequestCount),
		"avg_first_token_ms":    avg(sum.SumFirstTokenMs, sum.CountFirstToken),
		"pricing_missing":       missing,
		"dropped_events":        h.emitter.Dropped(),
	})
}

type tsPoint struct {
	Bucket       string  `json:"bucket"`
	RequestCount int64   `json:"request_count"`
	ErrorCount   int64   `json:"error_count"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// Timeseries 处理 GET /admin/stats/timeseries：按 interval（hour|day）从汇总表取时序点。
func (h *StatsHandler) Timeseries(c *gin.Context) {
	ctx := c.Request.Context()
	dur := parseRange(c)
	now := time.Now()
	points := []tsPoint{}
	if c.Query("interval") == "day" {
		rows, err := h.dao.TimeseriesDaily(ctx, stats.DayBucket(now.Add(-dur)))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, r := range rows {
			points = append(points, tsPoint{r.Bucket, r.RequestCount, r.ErrorCount, r.InputTokens, r.OutputTokens, avg(r.SumLatencyMs, r.RequestCount)})
		}
		c.JSON(http.StatusOK, gin.H{"interval": "day", "points": points})
		return
	}
	rows, err := h.dao.TimeseriesHourly(ctx, stats.HourBucket(now.Add(-dur)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, r := range rows {
		points = append(points, tsPoint{r.Bucket, r.RequestCount, r.ErrorCount, r.InputTokens, r.OutputTokens, avg(r.SumLatencyMs, r.RequestCount)})
	}
	c.JSON(http.StatusOK, gin.H{"interval": "hour", "points": points})
}

// Breakdown 处理 GET /admin/stats/breakdown?by=model|channel|api_key|error_type。
func (h *StatsHandler) Breakdown(c *gin.Context) {
	ctx := c.Request.Context()
	dur := parseRange(c)
	sinceHour := stats.HourBucket(time.Now().Add(-dur))
	switch c.Query("by") {
	case "channel":
		rows, err := h.dao.BreakdownByChannel(ctx, sinceHour)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			out = append(out, gin.H{"dim": strconv.FormatInt(r.Dim, 10), "request_count": r.RequestCount, "error_count": r.ErrorCount, "input_tokens": r.InputTokens, "output_tokens": r.OutputTokens})
		}
		c.JSON(http.StatusOK, gin.H{"by": "channel", "data": out})
	case "api_key":
		rows, err := h.dao.BreakdownByAPIKey(ctx, sinceHour)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			out = append(out, gin.H{"dim": strconv.FormatInt(r.Dim, 10), "request_count": r.RequestCount, "error_count": r.ErrorCount, "input_tokens": r.InputTokens, "output_tokens": r.OutputTokens})
		}
		c.JSON(http.StatusOK, gin.H{"by": "api_key", "data": out})
	case "error_type":
		rows, err := h.dao.BreakdownByErrorType(ctx, time.Now().Add(-dur).UTC().Format(time.RFC3339))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			out = append(out, gin.H{"dim": r.ErrorType, "request_count": r.RequestCount})
		}
		c.JSON(http.StatusOK, gin.H{"by": "error_type", "data": out})
	default: // model
		rows, err := h.dao.BreakdownByModel(ctx, sinceHour)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			cost, miss := h.pricing.Compute(r.Dim, r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheCreationTokens)
			out = append(out, gin.H{
				"dim": r.Dim, "request_count": r.RequestCount, "error_count": r.ErrorCount,
				"input_tokens": r.InputTokens, "output_tokens": r.OutputTokens,
				"cost": cost.Total.StringFixed(6), "pricing_missing": miss,
			})
		}
		c.JSON(http.StatusOK, gin.H{"by": "model", "data": out})
	}
}

type logDTO struct {
	ID             int64  `json:"id"`
	RequestID      string `json:"request_id"`
	CreatedAt      string `json:"created_at"`
	APIKeyID       int64  `json:"api_key_id"`
	ChannelID      int64  `json:"channel_id"`
	Group          string `json:"group"`
	RequestedModel string `json:"requested_model"`
	UpstreamModel  string `json:"upstream_model"`
	EndpointFormat string `json:"endpoint_format"`
	IsStream       bool   `json:"is_stream"`
	InputTokens    int64  `json:"input_tokens"`
	OutputTokens   int64  `json:"output_tokens"`
	TotalTokens    int64  `json:"total_tokens"`
	LatencyMs      int64  `json:"latency_ms"`
	FirstTokenMs   *int64 `json:"first_token_ms"`
	StatusCode     int64  `json:"status_code"`
	IsError        bool   `json:"is_error"`
	ErrorType      string `json:"error_type"`
	SessionID      string `json:"session_id"`
	UsageSource    string `json:"usage_source"`
	Cost           string `json:"cost"`
}

// Logs 处理 GET /admin/stats/logs：分页钻取请求日志（可按 request_id/api_key_id/channel_id/model 过滤）。
func (h *StatsHandler) Logs(c *gin.Context) {
	ctx := c.Request.Context()
	page := atoiDefault(c.Query("page"), 1)
	if page < 1 {
		page = 1
	}
	size := atoiDefault(c.Query("size"), 50)
	if size < 1 || size > 500 {
		size = 50
	}
	f := stats.LogFilter{
		RequestID: c.Query("request_id"),
		APIKeyID:  atoi64(c.Query("api_key_id")),
		ChannelID: atoi64(c.Query("channel_id")),
		Model:     c.Query("model"),
		Limit:     int64(size),
		Offset:    int64((page - 1) * size),
	}
	logs, err := h.dao.ListLogs(ctx, f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	total, err := h.dao.CountLogs(ctx, f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]logDTO, 0, len(logs))
	for i := range logs {
		out = append(out, h.toLogDTO(&logs[i]))
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "page": page, "size": size, "total": total})
}

func (h *StatsHandler) toLogDTO(r *dbgen.RequestLog) logDTO {
	var ftm *int64
	if r.FirstTokenMs.Valid {
		v := r.FirstTokenMs.Int64
		ftm = &v
	}
	// 成本：把原始 input 归一为纯新输入后按定价计算。
	billable := r.InputTokens
	if r.EndpointFormat == "openai" || r.EndpointFormat == "responses" {
		billable -= r.CacheReadTokens
		if billable < 0 {
			billable = 0
		}
	}
	cost, _ := h.pricing.Compute(r.RequestedModel, billable, r.OutputTokens, r.CacheReadTokens, r.CacheCreationTokens)
	return logDTO{
		ID: r.ID, RequestID: r.RequestID, CreatedAt: r.CreatedAt, APIKeyID: r.ApiKeyID, ChannelID: r.ChannelID,
		Group: r.GroupName, RequestedModel: r.RequestedModel, UpstreamModel: r.UpstreamModel, EndpointFormat: r.EndpointFormat,
		IsStream: r.IsStream != 0, InputTokens: r.InputTokens, OutputTokens: r.OutputTokens, TotalTokens: r.TotalTokens,
		LatencyMs: r.LatencyMs, FirstTokenMs: ftm, StatusCode: r.StatusCode, IsError: r.IsError != 0,
		ErrorType: r.ErrorType, SessionID: r.SessionID, UsageSource: r.UsageSource, Cost: cost.Total.StringFixed(6),
	}
}

type healthDTO struct {
	ChannelID           int64  `json:"channel_id"`
	Model               string `json:"model"`
	IsHealthy           bool   `json:"is_healthy"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	LastError           string `json:"last_error"`
	CooldownUntil       string `json:"cooldown_until"`
	UpdatedAt           string `json:"updated_at"`
}

// Health 处理 GET /admin/stats/health：渠道×模型 健康/冷却快照。
func (h *StatsHandler) Health(c *gin.Context) {
	snaps, err := h.health.ListHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]healthDTO, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, healthDTO{
			ChannelID: s.ChannelID, Model: s.Model, IsHealthy: s.IsHealthy, ConsecutiveFailures: s.ConsecutiveFailures,
			LastError: s.LastError, CooldownUntil: rfc3339OrEmpty(s.CooldownUntil), UpdatedAt: rfc3339OrEmpty(s.UpdatedAt),
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// HealthChecks 处理 GET /admin/health/checks：最近若干条主动探测记录（health.enabled 开启时由探测器写入）。
func (h *StatsHandler) HealthChecks(c *gin.Context) {
	limit := atoiDefault(c.Query("limit"), 100)
	checks, err := h.health.ListRecentHealthChecks(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(checks))
	for _, ck := range checks {
		out = append(out, gin.H{
			"channel_id": ck.ChannelID, "model": ck.Model, "success": ck.Success,
			"http_status": ck.HTTPStatus, "response_time_ms": ck.ResponseTimeMS,
			"message": ck.Message, "checked_at": ck.CheckedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

type pricingDTO struct {
	ModelID                 string `json:"model_id"`
	InputPerMillion         string `json:"input_per_million"`
	OutputPerMillion        string `json:"output_per_million"`
	CacheReadPerMillion     string `json:"cache_read_per_million"`
	CacheCreationPerMillion string `json:"cache_creation_per_million"`
	Source                  string `json:"source"`
	UpdatedAt               string `json:"updated_at"`
}

// ListPricing 处理 GET /admin/pricing。
func (h *StatsHandler) ListPricing(c *gin.Context) {
	rows, err := h.dao.ListPricing(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]pricingDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, pricingDTO{
			ModelID: r.ModelID, InputPerMillion: r.InputPerMillion, OutputPerMillion: r.OutputPerMillion,
			CacheReadPerMillion: r.CacheReadPerMillion, CacheCreationPerMillion: r.CacheCreationPerMillion,
			Source: r.Source, UpdatedAt: r.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

type pricingRequest struct {
	InputPerMillion         string `json:"input_per_million"`
	OutputPerMillion        string `json:"output_per_million"`
	CacheReadPerMillion     string `json:"cache_read_per_million"`
	CacheCreationPerMillion string `json:"cache_creation_per_million"`
}

// UpsertPricing 处理 PUT /admin/pricing/:model：admin 覆盖定价并重载内存表。
func (h *StatsHandler) UpsertPricing(c *gin.Context) {
	model := c.Param("model")
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 model"})
		return
	}
	var req pricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体非法: " + err.Error()})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := h.dao.UpsertPricing(c.Request.Context(), dbgen.UpsertModelPricingParams{
		ModelID: model, InputPerMillion: zeroIfEmpty(req.InputPerMillion), OutputPerMillion: zeroIfEmpty(req.OutputPerMillion),
		CacheReadPerMillion: zeroIfEmpty(req.CacheReadPerMillion), CacheCreationPerMillion: zeroIfEmpty(req.CacheCreationPerMillion),
		Source: "admin", UpdatedAt: now,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.reloadPricing(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"model_id": model, "source": "admin"})
}

// DeletePricing 处理 DELETE /admin/pricing/:model。
func (h *StatsHandler) DeletePricing(c *gin.Context) {
	model := c.Param("model")
	if err := h.dao.DeletePricing(c.Request.Context(), model); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.reloadPricing(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"deleted": model})
}

// reloadPricing 改价后从 DB 重载内存定价表（失败仅记日志，下次重启自愈）。
func (h *StatsHandler) reloadPricing(ctx context.Context) {
	rows, err := h.dao.PriceRows(ctx)
	if err != nil {
		slog.Warn("重载定价表失败（下次重启自愈）", "error", err)
		return
	}
	h.pricing.Load(rows)
}

// ---- 小工具 ----

func parseRange(c *gin.Context) time.Duration {
	switch c.Query("range") {
	case "1h":
		return time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	case "", "24h":
		return 24 * time.Hour
	default:
		if d, err := time.ParseDuration(c.Query("range")); err == nil && d > 0 {
			return d
		}
		return 24 * time.Hour
	}
}

func rangeLabel(c *gin.Context) string {
	if r := c.Query("range"); r != "" {
		return r
	}
	return "24h"
}

func ratio(num, den int64) float64 {
	if den <= 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func avg(sum, count int64) float64 {
	if count <= 0 {
		return 0
	}
	return float64(sum) / float64(count)
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func zeroIfEmpty(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

func rfc3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
