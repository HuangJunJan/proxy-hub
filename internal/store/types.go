package store

import "context"

type RequestLogRepo interface {
	BatchInsert(ctx context.Context, entries []LogEntry) error
	Query(ctx context.Context, filter QueryFilter) ([]LogEntry, error)
	DeleteBefore(ctx context.Context, ts int64) (int64, error)
}

type StatsRepo interface {
	UpsertHourly(ctx context.Context, deltas []HourlyDelta) error
	QueryChannelSummary(ctx context.Context, window TimeWindow) ([]ChannelSummary, error)
	QuerySeries(ctx context.Context, channelName string, metric Metric, window TimeWindow) ([]Point, error)
}

type LogEntry struct {
	ID               int64  `json:"id"`
	TimestampMS      int64  `json:"ts"`
	APIKeyTokenMask  string `json:"apiKeyTokenMask"`
	APIKeyName       string `json:"apiKeyName,omitempty"`
	Endpoint         string `json:"endpoint,omitempty"`
	RequestType      string `json:"requestType,omitempty"`
	ChannelName      string `json:"channelName,omitempty"`
	ChannelType      string `json:"channelType,omitempty"`
	DownstreamModel  string `json:"downstreamModel"`
	UpstreamModel    string `json:"upstreamModel,omitempty"`
	UpstreamKeyIndex *int   `json:"upstreamKeyIndex,omitempty"`
	StatusCode       int    `json:"statusCode"`
	IsStream         bool   `json:"isStream"`
	DurationMS       int64  `json:"durationMs"`
	FirstTokenMS     *int64 `json:"firstTokenMs,omitempty"`
	ReasoningEffort  string `json:"reasoningEffort,omitempty"`
	BillingMode      string `json:"billingMode,omitempty"`
	PromptTokens     *int64 `json:"promptTokens,omitempty"`
	CompletionTokens *int64 `json:"completionTokens,omitempty"`
	ReasoningTokens  *int64 `json:"reasoningTokens,omitempty"`
	TotalTokens      *int64 `json:"totalTokens,omitempty"`
	ErrorKind        string `json:"errorKind,omitempty"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
	RequestBody      []byte `json:"requestBody,omitempty"`
	ResponseBody     []byte `json:"responseBody,omitempty"`
	Attempts         int    `json:"attempts"`
	UserAgent        string `json:"userAgent,omitempty"`
}

type QueryFilter struct {
	ChannelName string
	Model       string
	Status      string
	StartMS     int64
	EndMS       int64
	Limit       int
	Offset      int
}

type TimeWindow struct {
	StartMS int64
	EndMS   int64
}

type HourlyDelta struct {
	ChannelName      string `json:"channelName"`
	HourTimestampMS  int64  `json:"hourTs"`
	Requests         int64  `json:"requests"`
	Successes        int64  `json:"successes"`
	Failures         int64  `json:"failures"`
	PromptTokens     int64  `json:"promptTokens"`
	CompletionTokens int64  `json:"completionTokens"`
	TotalTokens      int64  `json:"totalTokens"`
	AvgDurationMS    int64  `json:"avgDurationMs"`
}

type ChannelSummary struct {
	ChannelName      string `json:"channelName"`
	Requests         int64  `json:"requests"`
	Successes        int64  `json:"successes"`
	Failures         int64  `json:"failures"`
	PromptTokens     int64  `json:"promptTokens"`
	CompletionTokens int64  `json:"completionTokens"`
	TotalTokens      int64  `json:"totalTokens"`
	AvgDurationMS    int64  `json:"avgDurationMs"`
}

type Point struct {
	TimestampMS int64 `json:"ts"`
	Value       int64 `json:"value"`
}

type Metric string

const (
	MetricRequests         Metric = "requests"
	MetricSuccesses        Metric = "successes"
	MetricFailures         Metric = "failures"
	MetricPromptTokens     Metric = "prompt_tokens"
	MetricCompletionTokens Metric = "completion_tokens"
	MetricTotalTokens      Metric = "total_tokens"
	MetricAvgDurationMS    Metric = "avg_duration_ms"
)
