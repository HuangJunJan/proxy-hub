-- Migration 0003: stats tables for usage logging, rollups and model pricing (M3).
-- ASCII-only comments (sqlc parser + project .sql convention). Money lives only in model_pricing;
-- request_logs / rollups store tokens and latency only -- cost is computed at read time.

-- request_logs: append-only fact table for drill-down and tracing. No cost columns; no request/response bodies.
CREATE TABLE request_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,
    created_at TEXT NOT NULL,                 -- RFC3339 UTC
    api_key_id INTEGER NOT NULL DEFAULT 0,
    channel_id INTEGER NOT NULL DEFAULT 0,
    user_id INTEGER NOT NULL DEFAULT 0,       -- reserved (OQ-4); M3 writes 0
    group_name TEXT NOT NULL DEFAULT 'default',
    requested_model TEXT NOT NULL,            -- client-facing name (may include prefix)
    upstream_model TEXT NOT NULL DEFAULT '',
    endpoint_format TEXT NOT NULL,            -- openai|claude|responses
    is_stream INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    first_token_ms INTEGER,                   -- NULL = unknown (TTFT)
    status_code INTEGER NOT NULL DEFAULT 0,
    is_error INTEGER NOT NULL DEFAULT 0,
    error_type TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',   -- truncated (<=256B)
    session_id TEXT NOT NULL DEFAULT '',
    usage_source TEXT NOT NULL DEFAULT ''     -- stream|usage_block|estimated|missing
);

CREATE INDEX idx_request_logs_created_at ON request_logs(created_at);
CREATE INDEX idx_request_logs_api_key_created ON request_logs(api_key_id, created_at);
CREATE INDEX idx_request_logs_channel_created ON request_logs(channel_id, created_at);
CREATE INDEX idx_request_logs_model_created ON request_logs(requested_model, created_at);
CREATE INDEX idx_request_logs_status ON request_logs(status_code);
CREATE INDEX idx_request_logs_session ON request_logs(session_id);
CREATE INDEX idx_request_logs_request_id ON request_logs(request_id);

-- usage_hourly_rollups: pre-aggregated time series (hour grain). Tokens/latency only.
CREATE TABLE usage_hourly_rollups (
    bucket_hour TEXT NOT NULL,                -- RFC3339 UTC truncated to the hour
    channel_id INTEGER NOT NULL,
    api_key_id INTEGER NOT NULL,
    requested_model TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    sum_latency_ms INTEGER NOT NULL DEFAULT 0,
    sum_first_token_ms INTEGER NOT NULL DEFAULT 0,
    count_first_token INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket_hour, channel_id, api_key_id, requested_model)
);

CREATE INDEX idx_hourly_bucket ON usage_hourly_rollups(bucket_hour);

-- usage_daily_rollups: pre-aggregated time series (day grain).
CREATE TABLE usage_daily_rollups (
    bucket_date TEXT NOT NULL,                -- YYYY-MM-DD (UTC)
    channel_id INTEGER NOT NULL,
    requested_model TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    sum_latency_ms INTEGER NOT NULL DEFAULT 0,
    sum_first_token_ms INTEGER NOT NULL DEFAULT 0,
    count_first_token INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket_date, channel_id, requested_model)
);

CREATE INDEX idx_daily_bucket ON usage_daily_rollups(bucket_date);

-- model_pricing: config-driven; per-million prices stored as decimal TEXT. Cost computed at read time.
CREATE TABLE model_pricing (
    model_id TEXT PRIMARY KEY,
    input_per_million TEXT NOT NULL DEFAULT '0',
    output_per_million TEXT NOT NULL DEFAULT '0',
    cache_read_per_million TEXT NOT NULL DEFAULT '0',
    cache_creation_per_million TEXT NOT NULL DEFAULT '0',
    source TEXT NOT NULL DEFAULT 'seed',      -- seed|admin
    updated_at TEXT NOT NULL
);
