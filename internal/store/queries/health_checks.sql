-- health_check_logs queries (active probe). ASCII-only comments.

-- name: InsertHealthCheck :exec
INSERT INTO health_check_logs (
    channel_id, model, success, http_status, response_time_ms, message, checked_at
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListRecentHealthChecks :many
SELECT * FROM health_check_logs ORDER BY checked_at DESC, id DESC LIMIT ?;

-- name: DeleteHealthChecksBefore :execrows
DELETE FROM health_check_logs WHERE checked_at < ?;
