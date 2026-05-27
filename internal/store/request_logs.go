package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s *SQLiteStore) BatchInsert(ctx context.Context, entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin request log batch: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO request_logs (
			ts, api_key_token_mask, api_key_name, endpoint, request_type,
			channel_name, channel_type,
			downstream_model, upstream_model, upstream_key_index, status_code,
			is_stream, duration_ms, first_token_ms, reasoning_effort, billing_mode,
			prompt_tokens, completion_tokens, reasoning_tokens, total_tokens, error_kind, error_message,
			request_body, response_body, attempts, user_agent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare request log insert: %w", err)
	}
	defer stmt.Close()

	for _, entry := range entries {
		attempts := entry.Attempts
		if attempts == 0 {
			attempts = 1
		}
		if _, err := stmt.ExecContext(ctx,
			entry.TimestampMS,
			entry.APIKeyTokenMask,
			nullString(entry.APIKeyName),
			nullString(entry.Endpoint),
			nullString(entry.RequestType),
			nullString(entry.ChannelName),
			nullString(entry.ChannelType),
			entry.DownstreamModel,
			nullString(entry.UpstreamModel),
			nullInt(entry.UpstreamKeyIndex),
			entry.StatusCode,
			boolInt(entry.IsStream),
			entry.DurationMS,
			nullInt64(entry.FirstTokenMS),
			nullString(entry.ReasoningEffort),
			nullString(entry.BillingMode),
			nullInt64(entry.PromptTokens),
			nullInt64(entry.CompletionTokens),
			nullInt64(entry.ReasoningTokens),
			nullInt64(entry.TotalTokens),
			nullString(entry.ErrorKind),
			nullString(entry.ErrorMessage),
			nullBytes(entry.RequestBody),
			nullBytes(entry.ResponseBody),
			attempts,
			nullString(entry.UserAgent),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert request log: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit request log batch: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Query(ctx context.Context, filter QueryFilter) ([]LogEntry, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	query := strings.Builder{}
	query.WriteString(`
		SELECT id, ts, api_key_token_mask, api_key_name, endpoint, request_type,
			channel_name, channel_type,
			downstream_model, upstream_model, upstream_key_index, status_code,
			is_stream, duration_ms, first_token_ms, reasoning_effort, billing_mode,
			prompt_tokens, completion_tokens, reasoning_tokens, total_tokens, error_kind, error_message,
			request_body, response_body, attempts, user_agent
		FROM request_logs
		WHERE 1 = 1
	`)
	args := []any{}
	if filter.ChannelName != "" {
		query.WriteString(" AND channel_name = ?")
		args = append(args, filter.ChannelName)
	}
	if filter.APIKey != "" {
		query.WriteString(" AND (api_key_name LIKE ? OR api_key_token_mask LIKE ?)")
		value := "%" + filter.APIKey + "%"
		args = append(args, value, value)
	}
	if filter.Model != "" {
		query.WriteString(" AND (downstream_model LIKE ? OR upstream_model LIKE ?)")
		value := "%" + filter.Model + "%"
		args = append(args, value, value)
	}
	if filter.Endpoint != "" {
		query.WriteString(" AND endpoint LIKE ?")
		args = append(args, "%"+filter.Endpoint+"%")
	}
	if filter.RequestType != "" {
		query.WriteString(" AND request_type = ?")
		args = append(args, filter.RequestType)
	}
	if filter.ErrorKind != "" {
		query.WriteString(" AND error_kind LIKE ?")
		args = append(args, "%"+filter.ErrorKind+"%")
	}
	if filter.StatusCode != 0 {
		query.WriteString(" AND status_code = ?")
		args = append(args, filter.StatusCode)
	}
	switch filter.StatusClass {
	case "success":
		query.WriteString(" AND status_code >= 200 AND status_code < 400")
	case "error":
		query.WriteString(" AND status_code >= 400")
	}
	if filter.StartMS != 0 {
		query.WriteString(" AND ts >= ?")
		args = append(args, filter.StartMS)
	}
	if filter.EndMS != 0 {
		query.WriteString(" AND ts <= ?")
		args = append(args, filter.EndMS)
	}
	query.WriteString(" ORDER BY ts DESC, id DESC LIMIT ? OFFSET ?")
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query request logs: %w", err)
	}
	defer rows.Close()

	entries := make([]LogEntry, 0)
	for rows.Next() {
		entry, err := scanLogEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request logs: %w", err)
	}
	return entries, nil
}

func (s *SQLiteStore) DeleteBefore(ctx context.Context, ts int64) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM request_logs WHERE ts < ?", ts)
	if err != nil {
		return 0, fmt.Errorf("delete request logs before %d: %w", ts, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read deleted request log count: %w", err)
	}
	return n, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLogEntry(row rowScanner) (LogEntry, error) {
	var entry LogEntry
	var apiKeyName, endpoint, requestType, channelName, channelType, upstreamModel sql.NullString
	var upstreamKeyIndex sql.NullInt64
	var isStream int
	var firstTokenMS, promptTokens, completionTokens, reasoningTokens, totalTokens sql.NullInt64
	var reasoningEffort, billingMode, errorKind, errorMessage, userAgent sql.NullString
	var requestBody, responseBody []byte
	if err := row.Scan(
		&entry.ID,
		&entry.TimestampMS,
		&entry.APIKeyTokenMask,
		&apiKeyName,
		&endpoint,
		&requestType,
		&channelName,
		&channelType,
		&entry.DownstreamModel,
		&upstreamModel,
		&upstreamKeyIndex,
		&entry.StatusCode,
		&isStream,
		&entry.DurationMS,
		&firstTokenMS,
		&reasoningEffort,
		&billingMode,
		&promptTokens,
		&completionTokens,
		&reasoningTokens,
		&totalTokens,
		&errorKind,
		&errorMessage,
		&requestBody,
		&responseBody,
		&entry.Attempts,
		&userAgent,
	); err != nil {
		return LogEntry{}, fmt.Errorf("scan request log: %w", err)
	}
	entry.APIKeyName = apiKeyName.String
	entry.Endpoint = endpoint.String
	entry.RequestType = requestType.String
	entry.ChannelName = channelName.String
	entry.ChannelType = channelType.String
	entry.UpstreamModel = upstreamModel.String
	if upstreamKeyIndex.Valid {
		value := int(upstreamKeyIndex.Int64)
		entry.UpstreamKeyIndex = &value
	}
	entry.IsStream = isStream != 0
	if firstTokenMS.Valid {
		value := firstTokenMS.Int64
		entry.FirstTokenMS = &value
	}
	entry.ReasoningEffort = reasoningEffort.String
	entry.BillingMode = billingMode.String
	if promptTokens.Valid {
		value := promptTokens.Int64
		entry.PromptTokens = &value
	}
	if completionTokens.Valid {
		value := completionTokens.Int64
		entry.CompletionTokens = &value
	}
	if reasoningTokens.Valid {
		value := reasoningTokens.Int64
		entry.ReasoningTokens = &value
	}
	if totalTokens.Valid {
		value := totalTokens.Int64
		entry.TotalTokens = &value
	}
	entry.ErrorKind = errorKind.String
	entry.ErrorMessage = errorMessage.String
	entry.RequestBody = requestBody
	entry.ResponseBody = responseBody
	entry.UserAgent = userAgent.String
	return entry, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullString(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func nullBytes(v []byte) any {
	if len(v) == 0 {
		return nil
	}
	return v
}
