# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

Proxy Hub uses SQLite for runtime data. YAML remains the source of truth for configuration, credentials, channels, and downstream API keys.

The implementation uses `database/sql` with the pure-Go `modernc.org/sqlite` driver.

---

## Query Patterns

### Scenario: Request Log and Channel Stats Repositories

#### 1. Scope / Trigger
- Trigger: runtime request observability spans proxy, monitor, SQLite, admin API, and frontend.

#### 2. Signatures
- `RequestLogRepo.BatchInsert(ctx, []store.LogEntry) error`
- `RequestLogRepo.Query(ctx, store.QueryFilter) ([]store.LogEntry, error)`
- `RequestLogRepo.DeleteBefore(ctx, ts int64) (int64, error)`
- `StatsRepo.UpsertHourly(ctx, []store.HourlyDelta) error`
- `StatsRepo.QueryChannelSummary(ctx, store.TimeWindow) ([]store.ChannelSummary, error)`
- `StatsRepo.QuerySeries(ctx, channelName, metric, window) ([]store.Point, error)`

#### 3. Contracts
- `request_logs.ts` and `channel_stats_hourly.hour_ts` are Unix milliseconds.
- Request logs store masked downstream key tokens only.
- `channel_name` is historical text; channel rename creates new future records.
- `channel_stats_hourly.avg_duration_ms` is a weighted average by request count.

#### 4. Validation & Error Matrix
- Empty insert batch -> no-op.
- Query limit <= 0 or > 500 -> coerced to 100.
- Unsupported stats metric -> error before SQL interpolation.
- Retention days <= 0 -> cleanup no-op.

#### 5. Good/Base/Bad Cases
- Good: proxy submits asynchronously; monitor batches writes and upserts stats.
- Base: admin logs endpoint reads through repository interfaces.
- Bad: proxy writes SQLite synchronously on the request path.

#### 6. Tests Required
- Migration idempotency/open store test.
- Batch insert + filtered query test.
- `DeleteBefore` retention test.
- Hourly upsert weighted average test.
- Proxy/server integration test asserting logs and stats reflect a request.

#### 7. Wrong vs Correct

Wrong:

```go
db.ExecContext(ctx, "INSERT INTO request_logs ...")
```

Correct:

```go
monitor.Submit(store.LogEntry{ChannelName: hit.ChannelName, StatusCode: status})
```

---

## Migrations

Migrations live under `internal/store/migrations/*.sql` and are embedded with `//go:embed`.

- Prefix migration files with an integer version, e.g. `0001_init.sql`.
- Record applied version in `meta` with key `schema_version`.
- Migrations must be idempotent where possible (`IF NOT EXISTS`) because tests open fresh stores frequently.
- SQLite is configured with WAL, `synchronous=NORMAL`, `busy_timeout=5000`, and `foreign_keys=ON`.

---

## Naming Conventions

- Table names use snake_case plural nouns: `request_logs`, `channel_stats_hourly`.
- Column names use snake_case and avoid JSON naming.
- Millisecond timestamps end with `_ts` in SQL and `TimestampMS` in Go.
- Index names use `idx_<table>_<columns>`.

---

## Common Mistakes

### Common Mistake: Forgetting to Drain Monitor Entries on Shutdown

**Symptom**: The process exits cleanly but the last few request logs are missing.

**Cause**: The monitor flushed only its current batch when the context was canceled and did not drain queued channel entries.

**Fix**: On `ctx.Done()`, drain `s.entries` until empty, then flush once.

**Prevention**: Monitor shutdown tests should submit entries and cancel before the flush tick when adding future queue behavior.
