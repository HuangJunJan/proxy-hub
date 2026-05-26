package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"proxy-hub/internal/upstream"
)

type Adapter struct {
	client *http.Client
}

func New(client *http.Client) *Adapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Adapter{client: client}
}

func (a *Adapter) Chat(ctx context.Context, req upstream.ChatRequest) (*upstream.ChatResponse, error) {
	return a.postModelRequest(ctx, modelRequest{
		operation:         "chat",
		path:              "/v1/chat/completions",
		baseURL:           req.BaseURL,
		apiKey:            req.APIKey,
		upstreamModelName: req.UpstreamModelName,
		originalBody:      req.OriginalBody,
		stream:            req.Stream,
		headers:           req.Headers,
		timeout:           req.Timeout,
	})
}

func (a *Adapter) Responses(ctx context.Context, req upstream.ResponsesRequest) (*upstream.ChatResponse, error) {
	return a.postModelRequest(ctx, modelRequest{
		operation:         "responses",
		path:              "/v1/responses",
		baseURL:           req.BaseURL,
		apiKey:            req.APIKey,
		upstreamModelName: req.UpstreamModelName,
		originalBody:      req.OriginalBody,
		stream:            req.Stream,
		headers:           req.Headers,
		timeout:           req.Timeout,
	})
}

type modelRequest struct {
	operation         string
	path              string
	baseURL           string
	apiKey            string
	upstreamModelName string
	originalBody      []byte
	stream            bool
	headers           http.Header
	timeout           time.Duration
}

func (a *Adapter) postModelRequest(ctx context.Context, req modelRequest) (*upstream.ChatResponse, error) {
	if strings.TrimSpace(req.apiKey) == "" {
		return nil, errors.New("api key is required")
	}
	body, err := ReplaceModel(req.originalBody, req.upstreamModelName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(ctx, req.timeout)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, joinBaseURL(req.baseURL, req.path), bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create upstream %s request: %w", req.operation, err)
	}
	copyHeaders(httpReq.Header, req.headers)
	httpReq.Header.Set("Authorization", "Bearer "+req.apiKey)
	if httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Accept", acceptHeader(req.stream))

	resp, err := a.client.Do(httpReq)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("send upstream %s request: %w", req.operation, err)
	}
	return &upstream.ChatResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       cancelReadCloser{ReadCloser: resp.Body, cancel: cancel},
		Stream:     req.stream,
	}, nil
}

func (a *Adapter) Models(ctx context.Context, req upstream.ModelsRequest) ([]string, error) {
	if strings.TrimSpace(req.APIKey) == "" {
		return nil, errors.New("api key is required")
	}
	ctx, cancel := withTimeout(ctx, req.Timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, joinBaseURL(req.BaseURL, "/v1/models"), nil)
	if err != nil {
		return nil, fmt.Errorf("create upstream models request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("Accept", "application/json")
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send upstream models request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream models returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var payload modelsPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode upstream models response: %w", err)
	}
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			models = append(models, item.ID)
		}
	}
	return models, nil
}

func (a *Adapter) HealthCheck(ctx context.Context, req upstream.HealthCheckRequest) error {
	_, err := a.Models(ctx, upstream.ModelsRequest{
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
		Timeout: req.Timeout,
	})
	return err
}

func ReplaceModel(body []byte, model string) ([]byte, error) {
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("upstream model name is required")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode model request body: %w", err)
	}
	payload["model"] = model
	next, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode chat request body: %w", err)
	}
	return next, nil
}

func joinBaseURL(baseURL, path string) string {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return strings.TrimRight(baseURL, "/") + path
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	if strings.HasPrefix(path, "/v1/") && strings.HasSuffix(basePath, "/v1") {
		path = strings.TrimPrefix(path, "/v1")
	}
	parsed.Path = basePath + path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		switch strings.ToLower(key) {
		case "authorization", "host", "content-length", "accept-encoding", "connection", "keep-alive", "proxy-connection", "te", "trailer", "transfer-encoding", "upgrade":
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func acceptHeader(stream bool) string {
	if stream {
		return "text/event-stream"
	}
	return "application/json"
}

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

type modelsPayload struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
