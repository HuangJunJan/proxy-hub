package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"proxy-hub/internal/auth"
	"proxy-hub/internal/config"
	"proxy-hub/internal/monitor"
	routeindex "proxy-hub/internal/router"
	"proxy-hub/internal/scheduler"
	"proxy-hub/internal/store"
	"proxy-hub/internal/upstream"
	"proxy-hub/internal/upstream/openai"
)

const maxModelRequestBodyBytes = 10 << 20

type Handler struct {
	config    *config.Manager
	scheduler *scheduler.Scheduler
	monitor   *monitor.Service
	openai    upstream.Adapter
	logger    *slog.Logger
	index     atomic.Pointer[routeindex.Index]
}

func NewHandler(configManager *config.Manager, sched *scheduler.Scheduler, monitorService *monitor.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if sched == nil {
		cfg := configManager.Snapshot()
		sched = scheduler.New(scheduler.Options{
			Cooldown:         time.Duration(cfg.EffectiveCircuitCooldownSec()) * time.Second,
			FailureThreshold: cfg.EffectiveCircuitFailureThreshold(),
		})
	}
	h := &Handler{
		config:    configManager,
		scheduler: sched,
		monitor:   monitorService,
		openai:    openai.New(nil),
		logger:    logger,
	}
	h.rebuildIndex(configManager.Snapshot())
	configManager.Subscribe(h.rebuildIndex)
	return h
}

func (h *Handler) Register(r gin.IRouter) {
	r.GET("/models", h.models)
	r.POST("/chat/completions", h.chatCompletions)
	r.POST("/responses", h.responses)
}

