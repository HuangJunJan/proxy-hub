-- Passive per (channel, model) health/cooldown. MarkResult upserts; selector reads
-- the in-memory mirror loaded via ListChannelModelHealth at startup.
-- ASCII comments on purpose (sqlc CJK parser limitation).

-- name: UpsertChannelModelHealth :exec
INSERT INTO channel_model_health (
    channel_id, model, is_healthy, consecutive_failures,
    last_success_at, last_failure_at, last_error, cooldown_until, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (channel_id, model) DO UPDATE SET
    is_healthy = excluded.is_healthy,
    consecutive_failures = excluded.consecutive_failures,
    last_success_at = excluded.last_success_at,
    last_failure_at = excluded.last_failure_at,
    last_error = excluded.last_error,
    cooldown_until = excluded.cooldown_until,
    updated_at = excluded.updated_at;

-- name: GetChannelModelHealth :one
SELECT * FROM channel_model_health WHERE channel_id = ? AND model = ?;

-- name: ListChannelModelHealth :many
SELECT * FROM channel_model_health;
