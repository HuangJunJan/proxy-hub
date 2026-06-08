// dao.go：渠道/ability/入站 key 的持久化层，包裹 sqlc 生成的 dbgen 查询，做 dbgen 行 <-> 领域类型转换。
//
// 读走 store.Read()（连接池），写/事务走 store.Write()（已串行化为单连接）。中文设计见任务 design.md §2。
// 注意 sqlc 默认 initialisms 仅 ["id"]，故生成字段为 BaseUrl/ProxyUrl（非 BaseURL），bool 列为 int64。
package channel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/apikey"
	"github.com/huangjunjan/proxy-hub/internal/store"
	"github.com/huangjunjan/proxy-hub/internal/store/dbgen"
)

// timeFormat 是所有时间列的存储格式。
const timeFormat = time.RFC3339

// DAO 封装 dbgen 查询。
type DAO struct {
	st *store.Store
}

// NewDAO 创建 DAO。
func NewDAO(st *store.Store) *DAO { return &DAO{st: st} }

// reader 返回基于读池的查询对象。
func (d *DAO) reader() *dbgen.Queries { return dbgen.New(d.st.Read()) }

// ---- 标量/时间转换工具 ----

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func i2b(i int64) bool { return i != 0 }

func nowStr() string { return time.Now().UTC().Format(timeFormat) }

// timeToNull 把时间转为可空文本列（零值 -> NULL）。
func timeToNull(t time.Time) sql.NullString {
	if t.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(timeFormat), Valid: true}
}