func (h *Handler) models(c *gin.Context) {
	idx := h.index.Load()
	models := idx.Models()
	data := make([]gin.H, 0, len(models))
	for _, model := range models {
		data = append(data, gin.H{"id": model, "object": "model"})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

func (h *Handler) chatCompletions(c *gin.Context) {
	h.modelRequest(c, modelProxyEndpoint{
		requestType:               "chat.completions",
		unsupportedChannelMessage: "chatgpt-oauth upstream is not implemented yet",
		call: func(ctx context.Context, selection scheduler.Selection, body []byte, meta modelRequestMeta, headers http.Header) (*upstream.ChatResponse, error) {
			return h.openai.Chat(ctx, upstream.ChatRequest{
				BaseURL:           selection.Hit.BaseURL,
				APIKey:            selection.APIKey,
				UpstreamModelName: selection.Hit.UpstreamModelName,
				OriginalBody:      body,
				Stream:            meta.Stream,
				Headers:           headers,
				Timeout:           selection.Hit.Timeout,
			})
		},
	})
}

func (h *Handler) responses(c *gin.Context) {
	h.modelRequest(c, modelProxyEndpoint{
		requestType:               "responses",
		unsupportedChannelMessage: "chatgpt-oauth upstream responses endpoint is not implemented yet",
		call: func(ctx context.Context, selection scheduler.Selection, body []byte, meta modelRequestMeta, headers http.Header) (*upstream.ChatResponse, error) {
			return h.openai.Responses(ctx, upstream.ResponsesRequest{
				BaseURL:           selection.Hit.BaseURL,
				APIKey:            selection.APIKey,
				UpstreamModelName: selection.Hit.UpstreamModelName,
				OriginalBody:      body,
				Stream:            meta.Stream,
				Headers:           headers,
				Timeout:           selection.Hit.Timeout,
			})
		},
	})
}

type modelProxyEndpoint struct {
	requestType               string
	unsupportedChannelMessage string
	call                      func(context.Context, scheduler.Selection, []byte, modelRequestMeta, http.Header) (*upstream.ChatResponse, error)
}

func (h *Handler) modelRequest(c *gin.Context, endpoint modelProxyEndpoint) {
	start := time.Now()
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxModelRequestBodyBytes))
	if err != nil {
		auth.AbortOpenAIError(c, http.StatusBadRequest, "Request body is too large or invalid.", "invalid_request_error", "invalid_request")
		h.submitLog(c, requestLogInput{start: start, endpoint: c.Request.URL.Path, requestType: endpoint.requestType, statusCode: http.StatusBadRequest, errorKind: "invalid_request", errorMessage: "request body is too large or invalid"})
		return
	}
	meta, err := parseModelRequestMeta(body)
	if err != nil {
		auth.AbortOpenAIError(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "invalid_request")
		h.submitLog(c, requestLogInput{start: start, endpoint: c.Request.URL.Path, requestType: endpoint.requestType, statusCode: http.StatusBadRequest, downstreamModel: meta.Model, reasoningEffort: meta.EffectiveReasoningEffort(), errorKind: "invalid_request", errorMessage: err.Error()})
		return
	}
	idx := h.index.Load()
	hits := idx.Resolve(meta.Model)
	if len(hits) == 0 {
		auth.AbortOpenAIError(c, http.StatusNotFound, "The requested model was not found.", "invalid_request_error", "model_not_found")
		h.submitLog(c, requestLogInput{start: start, endpoint: c.Request.URL.Path, requestType: endpoint.requestType, statusCode: http.StatusNotFound, downstreamModel: meta.Model, isStream: meta.Stream, reasoningEffort: meta.EffectiveReasoningEffort(), errorKind: "model_not_found", errorMessage: "model not found"})
		return
	}

	cfg := h.config.Snapshot()
	selections := h.scheduler.Pick(hits, cfg.EffectiveMaxRetries()+1)
	if len(selections) == 0 {
		auth.AbortOpenAIError(c, http.StatusServiceUnavailable, "No available upstream channel.", "upstream_error", "no_available_channel")
		h.submitLog(c, requestLogInput{start: start, endpoint: c.Request.URL.Path, requestType: endpoint.requestType, statusCode: http.StatusServiceUnavailable, downstreamModel: meta.Model, isStream: meta.Stream, reasoningEffort: meta.EffectiveReasoningEffort(), errorKind: "no_available_channel", errorMessage: "no available upstream channel"})
		return
	}

	var last upstreamFailure
	attempts := 0
	for _, selection := range selections {
		attempts++
		if selection.Hit.ChannelType != config.ChannelTypeOpenAIAPI {
			last = upstreamFailure{status: http.StatusNotImplemented, message: endpoint.unsupportedChannelMessage, code: "unsupported_channel"}
			h.scheduler.ReportFailure(selection.Hit.ChannelName)
			continue
		}
		resp, err := endpoint.call(c.Request.Context(), selection, body, meta, c.Request.Header)
		if err != nil {
			last = upstreamFailure{status: http.StatusGatewayTimeout, message: err.Error(), code: "upstream_timeout"}
			h.scheduler.ReportFailure(selection.Hit.ChannelName)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			last = readFailure(resp)
			h.scheduler.ReportFailure(selection.Hit.ChannelName)
			continue
		}
		h.scheduler.ReportSuccess(selection.Hit.ChannelName)
		logInput := requestLogInput{
			start:            start,
			endpoint:         c.Request.URL.Path,
			requestType:      endpoint.requestType,
			statusCode:       resp.StatusCode,
			downstreamModel:  meta.Model,
			upstreamModel:    selection.Hit.UpstreamModelName,
			channelName:      selection.Hit.ChannelName,
			channelType:      selection.Hit.ChannelType,
			upstreamKeyIndex: intPtr(selection.APIKeyEntryIndex),
			isStream:         meta.Stream,
			reasoningEffort:  meta.EffectiveReasoningEffort(),
			billingMode:      "token",
			attempts:         attempts,
			requestBody:      h.logRequestBody(cfg, body, false),
		}
		if meta.Stream {
			logInput.firstTokenMS = h.relay(c, resp, start)
			h.submitLog(c, logInput)
			return
		}
		responseBody, firstTokenMS, err := readAndClose(resp.Body, start)
		if err != nil {
			h.logger.Warn("failed to read upstream response", "error", err)
			logInput.statusCode = http.StatusBadGateway
			logInput.errorKind = "upstream_read_failed"
			logInput.errorMessage = err.Error()
			h.submitLog(c, logInput)
			auth.AbortOpenAIError(c, http.StatusBadGateway, "Failed to read upstream response.", "upstream_error", "bad_gateway")
			return
		}
		logInput.firstTokenMS = firstTokenMS
		logInput.promptTokens, logInput.completionTokens, logInput.reasoningTokens, logInput.totalTokens = parseUsage(responseBody)
		logInput.responseBody = h.logResponseBody(cfg, responseBody, false)
		h.writeBuffered(c, resp, responseBody)
		h.submitLog(c, logInput)
		return
	}
	h.writeFailure(c, last)
	status, _, code := classifyFailure(last)
	h.submitLog(c, requestLogInput{
		start:           start,
		endpoint:        c.Request.URL.Path,
		requestType:     endpoint.requestType,
		statusCode:      status,
		downstreamModel: meta.Model,
		isStream:        meta.Stream,
		reasoningEffort: meta.EffectiveReasoningEffort(),
		billingMode:     "token",
		errorKind:       code,
		errorMessage:    last.message,
		attempts:        attempts,
		requestBody:     h.logRequestBody(cfg, body, true),
		responseBody:    h.logResponseBody(cfg, []byte(last.message), true),
	})
}

