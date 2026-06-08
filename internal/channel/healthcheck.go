package channel

import (
	"context"

	"github.com/huangjunjan/proxy-hub/internal/store/dbgen"
)

// HealthCheck 是一条主动健康探测记录（领域表示）。
type HealthCheck struct {
	ID             int64
	ChannelID      int64
	Model          string
	Success        bool
	HTTPStatus     int
	ResponseTimeMS int64
	Message        string
	CheckedAt      string
}

// InsertHealthCheck 写入一条主动探测记录。
func (d *DAO) InsertHealthCheck(ctx context.Context, hc HealthCheck) error {
	return dbgen.New(d.st.Write()).InsertHealthCheck(ctx, dbgen.InsertHealthCheckParams{
		ChannelID:      hc.ChannelID,
		Model:          hc.Model,
		Success:        b2i(hc.Success),
		HttpStatus:     int64(hc.HTTPStatus),
		ResponseTimeMs: hc.ResponseTimeMS,
		Message:        hc.Message,
		CheckedAt:      hc.CheckedAt,
	})
}

// ListRecentHealthChecks 返回最近 limit 条探测记录（倒序；limit<=0 取 100）。
func (d *DAO) ListRecentHealthChecks(ctx context.Context, limit int) ([]HealthCheck, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.reader().ListRecentHealthChecks(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	out := make([]HealthCheck, 0, len(rows))
	for _, r := range rows {
		out = append(out, HealthCheck{
			ID: r.ID, ChannelID: r.ChannelID, Model: r.Model, Success: r.Success != 0,
			HTTPStatus: int(r.HttpStatus), ResponseTimeMS: r.ResponseTimeMs,
			Message: r.Message, CheckedAt: r.CheckedAt,
		})
	}
	return out, nil
}