// nullToTime 解析可空文本时间列（NULL/空/非法 -> 零值）。
func nullToTime(ns sql.NullString) time.Time {
	if !ns.Valid || ns.String == "" {
		return time.Time{}
	}
	t, err := time.Parse(timeFormat, ns.String)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ---- dbgen 行 -> 领域 ----

func channelFromRow(r dbgen.Channel) (Channel, error) {
	models, err := ParseModels(r.Models)
	if err != nil {
		return Channel{}, fmt.Errorf("渠道 %d 的 models 解析失败: %w", r.ID, err)
	}
	mapping, err := ParseModelMapping(r.ModelMapping)
	if err != nil {
		return Channel{}, fmt.Errorf("渠道 %d 的 model_mapping 解析失败: %w", r.ID, err)
	}
	return Channel{
		ID:           r.ID,
		Name:         r.Name,
		Enabled:      i2b(r.Enabled),
		Platform:     Platform(r.Platform),
		Type:         ChannelType(r.Type),
		BaseURL:      r.BaseUrl,
		Group:        r.GroupName,
		Priority:     int(r.Priority),
		Weight:       int(r.Weight),
		Models:       models,
		ModelMapping: mapping,
		Prefix:       r.Prefix,
		ProxyURL:     r.ProxyUrl,
		Status:       r.Status,
		ErrorMessage: r.ErrorMessage,
	}, nil
}

// ---- 渠道读 ----

// ListChannels 返回全部渠道（按 priority 降序、id 升序）。
func (d *DAO) ListChannels(ctx context.Context) ([]Channel, error) {
	rows, err := d.reader().ListChannels(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出渠道失败: %w", err)
	}
	out := make([]Channel, 0, len(rows))
	for _, r := range rows {
		c, err := channelFromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// GetChannel 返回单个渠道；不存在时 found=false。
func (d *DAO) GetChannel(ctx context.Context, id int64) (Channel, bool, error) {
	r, err := d.reader().GetChannel(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, false, nil
	}
	if err != nil {
		return Channel{}, false, fmt.Errorf("查询渠道 %d 失败: %w", id, err)
	}
	c, err := channelFromRow(r)
	if err != nil {
		return Channel{}, false, err
	}
	return c, true, nil
}

// ListEnabledChannelAbilities 返回所有启用渠道及其展开的 abilities（供 RouteIndex.Rebuild）。
// abilities 由 BuildAbilities 从渠道现态重导出，与保存时写入 abilities 表的内容一致。
func (d *DAO) ListEnabledChannelAbilities(ctx context.Context) ([]ChannelAbilities, error) {
	rows, err := d.reader().ListEnabledChannels(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出启用渠道失败: %w", err)
	}
	out := make([]ChannelAbilities, 0, len(rows))
	for _, r := range rows {
		c, err := channelFromRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, ChannelAbilities{Channel: c, Abilities: BuildAbilities(c)})
	}
	return out, nil
}

// ---- 渠道写（事务：渠道行 + abilities 同事务重建）----

// SaveChannelTx 在单事务内创建/更新渠道并重建其 abilities。c.ID==0 视为创建，否则更新。
// 返回带 ID 的已保存渠道（领域形态）。不触碰凭证与 RouteIndex（由 Manager 在事务提交后处理）。
func (d *DAO) SaveChannelTx(ctx context.Context, c Channel) (Channel, error) {
	models, err := EncodeModels(c.Models)
	if err != nil {
		return Channel{}, err
	}
	mapping, err := EncodeModelMapping(c.ModelMapping)
	if err != nil {
		return Channel{}, err
	}
	status := c.Status
	if status == "" {
		status = "active"
	}
	now := nowStr()

	tx, err := d.st.Write().BeginTx(ctx, nil)
	if err != nil {
		return Channel{}, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 提交后为 no-op

	q := dbgen.New(tx)

	var saved dbgen.Channel
	if c.ID == 0 {
		saved, err = q.CreateChannel(ctx, dbgen.CreateChannelParams{
			Name: c.Name, Enabled: b2i(c.Enabled), Platform: string(c.Platform), Type: string(c.Type),
			BaseUrl: c.BaseURL, GroupName: c.Group, Priority: int64(c.Priority), Weight: int64(c.Weight),
			Models: models, ModelMapping: mapping, Prefix: c.Prefix, ProxyUrl: c.ProxyURL,
			Status: status, ErrorMessage: c.ErrorMessage, CreatedAt: now, UpdatedAt: now,
		})
	} else {
		saved, err = q.UpdateChannel(ctx, dbgen.UpdateChannelParams{
			Name: c.Name, Enabled: b2i(c.Enabled), Platform: string(c.Platform), Type: string(c.Type),
			BaseUrl: c.BaseURL, GroupName: c.Group, Priority: int64(c.Priority), Weight: int64(c.Weight),
			Models: models, ModelMapping: mapping, Prefix: c.Prefix, ProxyUrl: c.ProxyURL,
			Status: status, ErrorMessage: c.ErrorMessage, UpdatedAt: now, ID: c.ID,
		})
	}
	if err != nil {
		return Channel{}, fmt.Errorf("保存渠道失败: %w", err)
	}

	domain, err := channelFromRow(saved)
	if err != nil {
		return Channel{}, err
	}

	// 重建该渠道 abilities：先删后插（增量，绝不 TRUNCATE 全表）。
	if err := q.DeleteAbilitiesForChannel(ctx, domain.ID); err != nil {
		return Channel{}, fmt.Errorf("清理旧 abilities 失败: %w", err)
	}
	for _, a := range BuildAbilities(domain) {
		if err := q.CreateAbility(ctx, dbgen.CreateAbilityParams{
			GroupName: a.Group, AliasModel: a.AliasModel, ChannelID: a.ChannelID,
			UpstreamModel: a.UpstreamModel, Priority: int64(a.Priority), Weight: int64(a.Weight), Enabled: b2i(a.Enabled),
		}); err != nil {
			return Channel{}, fmt.Errorf("写入 ability 失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Channel{}, fmt.Errorf("提交事务失败: %w", err)
	}
	return domain, nil
}

// DeleteChannel 删除渠道（abilities 经 FK ON DELETE CASCADE 一并删除）。
func (d *DAO) DeleteChannel(ctx context.Context, id int64) error {
	if err := dbgen.New(d.st.Write()).DeleteChannel(ctx, id); err != nil {
		return fmt.Errorf("删除渠道 %d 失败: %w", id, err)
	}
	return nil
}

// SetChannelStatus 更新渠道状态与错误信息（渠道测试结果回写）。
func (d *DAO) SetChannelStatus(ctx context.Context, id int64, status, errMsg string) error {
	err := dbgen.New(d.st.Write()).SetChannelStatus(ctx, dbgen.SetChannelStatusParams{
		Status: status, ErrorMessage: errMsg, UpdatedAt: nowStr(), ID: id,
	})
	if err != nil {
		return fmt.Errorf("更新渠道 %d 状态失败: %w", id, err)
	}
	return nil
}

// ---- 入站 key ----

// CreateAPIKey 写入一条入站 key（仅哈希入库），返回新行 id。
func (d *DAO) CreateAPIKey(ctx context.Context, hash, name, group string) (int64, error) {
	if group == "" {
		group = "default"
	}
	row, err := dbgen.New(d.st.Write()).CreateAPIKey(ctx, dbgen.CreateAPIKeyParams{
		Hash: hash, Name: name, GroupName: group, Enabled: 1, CreatedAt: nowStr(),
	})
	if err != nil {
		return 0, fmt.Errorf("创建入站 key 失败: %w", err)
	}
	return row.ID, nil
}

// GetAPIKeyByHash 供鉴权缓存回源：按哈希查 key 元数据。
func (d *DAO) GetAPIKeyByHash(ctx context.Context, hash string) (apikey.KeyMeta, bool, error) {
	r, err := d.reader().GetAPIKeyByHash(ctx, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return apikey.KeyMeta{}, false, nil
	}
	if err != nil {
		return apikey.KeyMeta{}, false, fmt.Errorf("按哈希查 key 失败: %w", err)
	}
	return apikey.KeyMeta{ID: r.ID, Group: r.GroupName, Enabled: i2b(r.Enabled)}, true, nil
}

// ListAPIKeys 返回全部入站 key 的可展示信息（不含哈希）。
func (d *DAO) ListAPIKeys(ctx context.Context) ([]apikey.KeyInfo, error) {
	rows, err := d.reader().ListAPIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出入站 key 失败: %w", err)
	}
	out := make([]apikey.KeyInfo, 0, len(rows))
	for _, r := range rows {
		var lastUsed string
		if r.LastUsedAt.Valid {
			lastUsed = r.LastUsedAt.String
		}
		out = append(out, apikey.KeyInfo{
			ID: r.ID, Name: r.Name, Group: r.GroupName, Enabled: i2b(r.Enabled),
			CreatedAt: r.CreatedAt, LastUsedAt: lastUsed,
		})
	}
	return out, nil
}

// SetAPIKeyEnabled 启用/禁用一条入站 key。
func (d *DAO) SetAPIKeyEnabled(ctx context.Context, id int64, enabled bool) error {
	err := dbgen.New(d.st.Write()).SetAPIKeyEnabled(ctx, dbgen.SetAPIKeyEnabledParams{Enabled: b2i(enabled), ID: id})
	if err != nil {
		return fmt.Errorf("更新 key %d 启用态失败: %w", id, err)
	}
	return nil
}

// DeleteAPIKey 删除一条入站 key。
func (d *DAO) DeleteAPIKey(ctx context.Context, id int64) error {
	if err := dbgen.New(d.st.Write()).DeleteAPIKey(ctx, id); err != nil {
		return fmt.Errorf("删除 key %d 失败: %w", id, err)
	}
	return nil
}

// ---- 健康（被 relay.HealthStore 经领域快照桥接；此处提供原始读写）----

// HealthSnapshot 是 channel_model_health 的领域快照（time.Time 形态，避免上层处理 NULL/格式）。
type HealthSnapshot struct {
	ChannelID           int64
	Model               string
	IsHealthy           bool
	ConsecutiveFailures int
	LastSuccessAt       time.Time
	LastFailureAt       time.Time
	LastError           string
	CooldownUntil       time.Time
	UpdatedAt           time.Time
}

// ListHealth 读取全部健康快照（启动时装配内存镜像）。
func (d *DAO) ListHealth(ctx context.Context) ([]HealthSnapshot, error) {
	rows, err := d.reader().ListChannelModelHealth(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出渠道健康失败: %w", err)
	}
	out := make([]HealthSnapshot, 0, len(rows))
	for _, r := range rows {
		out = append(out, HealthSnapshot{
			ChannelID: r.ChannelID, Model: r.Model, IsHealthy: i2b(r.IsHealthy),
			ConsecutiveFailures: int(r.ConsecutiveFailures),
			LastSuccessAt:       nullToTime(r.LastSuccessAt),
			LastFailureAt:       nullToTime(r.LastFailureAt),
			LastError:           r.LastError,
			CooldownUntil:       nullToTime(r.CooldownUntil),
			UpdatedAt:           nullToTime(sql.NullString{String: r.UpdatedAt, Valid: true}),
		})
	}
	return out, nil
}

// UpsertHealth 写入一条健康快照（MarkResult 同步落库）。
func (d *DAO) UpsertHealth(ctx context.Context, s HealthSnapshot) error {
	err := dbgen.New(d.st.Write()).UpsertChannelModelHealth(ctx, dbgen.UpsertChannelModelHealthParams{
		ChannelID: s.ChannelID, Model: s.Model, IsHealthy: b2i(s.IsHealthy),
		ConsecutiveFailures: int64(s.ConsecutiveFailures),
		LastSuccessAt:       timeToNull(s.LastSuccessAt),
		LastFailureAt:       timeToNull(s.LastFailureAt),
		LastError:           s.LastError,
		CooldownUntil:       timeToNull(s.CooldownUntil),
		UpdatedAt:           s.UpdatedAt.UTC().Format(timeFormat),
	})
	if err != nil {
		return fmt.Errorf("写入渠道健康失败: %w", err)
	}
	return nil
}
