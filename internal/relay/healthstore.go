package relay

import (
	"context"

	"github.com/huangjunjan/proxy-hub/internal/channel"
)

// HealthStore 把 HealthMirror 的内存健康状态桥接到持久层（经 channel.DAO，dbgen 细节不外泄到 relay）。
// 启动时 Load 装配镜像；MarkResult 经 Persist 同步落库。
type HealthStore struct {
	dao *channel.DAO
}

// NewHealthStore 创建健康持久化桥。
func NewHealthStore(dao *channel.DAO) *HealthStore {
	return &HealthStore{dao: dao}
}

// Load 从 DB 读全部健康快照并转为 relay.HealthState（供 HealthMirror.Load）。
func (h *HealthStore) Load(ctx context.Context) ([]HealthState, error) {
	snaps, err := h.dao.ListHealth(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]HealthState, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, HealthState{
			ChannelID:           s.ChannelID,
			Model:               s.Model,
			IsHealthy:           s.IsHealthy,
			ConsecutiveFailures: s.ConsecutiveFailures,
			LastSuccessAt:       s.LastSuccessAt,
			LastFailureAt:       s.LastFailureAt,
			LastError:           s.LastError,
			CooldownUntil:       s.CooldownUntil,
			UpdatedAt:           s.UpdatedAt,
		})
	}
	return out, nil
}

// Persist 落库单条健康状态（作为 HealthMirror 的 persist 注入函数）。
func (h *HealthStore) Persist(s HealthState) error {
	return h.dao.UpsertHealth(context.Background(), channel.HealthSnapshot{
		ChannelID:           s.ChannelID,
		Model:               s.Model,
		IsHealthy:           s.IsHealthy,
		ConsecutiveFailures: s.ConsecutiveFailures,
		LastSuccessAt:       s.LastSuccessAt,
		LastFailureAt:       s.LastFailureAt,
		LastError:           s.LastError,
		CooldownUntil:       s.CooldownUntil,
		UpdatedAt:           s.UpdatedAt,
	})
}
