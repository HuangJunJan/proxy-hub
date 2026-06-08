-- usage rollup queries. ASCII-only comments. Aggregates CAST to INTEGER for clean int64 generation.

-- name: UpsertHourlyRollup :exec
INSERT INTO usage_hourly_rollups (
    bucket_hour, channel_id, api_key_id, requested_model,
    request_count, success_count, error_count,
    input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, reasoning_tokens,
    sum_latency_ms, sum_first_token_ms, count_first_token
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(bucket_hour, channel_id, api_key_id, requested_model) DO UPDATE SET
    request_count = request_count + excluded.request_count,
    success_count = success_count + excluded.success_count,
    error_count = error_count + excluded.error_count,
    input_tokens = input_tokens + excluded.input_tokens,
    output_tokens = output_tokens + excluded.output_tokens,
    cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
    cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
    reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
    sum_latency_ms = sum_latency_ms + excluded.sum_latency_ms,
    sum_first_token_ms = sum_first_token_ms + excluded.sum_first_token_ms,
    count_first_token = count_first_token + excluded.count_first_token;

-- name: UpsertDailyRollup :exec
INSERT INTO usage_daily_rollups (
    bucket_date, channel_id, requested_model,
    request_count, success_count, error_count,
    input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, reasoning_tokens,
    sum_latency_ms, sum_first_token_ms, count_first_token
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(bucket_date, channel_id, requested_model) DO UPDATE SET
    request_count = request_count + excluded.request_count,
    success_count = success_count + excluded.success_count,
    error_count = error_count + excluded.error_count,
    input_tokens = input_tokens + excluded.input_tokens,
    output_tokens = output_tokens + excluded.output_tokens,
    cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
    cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
    reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
    sum_latency_ms = sum_latency_ms + excluded.sum_latency_ms,
    sum_first_token_ms = sum_first_token_ms + excluded.sum_first_token_ms,
    count_first_token = count_first_token + excluded.count_first_token;

-- name: SumHourlyRange :one
SELECT
    CAST(COALESCE(SUM(request_count), 0) AS INTEGER) AS request_count,
    CAST(COALESCE(SUM(success_count), 0) AS INTEGER) AS success_count,
    CAST(COALESCE(SUM(error_count), 0) AS INTEGER) AS error_count,
    CAST(COALESCE(SUM(input_tokens), 0) AS INTEGER) AS input_tokens,
    CAST(COALESCE(SUM(output_tokens), 0) AS INTEGER) AS output_tokens,
    CAST(COALESCE(SUM(cache_read_tokens), 0) AS INTEGER) AS cache_read_tokens,
    CAST(COALESCE(SUM(cache_creation_tokens), 0) AS INTEGER) AS cache_creation_tokens,
    CAST(COALESCE(SUM(reasoning_tokens), 0) AS INTEGER) AS reasoning_tokens,
    CAST(COALESCE(SUM(sum_latency_ms), 0) AS INTEGER) AS sum_latency_ms,
    CAST(COALESCE(SUM(sum_first_token_ms), 0) AS INTEGER) AS sum_first_token_ms,
    CAST(COALESCE(SUM(count_first_token), 0) AS INTEGER) AS count_first_token
FROM usage_hourly_rollups
WHERE bucket_hour >= ?;

-- name: TimeseriesHourly :many
SELECT
    bucket_hour AS bucket,
    CAST(COALESCE(SUM(request_count), 0) AS INTEGER) AS request_count,
    CAST(COALESCE(SUM(success_count), 0) AS INTEGER) AS success_count,
    CAST(COALESCE(SUM(error_count), 0) AS INTEGER) AS error_count,
    CAST(COALESCE(SUM(input_tokens), 0) AS INTEGER) AS input_tokens,
    CAST(COALESCE(SUM(output_tokens), 0) AS INTEGER) AS output_tokens,
    CAST(COALESCE(SUM(sum_latency_ms), 0) AS INTEGER) AS sum_latency_ms,
    CAST(COALESCE(SUM(sum_first_token_ms), 0) AS INTEGER) AS sum_first_token_ms,
    CAST(COALESCE(SUM(count_first_token), 0) AS INTEGER) AS count_first_token
FROM usage_hourly_rollups
WHERE bucket_hour >= ?
GROUP BY bucket_hour
ORDER BY bucket_hour;

-- name: TimeseriesDaily :many
SELECT
    bucket_date AS bucket,
    CAST(COALESCE(SUM(request_count), 0) AS INTEGER) AS request_count,
    CAST(COALESCE(SUM(success_count), 0) AS INTEGER) AS success_count,
    CAST(COALESCE(SUM(error_count), 0) AS INTEGER) AS error_count,
    CAST(COALESCE(SUM(input_tokens), 0) AS INTEGER) AS input_tokens,
    CAST(COALESCE(SUM(output_tokens), 0) AS INTEGER) AS output_tokens,
    CAST(COALESCE(SUM(sum_latency_ms), 0) AS INTEGER) AS sum_latency_ms,
    CAST(COALESCE(SUM(sum_first_token_ms), 0) AS INTEGER) AS sum_first_token_ms,
    CAST(COALESCE(SUM(count_first_token), 0) AS INTEGER) AS count_first_token
FROM usage_daily_rollups
WHERE bucket_date >= ?
GROUP BY bucket_date
ORDER BY bucket_date;

-- name: BreakdownHourlyByModel :many
SELECT
    requested_model AS dim,
    CAST(COALESCE(SUM(request_count), 0) AS INTEGER) AS request_count,
    CAST(COALESCE(SUM(error_count), 0) AS INTEGER) AS error_count,
    CAST(COALESCE(SUM(input_tokens), 0) AS INTEGER) AS input_tokens,
    CAST(COALESCE(SUM(output_tokens), 0) AS INTEGER) AS output_tokens,
    CAST(COALESCE(SUM(cache_read_tokens), 0) AS INTEGER) AS cache_read_tokens,
    CAST(COALESCE(SUM(cache_creation_tokens), 0) AS INTEGER) AS cache_creation_tokens,
    CAST(COALESCE(SUM(reasoning_tokens), 0) AS INTEGER) AS reasoning_tokens,
    CAST(COALESCE(SUM(sum_latency_ms), 0) AS INTEGER) AS sum_latency_ms
FROM usage_hourly_rollups
WHERE bucket_hour >= ?
GROUP BY requested_model
ORDER BY SUM(input_tokens) + SUM(output_tokens) DESC;

-- name: BreakdownHourlyByChannel :many
SELECT
    CAST(channel_id AS INTEGER) AS dim,
    CAST(COALESCE(SUM(request_count), 0) AS INTEGER) AS request_count,
    CAST(COALESCE(SUM(error_count), 0) AS INTEGER) AS error_count,
    CAST(COALESCE(SUM(input_tokens), 0) AS INTEGER) AS input_tokens,
    CAST(COALESCE(SUM(output_tokens), 0) AS INTEGER) AS output_tokens,
    CAST(COALESCE(SUM(cache_read_tokens), 0) AS INTEGER) AS cache_read_tokens,
    CAST(COALESCE(SUM(cache_creation_tokens), 0) AS INTEGER) AS cache_creation_tokens,
    CAST(COALESCE(SUM(reasoning_tokens), 0) AS INTEGER) AS reasoning_tokens,
    CAST(COALESCE(SUM(sum_latency_ms), 0) AS INTEGER) AS sum_latency_ms
FROM usage_hourly_rollups
WHERE bucket_hour >= ?
GROUP BY channel_id
ORDER BY SUM(input_tokens) + SUM(output_tokens) DESC;

-- name: BreakdownHourlyByApiKey :many
SELECT
    CAST(api_key_id AS INTEGER) AS dim,
    CAST(COALESCE(SUM(request_count), 0) AS INTEGER) AS request_count,
    CAST(COALESCE(SUM(error_count), 0) AS INTEGER) AS error_count,
    CAST(COALESCE(SUM(input_tokens), 0) AS INTEGER) AS input_tokens,
    CAST(COALESCE(SUM(output_tokens), 0) AS INTEGER) AS output_tokens,
    CAST(COALESCE(SUM(cache_read_tokens), 0) AS INTEGER) AS cache_read_tokens,
    CAST(COALESCE(SUM(cache_creation_tokens), 0) AS INTEGER) AS cache_creation_tokens,
    CAST(COALESCE(SUM(reasoning_tokens), 0) AS INTEGER) AS reasoning_tokens,
    CAST(COALESCE(SUM(sum_latency_ms), 0) AS INTEGER) AS sum_latency_ms
FROM usage_hourly_rollups
WHERE bucket_hour >= ?
GROUP BY api_key_id
ORDER BY SUM(input_tokens) + SUM(output_tokens) DESC;