func (h *Handler) rebuildIndex(cfg *config.Config) {
	h.index.Store(routeindex.NewIndex(cfg))
}

func (h *Handler) relay(c *gin.Context, resp *upstream.ChatResponse, start time.Time) *int64 {
	defer resp.Body.Close()
	copyResponseHeaders(c.Writer.Header(), resp.Header)
	c.Status(resp.StatusCode)
	reader := &firstByteReader{reader: resp.Body, start: start}
	if _, err := io.Copy(c.Writer, reader); err != nil {
		h.logger.Warn("failed to relay upstream response", "error", err)
	}
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return reader.firstMS
}

func (h *Handler) writeBuffered(c *gin.Context, resp *upstream.ChatResponse, body []byte) {
	copyResponseHeaders(c.Writer.Header(), resp.Header)
	c.Status(resp.StatusCode)
	if _, err := c.Writer.Write(body); err != nil {
		h.logger.Warn("failed to write upstream response", "error", err)
	}
}

func (h *Handler) writeFailure(c *gin.Context, failure upstreamFailure) {
	status, typ, code := classifyFailure(failure)
	message := "Upstream request failed."
	if failure.message != "" {
		message = failure.message
	}
	auth.AbortOpenAIError(c, status, message, typ, code)
}

type modelRequestMeta struct {
	Model           string `json:"model"`
	Stream          bool   `json:"stream"`
	ReasoningEffort string `json:"reasoning_effort"`
	Reasoning       struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
}

func (m modelRequestMeta) EffectiveReasoningEffort() string {
	if m.ReasoningEffort != "" {
		return m.ReasoningEffort
	}
	return m.Reasoning.Effort
}

func parseModelRequestMeta(body []byte) (modelRequestMeta, error) {
	var meta modelRequestMeta
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&meta); err != nil {
		return modelRequestMeta{}, errors.New("Request body must be valid JSON.")
	}
	if meta.Model == "" {
		return modelRequestMeta{}, errors.New("Request body must include model.")
	}
	return meta, nil
}

type upstreamFailure struct {
	status  int
	message string
	code    string
}

func readFailure(resp *upstream.ChatResponse) upstreamFailure {
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	return upstreamFailure{
		status:  resp.StatusCode,
		message: string(bytes.TrimSpace(data)),
	}
}

func classifyFailure(failure upstreamFailure) (int, string, string) {
	if failure.code != "" {
		return failure.status, "upstream_error", failure.code
	}
	switch failure.status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return http.StatusInternalServerError, "upstream_error", "auth_failed"
	case http.StatusTooManyRequests:
		return http.StatusTooManyRequests, "rate_limit_error", "rate_limit_exceeded"
	case http.StatusGatewayTimeout:
		return http.StatusGatewayTimeout, "upstream_error", "gateway_timeout"
	case http.StatusNotImplemented:
		return http.StatusNotImplemented, "upstream_error", "unsupported_channel"
	}
	if failure.status >= 500 {
		return http.StatusBadGateway, "upstream_error", "bad_gateway"
	}
	return http.StatusBadGateway, "upstream_error", "upstream_failed"
}

