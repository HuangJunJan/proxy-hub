-- Migration 0004: MCP server registry (SSOT) and explicit sync targets (M4).
-- ASCII-only comments (sqlc parser + project .sql convention).

-- mcp_servers: single source of truth. id = config key (server name in client files).
CREATE TABLE mcp_servers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    spec_json TEXT NOT NULL,                  -- lenient canonical spec, stored as-is (unknown fields preserved)
    description TEXT NOT NULL DEFAULT '',
    homepage TEXT NOT NULL DEFAULT '',
    docs TEXT NOT NULL DEFAULT '',
    tags_json TEXT NOT NULL DEFAULT '[]',
    enabled_codex INTEGER NOT NULL DEFAULT 0,
    enabled_claude INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- mcp_sync_targets: operator-registered absolute client config paths (no HOME auto-detect).
CREATE TABLE mcp_sync_targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client TEXT NOT NULL CHECK(client IN ('codex','claude')),
    config_path TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    last_synced_at TEXT,
    last_sync_status TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_mcp_sync_targets_client ON mcp_sync_targets(client);
