-- 0002_channels: channel management + routing index + inbound keys + passive health (M2, capability C1).
-- Conventions: time columns store RFC3339 text; JSON stored as TEXT; credential blobs are NOT in DB
-- (upstream keys live in data/auths/<id>.json; inbound keys store only the sha256 hash).
-- ASCII comments on purpose -- sqlc's SQLite (ANTLR) parser mis-handles multibyte (CJK) bytes.
-- Full Chinese schema docs live in the task design.md section 2 and the Go dao layer.

-- channels: upstream channel metadata (credentials stored separately in files).
CREATE TABLE IF NOT EXISTS channels (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL,
    enabled       INTEGER NOT NULL DEFAULT 1,
    platform      TEXT    NOT NULL CHECK (platform IN ('openai', 'anthropic')),
    type          TEXT    NOT NULL CHECK (type IN ('api_key', 'upstream')),
    base_url      TEXT    NOT NULL DEFAULT '',          -- empty => use platform default endpoint
    group_name    TEXT    NOT NULL DEFAULT 'default',   -- 'group' is a SQL keyword; column is group_name
    priority      INTEGER NOT NULL DEFAULT 50,
    weight        INTEGER NOT NULL DEFAULT 1,
    models        TEXT    NOT NULL DEFAULT '[]',        -- JSON array: upstream model names this channel serves
    model_mapping TEXT    NOT NULL DEFAULT '{}',        -- JSON: alias_model -> upstream_model (trailing * allowed)
    prefix        TEXT    NOT NULL DEFAULT '',          -- client-facing namespace prefix
    proxy_url     TEXT    NOT NULL DEFAULT '',          -- egress proxy (optional)
    status        TEXT    NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'error', 'disabled')),
    error_message TEXT    NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_channels_enabled ON channels (enabled);

-- abilities: derived routing index (new-api style). Persistent DB mirror; hot path uses in-memory RouteIndex.
-- alias_model is strictly the client-facing key (may contain trailing *); upstream_model stored alongside.
CREATE TABLE IF NOT EXISTS abilities (
    group_name     TEXT    NOT NULL,
    alias_model    TEXT    NOT NULL,
    channel_id     INTEGER NOT NULL,
    upstream_model TEXT    NOT NULL,
    priority       INTEGER NOT NULL,
    weight         INTEGER NOT NULL,
    enabled        INTEGER NOT NULL,
    PRIMARY KEY (group_name, alias_model, channel_id),
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_abilities_lookup ON abilities (group_name, alias_model);

-- api_keys: platform-issued inbound keys. Plaintext shown once at creation; DB stores only sha256 hash.
CREATE TABLE IF NOT EXISTS api_keys (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    hash         TEXT    NOT NULL UNIQUE,             -- sha256(plaintext), hex
    name         TEXT    NOT NULL DEFAULT '',
    group_name   TEXT    NOT NULL DEFAULT 'default',
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT    NOT NULL,
    last_used_at TEXT                                  -- NULL allowed
);

-- channel_model_health: passive health/cooldown per (channel, model). Written by MarkResult; read by selector + dashboard.
CREATE TABLE IF NOT EXISTS channel_model_health (
    channel_id           INTEGER NOT NULL,
    model                TEXT    NOT NULL,             -- upstream_model
    is_healthy           INTEGER NOT NULL DEFAULT 1,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_success_at      TEXT,
    last_failure_at      TEXT,
    last_error           TEXT    NOT NULL DEFAULT '',
    cooldown_until       TEXT,                          -- NULL/past => not cooling down
    updated_at           TEXT    NOT NULL,
    PRIMARY KEY (channel_id, model),
    FOREIGN KEY (channel_id) REFERENCES channels (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_cmh_cooldown ON channel_model_health (cooldown_until);
