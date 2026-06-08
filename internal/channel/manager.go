// manager.go：渠道与入站 key 的应用编排层。装配 dao + credstore + 内存 RouteIndex：
// 渠道保存 = 事务写库（含 abilities 重建）→ 写凭证文件 → 增量更新 RouteIndex；删除反之。
// key 写库后由调用方失效鉴权缓存。启动时 LoadRouteIndex 从 DB 装配内存索引。
package channel

import (
	"context"
	"fmt"
	"strings"

	"github.com/huangjunjan/proxy-hub/internal/apikey"
	"github.com/huangjunjan/proxy-hub/internal/credstore"
)

// Manager 是渠道/凭证/路由的编排器。
type Manager struct {
	dao   *DAO
	creds *credstore.Store
	index *RouteIndex
}

// NewManager 创建编排器。
func NewManager(dao *DAO, creds *credstore.Store, index *RouteIndex) *Manager {
	return &Manager{dao: dao, creds: creds, index: index}
}

// LoadRouteIndex 启动时从 DB（启用渠道）装配内存 RouteIndex。
func (m *Manager) LoadRouteIndex(ctx context.Context) error {
	items, err := m.dao.ListEnabledChannelAbilities(ctx)
	if err != nil {
		return err
	}
	m.index.Rebuild(items)
	return nil
}

// ListChannels 返回全部渠道。
func (m *Manager) ListChannels(ctx context.Context) ([]Channel, error) {
	return m.dao.ListChannels(ctx)
}

// GetChannel 返回单个渠道；found=false 表示不存在。
func (m *Manager) GetChannel(ctx context.Context, id int64) (Channel, bool, error) {
	return m.dao.GetChannel(ctx, id)
}

// ValidateChannel 校验渠道关键字段。
func ValidateChannel(c Channel) error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("渠道 name 不能为空")
	}
	switch c.Platform {
	case PlatformOpenAI, PlatformAnthropic:
	default:
		return fmt.Errorf("platform 非法: %q（应为 openai|anthropic）", c.Platform)
	}
	switch c.Type {
	case TypeAPIKey, TypeUpstream:
	default:
		return fmt.Errorf("type 非法: %q（应为 api_key|upstream）", c.Type)
	}
	return nil
}

// SaveChannel 创建/更新渠道（c.ID==0 创建）。cred 非 nil 时一并写凭证文件。
// 顺序：校验 → 事务写库（渠道 + abilities）→ 写凭证 → 增量更新 RouteIndex。
func (m *Manager) SaveChannel(ctx context.Context, c Channel, cred *credstore.Cred) (Channel, error) {
	// 领域默认值（缺省安全值；priority 的"未设"默认由 API 层处理）。
	if c.Group == "" {
		c.Group = "default"
	}
	if c.Status == "" {
		c.Status = "active"
	}
	if c.Weight <= 0 {
		c.Weight = 1
	}
	if err := ValidateChannel(c); err != nil {
		return Channel{}, err
	}

	saved, err := m.dao.SaveChannelTx(ctx, c)
	if err != nil {
		return Channel{}, err
	}

	if cred != nil {
		if err := m.creds.Put(saved.ID, *cred); err != nil {
			return saved, fmt.Errorf("渠道已保存但写入凭证失败: %w", err)
		}
	}

	// 增量更新内存索引（禁用渠道等价于移除）。
	m.index.Set(ChannelAbilities{Channel: saved, Abilities: BuildAbilities(saved)})
	return saved, nil
}

// DeleteChannel 删除渠道：DB（abilities 级联）→ 凭证文件 → RouteIndex。
func (m *Manager) DeleteChannel(ctx context.Context, id int64) error {
	if err := m.dao.DeleteChannel(ctx, id); err != nil {
		return err
	}
	if err := m.creds.Delete(id); err != nil {
		return fmt.Errorf("渠道已删除但清理凭证失败: %w", err)
	}
	m.index.Remove(id)
	return nil
}

// SetChannelStatus 回写渠道状态（渠道测试结果）。
func (m *Manager) SetChannelStatus(ctx context.Context, id int64, status, errMsg string) error {
	return m.dao.SetChannelStatus(ctx, id, status, errMsg)
}

// Cred 返回某渠道的凭证（渠道测试用）。
func (m *Manager) Cred(channelID int64) (credstore.Cred, bool) {
	return m.creds.Get(channelID)
}

// ---- 入站 key ----

// CreateKey 生成并保存一条入站 key，返回**仅此一次**可见的明文与新行 id。
func (m *Manager) CreateKey(ctx context.Context, name, group string) (plaintext string, id int64, err error) {
	plaintext, err = apikey.GenerateKey()
	if err != nil {
		return "", 0, fmt.Errorf("生成 key 失败: %w", err)
	}
	id, err = m.dao.CreateAPIKey(ctx, apikey.HashKey(plaintext), name, group)
	if err != nil {
		return "", 0, err
	}
	return plaintext, id, nil
}

// ListKeys 返回全部入站 key 的可展示信息（不含明文/哈希）。
func (m *Manager) ListKeys(ctx context.Context) ([]apikey.KeyInfo, error) {
	return m.dao.ListAPIKeys(ctx)
}

// SetKeyEnabled 启用/禁用入站 key。
func (m *Manager) SetKeyEnabled(ctx context.Context, id int64, enabled bool) error {
	return m.dao.SetAPIKeyEnabled(ctx, id, enabled)
}

// DeleteKey 删除入站 key。
func (m *Manager) DeleteKey(ctx context.Context, id int64) error {
	return m.dao.DeleteAPIKey(ctx, id)
}

// LookupKeyByHash 供鉴权缓存回源。
func (m *Manager) LookupKeyByHash(hash string) (apikey.KeyMeta, bool, error) {
	return m.dao.GetAPIKeyByHash(context.Background(), hash)
}
