package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/huangjunjan/proxy-hub/internal/store"
	"github.com/huangjunjan/proxy-hub/internal/store/dbgen"
)

// DAO 包裹 dbgen，提供 MCP server 与 sync target 的领域级持久化。
type DAO struct {
	st *store.Store
}

// NewDAO 创建 MCP DAO。
func NewDAO(st *store.Store) *DAO { return &DAO{st: st} }

func (d *DAO) read() *dbgen.Queries  { return dbgen.New(d.st.Read()) }
func (d *DAO) write() *dbgen.Queries { return dbgen.New(d.st.Write()) }

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func serverFromRow(r dbgen.McpServer) Server {
	spec := map[string]any{}
	_ = json.Unmarshal([]byte(r.SpecJson), &spec)
	var tags []string
	_ = json.Unmarshal([]byte(r.TagsJson), &tags)
	return Server{
		ID: r.ID, Name: r.Name, Spec: spec, Description: r.Description, Homepage: r.Homepage,
		Docs: r.Docs, Tags: tags, EnabledCodex: r.EnabledCodex != 0, EnabledClaude: r.EnabledClaude != 0,
	}
}

func targetFromRow(r dbgen.McpSyncTarget) Target {
	t := Target{
		ID: r.ID, Client: r.Client, ConfigPath: r.ConfigPath, Label: r.Label,
		Enabled: r.Enabled != 0, LastSyncStatus: r.LastSyncStatus,
	}
	if r.LastSyncedAt.Valid {
		t.LastSyncedAt = r.LastSyncedAt.String
	}
	return t
}

// ListServers 返回全部 server（按 id）。
func (d *DAO) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := d.read().ListMcpServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出 MCP server 失败: %w", err)
	}
	out := make([]Server, 0, len(rows))
	for _, r := range rows {
		out = append(out, serverFromRow(r))
	}
	return out, nil
}

// GetServer 返回指定 server；found=false 表示不存在。
func (d *DAO) GetServer(ctx context.Context, id string) (Server, bool, error) {
	r, err := d.read().GetMcpServer(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Server{}, false, nil
	}
	if err != nil {
		return Server{}, false, fmt.Errorf("读取 MCP server 失败: %w", err)
	}
	return serverFromRow(r), true, nil
}

// UpsertServer 写入/更新 server（spec/tags 序列化为 JSON）。enabled 位由入参决定（service 负责保留）。
func (d *DAO) UpsertServer(ctx context.Context, s Server, now string) error {
	specJSON, err := json.Marshal(s.Spec)
	if err != nil {
		return fmt.Errorf("序列化 spec 失败: %w", err)
	}
	tags := s.Tags
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, _ := json.Marshal(tags)
	return d.write().UpsertMcpServer(ctx, dbgen.UpsertMcpServerParams{
		ID: s.ID, Name: s.Name, SpecJson: string(specJSON), Description: s.Description,
		Homepage: s.Homepage, Docs: s.Docs, TagsJson: string(tagsJSON),
		EnabledCodex: b2i(s.EnabledCodex), EnabledClaude: b2i(s.EnabledClaude),
		CreatedAt: now, UpdatedAt: now,
	})
}

// DeleteServer 删除 server。
func (d *DAO) DeleteServer(ctx context.Context, id string) error {
	return d.write().DeleteMcpServer(ctx, id)
}

// SetToggle 设置某 server 的两个 client 启用位。
func (d *DAO) SetToggle(ctx context.Context, id string, codex, claude bool, now string) error {
	return d.write().SetMcpServerToggle(ctx, dbgen.SetMcpServerToggleParams{
		EnabledCodex: b2i(codex), EnabledClaude: b2i(claude), UpdatedAt: now, ID: id,
	})
}

// ListTargets 返回全部 target。
func (d *DAO) ListTargets(ctx context.Context) ([]Target, error) {
	rows, err := d.read().ListMcpSyncTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出 sync target 失败: %w", err)
	}
	out := make([]Target, 0, len(rows))
	for _, r := range rows {
		out = append(out, targetFromRow(r))
	}
	return out, nil
}

// ListEnabledTargets 返回启用的 target。
func (d *DAO) ListEnabledTargets(ctx context.Context) ([]Target, error) {
	rows, err := d.read().ListEnabledMcpSyncTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出启用 sync target 失败: %w", err)
	}
	out := make([]Target, 0, len(rows))
	for _, r := range rows {
		out = append(out, targetFromRow(r))
	}
	return out, nil
}

// GetTarget 返回指定 target；found=false 表示不存在。
func (d *DAO) GetTarget(ctx context.Context, id int64) (Target, bool, error) {
	r, err := d.read().GetMcpSyncTarget(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, false, nil
	}
	if err != nil {
		return Target{}, false, fmt.Errorf("读取 sync target 失败: %w", err)
	}
	return targetFromRow(r), true, nil
}

// CreateTarget 新建 target，返回含 id 的行。
func (d *DAO) CreateTarget(ctx context.Context, t Target, now string) (Target, error) {
	r, err := d.write().CreateMcpSyncTarget(ctx, dbgen.CreateMcpSyncTargetParams{
		Client: t.Client, ConfigPath: t.ConfigPath, Label: t.Label, Enabled: b2i(t.Enabled),
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return Target{}, fmt.Errorf("新建 sync target 失败: %w", err)
	}
	return targetFromRow(r), nil
}

// UpdateTarget 更新 target。
func (d *DAO) UpdateTarget(ctx context.Context, t Target, now string) error {
	return d.write().UpdateMcpSyncTarget(ctx, dbgen.UpdateMcpSyncTargetParams{
		Client: t.Client, ConfigPath: t.ConfigPath, Label: t.Label, Enabled: b2i(t.Enabled),
		UpdatedAt: now, ID: t.ID,
	})
}

// DeleteTarget 删除 target。
func (d *DAO) DeleteTarget(ctx context.Context, id int64) error {
	return d.write().DeleteMcpSyncTarget(ctx, id)
}

// SetTargetStatus 回写 target 的同步状态与时间。
func (d *DAO) SetTargetStatus(ctx context.Context, id int64, syncedAt, status, now string) error {
	return d.write().SetMcpSyncTargetStatus(ctx, dbgen.SetMcpSyncTargetStatusParams{
		LastSyncedAt:   sql.NullString{String: syncedAt, Valid: syncedAt != ""},
		LastSyncStatus: status, UpdatedAt: now, ID: id,
	})
}