func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		switch key {
		case "Content-Length":
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

type requestLogInput struct {
	start            time.Time
	endpoint         string
	requestType      string
	statusCode       int
	downstreamModel  string
	upstreamModel    string
	channelName      string
	channelType      string
	upstreamKeyIndex *int
	isStream         bool
	firstTokenMS     *int64
	reasoningEffort  string
	billingMode      string
	promptTokens     *int64
	completionTokens *int64
	reasoningTokens  *int64
	totalTokens      *int64
	errorKind        string
	errorMessage     string
	requestBody      []byte
	responseBody     []byte
	attempts         int
}

func (h *Handler) submitLog(c *gin.Context, input requestLogInput) {
	if h.monitor == nil {
		return
	}
	if input.start.IsZero() {
		input.start = time.Now()
	}
	if input.attempts == 0 {
		input.attempts = 1
	}
	h.monitor.Submit(store.LogEntry{
		TimestampMS:      input.start.UnixMilli(),
		APIKeyTokenMask:  getString(c, "api_key_mask"),
		APIKeyName:       getString(c, "api_key_name"),
		Endpoint:         input.endpoint,
		RequestType:      input.requestType,
		ChannelName:      input.channelName,
		ChannelType:      input.channelType,
		DownstreamModel:  input.downstreamModel,
		UpstreamModel:    input.upstreamModel,
		UpstreamKeyIndex: input.upstreamKeyIndex,
		StatusCode:       input.statusCode,
		IsStream:         input.isStream,
		DurationMS:       time.Since(input.start).Milliseconds(),
		FirstTokenMS:     input.firstTokenMS,
		ReasoningEffort:  input.reasoningEffort,
		BillingMode:      input.billingMode,
		PromptTokens:     input.promptTokens,
		CompletionTokens: input.completionTokens,
		ReasoningTokens:  input.reasoningTokens,
		TotalTokens:      input.totalTokens,
		ErrorKind:        input.errorKind,
		ErrorMessage:     input.errorMessage,
		RequestBody:      input.requestBody,
		ResponseBody:     input.responseBody,
		Attempts:         input.attempts,
		UserAgent:        c.Request.UserAgent(),
	})
}

func intPtr(value int) *int {
	if value < 0 {
		return nil
	}
	return &value
}

func getString(c *gin.Context, key string) string {
	value, ok := c.Get(key)
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func readAndClose(body io.ReadCloser, start time.Time) ([]byte, *int64, error) {
	defer body.Close()
	reader := &firstByteReader{reader: body, start: start}
	data, err := io.ReadAll(reader)
	return data, reader.firstMS, err
}

type firstByteReader struct {
	reader  io.Reader
	start   time.Time
	firstMS *int64
}

func (r *firstByteReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.firstMS == nil {
		value := time.Since(r.start).Milliseconds()
		r.firstMS = &value
	}
	return n, err
}

func parseUsage(body []byte) (*int64, *int64, *int64, *int64) {
	var payload struct {
		Usage struct {
			PromptTokens        *int64 `json:"prompt_tokens"`
			CompletionTokens    *int64 `json:"completion_tokens"`
			InputTokens         *int64 `json:"input_tokens"`
			OutputTokens        *int64 `json:"output_tokens"`
			ReasoningTokens     *int64 `json:"reasoning_tokens"`
			OutputTokensDetails struct {
				ReasoningTokens *int64 `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
			CompletionTokensDetails struct {
				ReasoningTokens *int64 `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
			TotalTokens *int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, nil, nil, nil
	}
	promptTokens := payload.Usage.PromptTokens
	if promptTokens == nil {
		promptTokens = payload.Usage.InputTokens
	}
	completionTokens := payload.Usage.CompletionTokens
	if completionTokens == nil {
		completionTokens = payload.Usage.OutputTokens
	}
	reasoningTokens := payload.Usage.ReasoningTokens
	if reasoningTokens == nil {
		reasoningTokens = payload.Usage.OutputTokensDetails.ReasoningTokens
	}
	if reasoningTokens == nil {
		reasoningTokens = payload.Usage.CompletionTokensDetails.ReasoningTokens
	}
	return promptTokens, completionTokens, reasoningTokens, payload.Usage.TotalTokens
}

func (h *Handler) logRequestBody(cfg *config.Config, body []byte, failed bool) []byte {
	return bodyForMode(cfg, body, failed)
}

func (h *Handler) logResponseBody(cfg *config.Config, body []byte, failed bool) []byte {
	return bodyForMode(cfg, body, failed)
}

func bodyForMode(cfg *config.Config, body []byte, failed bool) []byte {
	if cfg == nil || len(body) == 0 {
		return nil
	}
	mode := cfg.EffectiveRequestLogBodyMode()
	if mode == config.BodyModeNone || (mode == config.BodyModeFailedOnly && !failed) {
		return nil
	}
	maxBytes := cfg.EffectiveRequestLogMaxBodyBytes()
	if maxBytes <= 0 {
		return nil
	}
	if len(body) > maxBytes {
		body = body[:maxBytes]
	}
	return append([]byte(nil), body...)
}
