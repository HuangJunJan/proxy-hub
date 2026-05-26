package upstream

import (
	"context"
	"io"
	"net/http"
	"time"
)

type Adapter interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	Responses(ctx context.Context, req ResponsesRequest) (*ChatResponse, error)
	Models(ctx context.Context, req ModelsRequest) ([]string, error)
	HealthCheck(ctx context.Context, req HealthCheckRequest) error
}

type ChatRequest struct {
	BaseURL           string
	APIKey            string
	UpstreamModelName string
	OriginalBody      []byte
	Stream            bool
	Headers           http.Header
	Timeout           time.Duration
}

type ChatResponse struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
	Stream     bool
}

type ResponsesRequest struct {
	BaseURL           string
	APIKey            string
	UpstreamModelName string
	OriginalBody      []byte
	Stream            bool
	Headers           http.Header
	Timeout           time.Duration
}

type ModelsRequest struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

type HealthCheckRequest struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}
