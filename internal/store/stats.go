package store

import (
	"context"
	"fmt"
)

func (s *SQLiteStore) UpsertHourly(ctx context.Context, deltas []HourlyDelta) error {
	if len(deltas) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin stats upsert: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO channel_stats_hourly (
			channel_name, hour_ts, requests, successes, failures,
			prompt_tokens, completion_tokens, total_tokens, avg_duration_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(channel_name, hour_ts) DO UPDATE SET
			avg_duration_ms = CASE
				WHEN channel_stats_hourly.requests + excluded.requests = 0 THEN 0
				ELSE (
					(channel_stats_hourly.avg_duration_ms * channel_stats_hourly.requests) +
					(excluded.avg_duration_ms * excluded.requests)
				) / (channel_stats_hourly.requests + excluded.requests)
			END,
			requests = channel_stats_hourly.requests + excluded.requests,
			successes = channel_stats_hourly.successes + excluded.successes,
			failures = channel_stats_hourly.failures + excluded.failures,
			prompt_tokens = channel_stats_hourly.prompt_tokens + excluded.prompt_tokens,
			completion_tokens = channel_stats_hourly.completion_tokens + excluded.completion_tokens,
			total_tokens = channel_stats_hourly.total_tokens + excluded.total_tokens
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare stats upsert: %w", err)
	}
	defer stmt.Close()

	for _, delta := range deltas {
		if delta.ChannelName == "" || delta.Requests == 0 {
			continue
		}
		if _, err := stmt.ExecContext(ctx,
			delta.ChannelName,
			delta.HourTimestampMS,
			delta.Requests,
			delta.Successes,
			delta.Failures,
			delta.PromptTokens,
			delta.CompletionTokens,
			delta.TotalTokens,
			delta.AvgDurationMS,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("upsert hourly stats: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit stats upsert: %w", err)
	}
	return nil
}

func (s *SQLiteStore) QueryChannelSummary(ctx context.Context, window TimeWindow) ([]ChannelSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT channel_name,
			SUM(requests) AS requests,
			SUM(successes) AS successes,
			SUM(failures) AS failures,
			SUM(prompt_tokens) AS prompt_tokens,
			SUM(completion_tokens) AS completion_tokens,
			SUM(total_tokens) AS total_tokens,
			CASE
				WHEN SUM(requests) = 0 THEN 0
				ELSE SUM(avg_duration_ms * requests) / SUM(requests)
			END AS avg_duration_ms
		FROM channel_stats_hourly
		WHERE (? = 0 OR hour_ts >= ?) AND (? = 0 OR hour_ts <= ?)
		GROUP BY channel_name
		ORDER BY channel_name ASC
	`, window.StartMS, window.StartMS, window.EndMS, window.EndMS)
	if err != nil {
		return nil, fmt.Errorf("query channel summary: %w", err)
	}
	defer rows.Close()

	summaries := make([]ChannelSummary, 0)
	for rows.Next() {
		var summary ChannelSummary
		if err := rows.Scan(
			&summary.ChannelName,
			&summary.Requests,
			&summary.Successes,
			&summary.Failures,
			&summary.PromptTokens,
			&summary.CompletionTokens,
			&summary.TotalTokens,
			&summary.AvgDurationMS,
		); err != nil {
			return nil, fmt.Errorf("scan channel summary: %w", err)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channel summary: %w", err)
	}
	return summaries, nil
}

func (s *SQLiteStore) QuerySeries(ctx context.Context, channelName string, metric Metric, window TimeWindow) ([]Point, error) {
	column, err := metricColumn(metric)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT hour_ts, %s
		FROM channel_stats_hourly
		WHERE channel_name = ?
			AND (? = 0 OR hour_ts >= ?)
			AND (? = 0 OR hour_ts <= ?)
		ORDER BY hour_ts ASC
	`, column)
	rows, err := s.db.QueryContext(ctx, query, channelName, window.StartMS, window.StartMS, window.EndMS, window.EndMS)
	if err != nil {
		return nil, fmt.Errorf("query stats series: %w", err)
	}
	defer rows.Close()

	points := make([]Point, 0)
	for rows.Next() {
		var point Point
		if err := rows.Scan(&point.TimestampMS, &point.Value); err != nil {
			return nil, fmt.Errorf("scan stats point: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stats series: %w", err)
	}
	return points, nil
}

func metricColumn(metric Metric) (string, error) {
	switch metric {
	case MetricRequests:
		return "requests", nil
	case MetricSuccesses:
		return "successes", nil
	case MetricFailures:
		return "failures", nil
	case MetricPromptTokens:
		return "prompt_tokens", nil
	case MetricCompletionTokens:
		return "completion_tokens", nil
	case MetricTotalTokens:
		return "total_tokens", nil
	case MetricAvgDurationMS:
		return "avg_duration_ms", nil
	default:
		return "", fmt.Errorf("unsupported stats metric %q", metric)
	}
}
