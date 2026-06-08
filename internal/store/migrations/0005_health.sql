-- Migration 0005: active health probe log (M5; optional feature, prober default-off).
-- ASCII-only comments.
CREATE TABLE health_check_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id INTEGER NOT NULL,
    model TEXT NOT NULL,
    success INTEGER NOT NULL DEFAULT 0,
    http_status INTEGER NOT NULL DEFAULT 0,
    response_time_ms INTEGER NOT NULL DEFAULT 0,
    message TEXT NOT NULL DEFAULT '',
    checked_at TEXT NOT NULL                  -- RFC3339 UTC
);

CREATE INDEX idx_health_check_logs_channel_checked ON health_check_logs(channel_id, checked_at);
CREATE INDEX idx_health_check_logs_checked ON health_check_logs(checked_at);
