-- Inbound API keys. Only the sha256 hash is stored; GetAPIKeyByHash backs auth.
-- ASCII comments on purpose (sqlc CJK parser limitation).

-- name: CreateAPIKey :one
INSERT INTO api_keys (hash, name, group_name, enabled, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys WHERE hash = ?;

-- name: ListAPIKeys :many
SELECT * FROM api_keys ORDER BY id ASC;

-- name: SetAPIKeyEnabled :exec
UPDATE api_keys SET enabled = ? WHERE id = ?;

-- name: TouchAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = ? WHERE id = ?;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys WHERE id = ?;
