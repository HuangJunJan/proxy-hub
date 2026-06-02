package monitor

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"proxy-hub/internal/store"
)

type Service struct {
	logRepo   store.RequestLogRepo
	statsRepo store.StatsRepo
	logger    *slog.Logger

	entries  chan store.LogEntry
	dropped atomic.Uint64
}

type Options struct {
	BufferSize    int
	BatchSize     int
	FlushInterval time.Duration
}

func NewService(logRepo store.RequestLogRepo, statsRepo store.StatsRepo, logger *slog.Logger, opts Options) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if opts.BufferSize <= 0 {
		opts.BufferSize = 1024
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 50
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = 100 * time.Millisecond
	}
	return &Service{
		logRepo:   logRepo,
		statsRepo: statsRepo,
		logger:    logger,
		entries:   make(chan store.LogEntry, opts.BufferSize),
	}
}

func (s *Service) Run(ctx context.Context, opts Options) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 50
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = 100 * time.Millisecond
	}
	ticker := time.NewTicker(opts.FlushInterval)
	defer ticker.Stop()

	batch := make([]store.LogEntry, 0, opts.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.flushBatch(context.Background(), batch); err != nil {
			s.logger.Warn("failed to flush request logs", "error", err, "count", len(batch))
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			for {
				select {
				case entry := <-s.entries:
					batch = append(batch, entry)
				default:
					flush()
					return
				}
			}
		case entry := <-s.entries:
			batch = append(batch, entry)
			if len(batch) >= opts.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Service) RunCleanup(ctx context.Context, retentionDays func() int) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanup(ctx, retentionDays)
		}
	}
}

func (s *Service) CleanupOnce(ctx context.Context, retentionDays int) (int64, error) {
	if s.logRepo == nil || retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).UnixMilli()
	return s.logRepo.DeleteBefore(ctx, cutoff)
}

func (s *Service) cleanup(ctx context.Context, retentionDays func() int) {
	if retentionDays == nil {
		return
	}
	deleted, err := s.CleanupOnce(ctx, retentionDays())
	if err != nil {
		s.logger.Warn("failed to clean old request logs", "error", err)
		return
	}
	if deleted > 0 {
		s.logger.Info("cleaned old request logs", "deleted", deleted)
	}
}

func (s *Service) Submit(entry store.LogEntry) {
	if entry.TimestampMS == 0 {
		entry.TimestampMS = time.Now().UnixMilli()
	}
	if entry.Attempts == 0 {
		entry.Attempts = 1
	}
	select {
	case s.entries <- entry:
	default:
		s.dropped.Add(1)
		s.logger.Warn("request log buffer full; dropping entry", "dropped", s.dropped.Load())
	}
}

func (s *Service) Dropped() uint64 {
	return s.dropped.Load()
}

func (s *Service) flushBatch(ctx context.Context, batch []store.LogEntry) error {
	if s.logRepo != nil {
		if err := s.logRepo.BatchInsert(ctx, batch); err != nil {
			return err
		}
	}
	if s.statsRepo != nil {
		return s.statsRepo.UpsertHourly(ctx, aggregateHourly(batch))
	}
	return nil
}

func aggregateHourly(entries []store.LogEntry) []store.HourlyDelta {
	type key struct {
		channel string
		hour    int64
	}
	acc := map[key]store.HourlyDelta{}
	for _, entry := range entries {
		if entry.ChannelName == "" {
			continue
		}
		hour := entry.TimestampMS - (entry.TimestampMS % int64(time.Hour/time.Millisecond))
		k := key{channel: entry.ChannelName, hour: hour}
		delta := acc[k]
		delta.ChannelName = entry.ChannelName
		delta.HourTimestampMS = hour
		delta.Requests++
		if entry.StatusCode >= 200 && entry.StatusCode < 400 {
			delta.Successes++
		} else {
			delta.Failures++
		}
		if entry.PromptTokens != nil {
			delta.PromptTokens += *entry.PromptTokens
		}
		if entry.CompletionTokens != nil {
			delta.CompletionTokens += *entry.CompletionTokens
		}
		if entry.TotalTokens != nil {
			delta.TotalTokens += *entry.TotalTokens
		}
		delta.AvgDurationMS = ((delta.AvgDurationMS * (delta.Requests - 1)) + entry.DurationMS) / delta.Requests
		acc[k] = delta
	}
	out := make([]store.HourlyDelta, 0, len(acc))
	for _, delta := range acc {
		out = append(out, delta)
	}
	return out
}
