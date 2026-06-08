-- request_logs queries. ASCII-only comments (sqlc parser).

-- name: InsertRequestLog :exec
INSERT INTO request_logs (
    request_id, created_at, api_key_id, channel_id, user_id, group_name,
    requested_model, upstream_model, endpoint_format, is_stream,
    input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_creation_tokens, total_tokens,
    latency_ms, first_token_ms, status_code, is_error, error_type, error_message, session_id, usage_source
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: ListRequestLogsFiltered :many
-- Optional filters via sqlc.narg (NULL = no filter). Paged, newest first.
SELECT * FROM request_logs
WHERE (sqlc.narg('request_id') IS NULL OR request_id = sqlc.narg('request_id'))
  AND (sqlc.narg('api_key_id') IS NULL OR api_key_id = sqlc.narg('api_key_id'))
  AND (sqlc.narg('channel_id') IS NULL OR channel_id = sqlc.narg('channel_id'))
  AND (sqlc.narg('requested_model') IS NULL OR requested_model = sqlc.narg('requested_model'))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountRequestLogsFiltered :one
SELECT CAST(COUNT(*) AS INTEGER) FROM request_logs
WHERE (sqlc.narg('request_id') IS NULL OR request_id = sqlc.narg('request_id'))
  AND (sqlc.narg('api_key_id') IS NULL OR api_key_id = sqlc.narg('api_key_id'))
  AND (sqlc.narg('channel_id') IS NULL OR channel_id = sqlc.narg('channel_id'))
  AND (sqlc.narg('requested_model') IS NULL OR requested_model = sqlc.narg('requested_model'));

-- name: BreakdownErrorTypeRange :many
-- Error-type breakdown comes from facts (rollups do not carry error_type).
SELECT
    error_type AS error_type,
    CAST(COUNT(*) AS INTEGER) AS request_count
FROM request_logs
WHERE created_at >= ? AND is_error = 1
GROUP BY error_type
ORDER BY COUNT(*) DESC;

-- name: DeleteRequestLogsBefore :execrows
DELETE FROM request_logs WHERE created_at < ?;
