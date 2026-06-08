package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/fileio"
)

// Service 编排 MCP SSOT 与向各 sync target 的投影（位图 diff → 写入/移除）。
type Service struct {
	dao  *DAO
	now  func() time.Time
	goos string // 便于测试；默认 runtime.GOOS
}

// NewService 创建 MCP 服务。
func NewService(dao *DAO) *Service {
	return &Service{dao: dao, now: time.Now, goos: runtime.GOOS}
}

func (s *Service) nowStr() string { return s.now().UTC().Format(time.RFC3339) }

// ListServers / GetServer / ListTargets / GetTarget 透传。
func (s *Service) ListServers(ctx context.Context) ([]Server, error) { return s.dao.ListServers(ctx) }
func (s *Service) GetServer(ctx context.Context, id string) (Server, bool, error) {
	return s.dao.GetServer(ctx, id)
}
func (s *Service) ListTargets(ctx context.Context) ([]Target, error) { return s.dao.ListTargets(ctx) }
func (s *Service) GetTarget(ctx context.Context, id int64) (Target, bool, error) {
	return s.dao.GetTarget(ctx, id)
}

// UpsertServer 校验 spec → 保留已存 enabled 位 → 入库 → 全量重对账（spec 变更投影到各目标）。
func (s *Service) UpsertServer(ctx context.Context, srv Server) error {
	if strings.TrimSpace(srv.ID) == "" {
		return errors.New("server id 不能为空")
	}
	if err := Validate(srv.Spec); err != nil {
		return err
	}
	if existing, found, err := s.dao.GetServer(ctx, srv.ID); err != nil {
		return err
	} else if found {
		srv.EnabledCodex = existing.EnabledCodex
		srv.EnabledClaude = existing.EnabledClaude
	}
	if err := s.dao.UpsertServer(ctx, srv, s.nowStr()); err != nil {
		return err
	}
	return s.SyncAll(ctx)
}

// ToggleClient 翻某 server 的某 client 启用位 → 写库 → 对该 client 目标重对账（启用→写入 / 禁用→移除）。
func (s *Service) ToggleClient(ctx context.Context, id, client string, on bool) error {
	srv, found, err := s.dao.GetServer(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("server 不存在: %s", id)
	}
	switch client {
	case ClientCodex:
		srv.EnabledCodex = on
	case ClientClaude:
		srv.EnabledClaude = on
	default:
		return fmt.Errorf("未知 client: %s", client)
	}
	if err := s.dao.SetToggle(ctx, id, srv.EnabledCodex, srv.EnabledClaude, s.nowStr()); err != nil {
		return err
	}
	return s.syncClient(ctx, client)
}

// DeleteServer 删除 server，并把它以「禁用」身份并入对账集，使各目标文件移除其条目。
func (s *Service) DeleteServer(ctx context.Context, id string) error {
	srv, found, _ := s.dao.GetServer(ctx, id)
	if err := s.dao.DeleteServer(ctx, id); err != nil {
		return err
	}
	if found {
		srv.EnabledCodex, srv.EnabledClaude = false, false
		return s.syncWithExtra(ctx, []Server{srv})
	}
	return s.SyncAll(ctx)
}

// CreateTarget 新建目标并立即对账它。
func (s *Service) CreateTarget(ctx context.Context, t Target) (Target, error) {
	created, err := s.dao.CreateTarget(ctx, t, s.nowStr())
	if err != nil {
		return Target{}, err
	}
	if created.Enabled {
		_ = s.SyncTarget(ctx, created.ID)
	}
	return created, nil
}

// UpdateTarget 更新目标。
func (s *Service) UpdateTarget(ctx context.Context, t Target) error {
	return s.dao.UpdateTarget(ctx, t, s.nowStr())
}

// DeleteTarget 删除目标登记（不改动其文件内容）。
func (s *Service) DeleteTarget(ctx context.Context, id int64) error {
	return s.dao.DeleteTarget(ctx, id)
}

// SyncAll 对所有启用目标做全量对账。
func (s *Service) SyncAll(ctx context.Context) error {
	all, err := s.dao.ListServers(ctx)
	if err != nil {
		return err
	}
	targets, err := s.dao.ListEnabledTargets(ctx)
	if err != nil {
		return err
	}
	return s.syncTargets(ctx, targets, all)
}

// SyncTarget 对单个目标对账。
func (s *Service) SyncTarget(ctx context.Context, targetID int64) error {
	t, found, err := s.dao.GetTarget(ctx, targetID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("sync target 不存在: %d", targetID)
	}
	all, err := s.dao.ListServers(ctx)
	if err != nil {
		return err
	}
	return s.applyToTarget(ctx, t, all)
}

func (s *Service) syncWithExtra(ctx context.Context, extra []Server) error {
	all, err := s.dao.ListServers(ctx)
	if err != nil {
		return err
	}
	all = append(all, extra...)
	targets, err := s.dao.ListEnabledTargets(ctx)
	if err != nil {
		return err
	}
	return s.syncTargets(ctx, targets, all)
}

