-- Channel CRUD. Reads use store.Read(); writes use store.Write().
-- NOTE: comments here are ASCII on purpose -- sqlc's SQLite parser mis-handles
-- multibyte (CJK) characters in query files. Chinese docs live in the Go dao layer.

-- name: CreateChannel :one
INSERT INTO channels (
    name, enabled, platform, type, base_url, group_name, priority, weight,
    models, model_mapping, prefix, proxy_url, status, error_message, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetChannel :one
SELECT * FROM channels WHERE id = ?;

-- name: ListChannels :many
SELECT * FROM channels ORDER BY priority DESC, id ASC;

-- name: ListEnabledChannels :many
SELECT * FROM channels WHERE enabled = 1 ORDER BY priority DESC, id ASC;

-- name: UpdateChannel :one
UPDATE channels SET
    name = ?, enabled = ?, platform = ?, type = ?, base_url = ?, group_name = ?,
    priority = ?, weight = ?, models = ?, model_mapping = ?, prefix = ?, proxy_url = ?,
    status = ?, error_message = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: SetChannelStatus :exec
UPDATE channels SET status = ?, error_message = ?, updated_at = ? WHERE id = ?;

-- name: DeleteChannel :exec
DELETE FROM channels WHERE id = ?;
