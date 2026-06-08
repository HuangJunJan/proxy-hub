-- mcp queries. ASCII-only comments.

-- name: ListMcpServers :many
SELECT * FROM mcp_servers ORDER BY id;

-- name: GetMcpServer :one
SELECT * FROM mcp_servers WHERE id = ?;

-- name: UpsertMcpServer :exec
INSERT INTO mcp_servers (
    id, name, spec_json, description, homepage, docs, tags_json,
    enabled_codex, enabled_claude, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    spec_json = excluded.spec_json,
    description = excluded.description,
    homepage = excluded.homepage,
    docs = excluded.docs,
    tags_json = excluded.tags_json,
    enabled_codex = excluded.enabled_codex,
    enabled_claude = excluded.enabled_claude,
    updated_at = excluded.updated_at;

-- name: SetMcpServerToggle :exec
UPDATE mcp_servers SET enabled_codex = ?, enabled_claude = ?, updated_at = ? WHERE id = ?;

-- name: DeleteMcpServer :exec
DELETE FROM mcp_servers WHERE id = ?;

-- name: ListMcpSyncTargets :many
SELECT * FROM mcp_sync_targets ORDER BY id;

-- name: ListEnabledMcpSyncTargets :many
SELECT * FROM mcp_sync_targets WHERE enabled = 1 ORDER BY id;

-- name: GetMcpSyncTarget :one
SELECT * FROM mcp_sync_targets WHERE id = ?;

-- name: CreateMcpSyncTarget :one
INSERT INTO mcp_sync_targets (
    client, config_path, label, enabled, last_sync_status, created_at, updated_at
) VALUES (?, ?, ?, ?, '', ?, ?)
RETURNING *;

-- name: UpdateMcpSyncTarget :exec
UPDATE mcp_sync_targets SET client = ?, config_path = ?, label = ?, enabled = ?, updated_at = ? WHERE id = ?;

-- name: SetMcpSyncTargetStatus :exec
UPDATE mcp_sync_targets SET last_synced_at = ?, last_sync_status = ?, updated_at = ? WHERE id = ?;

-- name: DeleteMcpSyncTarget :exec
DELETE FROM mcp_sync_targets WHERE id = ?;