func (s *Service) syncClient(ctx context.Context, client string) error {
	all, err := s.dao.ListServers(ctx)
	if err != nil {
		return err
	}
	targets, err := s.dao.ListEnabledTargets(ctx)
	if err != nil {
		return err
	}
	filtered := make([]Target, 0, len(targets))
	for _, t := range targets {
		if t.Client == client {
			filtered = append(filtered, t)
		}
	}
	return s.syncTargets(ctx, filtered, all)
}

func (s *Service) syncTargets(ctx context.Context, targets []Target, all []Server) error {
	var firstErr error
	for _, t := range targets {
		if err := s.applyToTarget(ctx, t, all); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// applyToTarget 把对账集投影进单个目标文件，并回写 last_sync_status（父目录缺失记为 skipped，非硬错）。
func (s *Service) applyToTarget(ctx context.Context, t Target, all []Server) error {
	var err error
	switch t.Client {
	case ClientClaude:
		wrap := s.goos == "windows" && !isWSLPath(t.ConfigPath)
		err = ApplyClaude(t.ConfigPath, all, wrap)
	case ClientCodex:
		err = ApplyCodex(t.ConfigPath, all)
	default:
		err = fmt.Errorf("未知 client: %s", t.Client)
	}
	status := "ok"
	if errors.Is(err, fileio.ErrParentMissing) {
		status = "skipped: 父目录不存在"
		err = nil
	} else if err != nil {
		status = "error: " + truncate(err.Error(), 200)
	}
	_ = s.dao.SetTargetStatus(ctx, t.ID, s.nowStr(), status, s.nowStr())
	return err
}

// Import 把目标现有配置读入注册表（只读）：id 冲突 ⇒ 翻该 client 开关位，绝不覆盖已存 spec（spec 不同则告警）。
func (s *Service) Import(ctx context.Context, targetID int64) error {
	t, found, err := s.dao.GetTarget(ctx, targetID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("sync target 不存在: %d", targetID)
	}
	var servers []Server
	switch t.Client {
	case ClientClaude:
		servers, err = ReadClaude(t.ConfigPath)
	case ClientCodex:
		servers, err = ReadCodex(t.ConfigPath)
	default:
		return fmt.Errorf("未知 client: %s", t.Client)
	}
	if err != nil {
		return err
	}
	now := s.nowStr()
	for _, imp := range servers {
		existing, found, gerr := s.dao.GetServer(ctx, imp.ID)
		if gerr != nil {
			return gerr
		}
		if found {
			if !specEqual(existing.Spec, imp.Spec) {
				slog.Warn("MCP 导入 id 冲突且 spec 不同：保留已存 spec，仅翻开关位", "id", imp.ID, "client", t.Client)
			}
			if t.Client == ClientCodex {
				existing.EnabledCodex = true
			} else {
				existing.EnabledClaude = true
			}
			if err := s.dao.SetToggle(ctx, existing.ID, existing.EnabledCodex, existing.EnabledClaude, now); err != nil {
				return err
			}
		} else {
			if err := Validate(imp.Spec); err != nil {
				slog.Warn("MCP 导入跳过非法 spec", "id", imp.ID, "error", err)
				continue
			}
			if err := s.dao.UpsertServer(ctx, imp, now); err != nil {
				return err
			}
		}
	}
	return nil
}

// ImportBundle 朴素导入 {mcpServers} bundle + 启用的 apps（codex/claude CSV），保留已存启用位（OR）。
func (s *Service) ImportBundle(ctx context.Context, bundle map[string]map[string]any, apps []string) error {
	enCodex := containsStr(apps, ClientCodex)
	enClaude := containsStr(apps, ClientClaude)
	now := s.nowStr()
	for id, spec := range bundle {
		if err := Validate(spec); err != nil {
			slog.Warn("MCP bundle 跳过非法 spec", "id", id, "error", err)
			continue
		}
		srv := Server{ID: id, Spec: spec, EnabledCodex: enCodex, EnabledClaude: enClaude}
		if ex, found, _ := s.dao.GetServer(ctx, id); found {
			srv.EnabledCodex = ex.EnabledCodex || enCodex
			srv.EnabledClaude = ex.EnabledClaude || enClaude
		}
		if err := s.dao.UpsertServer(ctx, srv, now); err != nil {
			return err
		}
	}
	return s.SyncAll(ctx)
}

// ---- helpers ----

func isWSLPath(p string) bool {
	lp := strings.ToLower(p)
	return strings.Contains(lp, `\\wsl$`) || strings.Contains(lp, `\\wsl.localhost`) ||
		strings.HasPrefix(lp, "/mnt/") || strings.HasPrefix(lp, `//wsl`)
}

func specEqual(a, b map[string]any) bool {
	ba, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ba) == string(bb)
}

func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
