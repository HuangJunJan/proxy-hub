package stats

import (
	"context"
	"database/sql"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/store"
	"github.com/huangjunjan/proxy-hub/internal/store/dbgen"
	"github.com/huangjunjan/proxy-hub/internal/usage"
)

// DAO 包裹 dbgen + store 双句柄，提供事实表批量插入、汇总 UPSERT、仪表盘查询、保留清理与定价持久化。
type DAO struct {
	st *store.Store
}

// NewDAO 创建统计 DAO。
func NewDAO(st *store.Store) *DAO { return &DAO{st: st} }

func (d *DAO) read() *dbgen.Queries { return dbgen.New(d.st.Read()) }

// LogFilter 是请求日志钻取的可选过滤（零值表示该维度不过滤）。
type LogFilter struct {
	RequestID string
	APIKeyID  int64
	ChannelID int64
	Model     string
	Limit     int64
	Offset    int64
}

// InsertLogsBatch 在单事务内批量插入事实行（采集器调用）。空切片为 no-op。
func (d *DAO) InsertLogsBatch(ctx context.Context, events []usage.Event) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := d.st.Write().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := dbgen.New(tx)
	for i := range events {
		if err := q.InsertRequestLog(ctx, toLogParams(&events[i])); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpsertHourly 在单事务内累加小时汇总（ON CONFLICT 累加）。
func (d *DAO) UpsertHourly(ctx context.Context, rows []dbgen.UpsertHourlyRollupParams) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := d.st.Write().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := dbgen.New(tx)
	for i := range rows {
		if err := q.UpsertHourlyRollup(ctx, rows[i]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpsertDaily 在单事务内累加日汇总。
func (d *DAO) UpsertDaily(ctx context.Context, rows []dbgen.UpsertDailyRollupParams) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := d.st.Write().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := dbgen.New(tx)
	for i := range rows {
		if err := q.UpsertDailyRollup(ctx, rows[i]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// 仪表盘查询（只读句柄；聚合在 SQL 内）。

func (d *DAO) SumHourlyRange(ctx context.Context, since string) (dbgen.SumHourlyRangeRow, error) {
	return d.read().SumHourlyRange(ctx, since)
}

func (d *DAO) TimeseriesHourly(ctx context.Context, since string) ([]dbgen.TimeseriesHourlyRow, error) {
	return d.read().TimeseriesHourly(ctx, since)
}

func (d *DAO) TimeseriesDaily(ctx context.Context, since string) ([]dbgen.TimeseriesDailyRow, error) {
	return d.read().TimeseriesDaily(ctx, since)
}

func (d *DAO) BreakdownByModel(ctx context.Context, since string) ([]dbgen.BreakdownHourlyByModelRow, error) {
	return d.read().BreakdownHourlyByModel(ctx, since)
}

func (d *DAO) BreakdownByChannel(ctx context.Context, since string) ([]dbgen.BreakdownHourlyByChannelRow, error) {
	return d.read().BreakdownHourlyByChannel(ctx, since)
}

func (d *DAO) BreakdownByAPIKey(ctx context.Context, since string) ([]dbgen.BreakdownHourlyByApiKeyRow, error) {
	return d.read().BreakdownHourlyByApiKey(ctx, since)
}

func (d *DAO) BreakdownByErrorType(ctx context.Context, since string) ([]dbgen.BreakdownErrorTypeRangeRow, error) {
	return d.read().BreakdownErrorTypeRange(ctx, since)
}

func (d *DAO) ListLogs(ctx context.Context, f LogFilter) ([]dbgen.RequestLog, error) {
	return d.read().ListRequestLogsFiltered(ctx, dbgen.ListRequestLogsFilteredParams{
		RequestID:      nilIfEmpty(f.RequestID),
		ApiKeyID:       nilIfZero(f.APIKeyID),
		ChannelID:      nilIfZero(f.ChannelID),
		RequestedModel: nilIfEmpty(f.Model),
		Off:            f.Offset,
		Lim:            f.Limit,
	})
}

func (d *DAO) CountLogs(ctx context.Context, f LogFilter) (int64, error) {
	return d.read().CountRequestLogsFiltered(ctx, dbgen.CountRequestLogsFilteredParams{
		RequestID:      nilIfEmpty(f.RequestID),
		ApiKeyID:       nilIfZero(f.APIKeyID),
		ChannelID:      nilIfZero(f.ChannelID),
		RequestedModel: nilIfEmpty(f.Model),
	})
}

// CleanupRawLogs 删除早于 before（RFC3339）的原始事实行，返回删除行数（汇总不删）。
func (d *DAO) CleanupRawLogs(ctx context.Context, before string) (int64, error) {
	return dbgen.New(d.st.Write()).DeleteRequestLogsBefore(ctx, before)
}

// 定价持久化。

// SeedPricing 以 SeedModelPricing（ON CONFLICT DO NOTHING）种入内置定价，绝不覆盖 admin 改价。
func (d *DAO) SeedPricing(ctx context.Context, entries []SeedEntry, nowRFC3339 string) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := d.st.Write().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := dbgen.New(tx)
	for _, e := range entries {
		if err := q.SeedModelPricing(ctx, dbgen.SeedModelPricingParams{
			ModelID:                 e.ModelID,
			InputPerMillion:         e.InputPerMillion,
			OutputPerMillion:        e.OutputPerMillion,
			CacheReadPerMillion:     e.CacheReadPerMillion,
			CacheCreationPerMillion: e.CacheCreationPerMillion,
			UpdatedAt:               nowRFC3339,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListPricing 返回全部定价行（含 source）。
func (d *DAO) ListPricing(ctx context.Context) ([]dbgen.ModelPricing, error) {
	return d.read().ListModelPricing(ctx)
}

// PriceRows 返回供 pricing.Table.Load 用的精简定价行。
func (d *DAO) PriceRows(ctx context.Context) ([]PriceRow, error) {
	rows, err := d.read().ListModelPricing(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PriceRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, PriceRow{
			ModelID:                 r.ModelID,
			InputPerMillion:         r.InputPerMillion,
			OutputPerMillion:        r.OutputPerMillion,
			CacheReadPerMillion:     r.CacheReadPerMillion,
			CacheCreationPerMillion: r.CacheCreationPerMillion,
		})
	}
	return out, nil
}

// UpsertPricing 写入/覆盖某模型定价（admin 改价，source=admin）。
func (d *DAO) UpsertPricing(ctx context.Context, p dbgen.UpsertModelPricingParams) error {
	return dbgen.New(d.st.Write()).UpsertModelPricing(ctx, p)
}

// DeletePricing 删除某模型定价。
func (d *DAO) DeletePricing(ctx context.Context, model string) error {
	return dbgen.New(d.st.Write()).DeleteModelPricing(ctx, model)
}

// toLogParams 把 usage.Event 映射为事实行插入参数（事实表存原始 token；first_token_ms 为 0 时存 NULL）。
func toLogParams(ev *usage.Event) dbgen.InsertRequestLogParams {
	return dbgen.InsertRequestLogParams{
		RequestID:           ev.RequestID,
		CreatedAt:           ev.CreatedAt.UTC().Format(time.RFC3339Nano),
		ApiKeyID:            ev.APIKeyID,
		ChannelID:           ev.ChannelID,
		UserID:              0,
		GroupName:           orDefault(ev.Group, "default"),
		RequestedModel:      ev.RequestedModel,
		UpstreamModel:       ev.UpstreamModel,
		EndpointFormat:      ev.EndpointFormat,
		IsStream:            b2i(ev.IsStream),
		InputTokens:         int64(ev.InputTokens),
		OutputTokens:        int64(ev.OutputTokens),
		ReasoningTokens:     int64(ev.ReasoningTokens),
		CacheReadTokens:     int64(ev.CacheReadTokens),
		CacheCreationTokens: int64(ev.CacheCreationTokens),
		TotalTokens:         int64(ev.TotalTokens),
		LatencyMs:           ev.LatencyMS,
		FirstTokenMs:        nullInt64(ev.FirstTokenMS),
		StatusCode:          int64(ev.StatusCode),
		IsError:             b2i(ev.IsError),
		ErrorType:           NormalizeErrorType(ev.StatusCode, ev.IsError, ev.ErrorType),
		ErrorMessage:        "", // event 不携带错误体；列预留（截断），M3 暂空
		SessionID:           ev.SessionID,
		UsageSource:         ev.UsageSource,
	}
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func nullInt64(v int64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nilIfZero(v int64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
