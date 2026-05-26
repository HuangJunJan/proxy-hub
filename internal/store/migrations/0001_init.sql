CREATE TABLE IF NOT EXISTS request_logs (
  id                  INTEGER PRIMARY KEY AUTOINCREMENT,
  ts                  INTEGER NOT NULL,
  api_key_token_mask  TEXT NOT NULL,
  api_key_name        TEXT,
  channel_name        TEXT,
  channel_type        TEXT,
  downstream_model    TEXT NOT NULL,
  upstream_model      TEXT,
  upstream_key_index  INTEGER,
  status_code         INTEGER NOT NULL,
  is_stream           INTEGER NOT NULL,
  duration_ms         INTEGER NOT NULL,
  prompt_tokens       INTEGER,
  completion_tokens   INTEGER,
  total_tokens        INTEGER,
  error_kind          TEXT,
  error_message       TEXT,
  request_body        BLOB,
  response_body       BLOB,
  attempts            INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_request_logs_ts ON request_logs(ts);
CREATE INDEX IF NOT EXISTS idx_request_logs_channel_ts ON request_logs(channel_name, ts);
CREATE INDEX IF NOT EXISTS idx_request_logs_status_ts ON request_logs(status_code, ts);

CREATE TABLE IF NOT EXISTS channel_stats_hourly (
  channel_name        TEXT NOT NULL,
  hour_ts             INTEGER NOT NULL,
  requests            INTEGER NOT NULL DEFAULT 0,
  successes           INTEGER NOT NULL DEFAULT 0,
  failures            INTEGER NOT NULL DEFAULT 0,
  prompt_tokens       INTEGER NOT NULL DEFAULT 0,
  completion_tokens   INTEGER NOT NULL DEFAULT 0,
  total_tokens        INTEGER NOT NULL DEFAULT 0,
  avg_duration_ms     INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (channel_name, hour_ts)
);

CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
