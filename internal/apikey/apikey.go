// Package apikey 提供入站 API key 的哈希、生成与内存查找缓存（含负缓存）。
//
// 设计为 dbgen 无关：缓存未命中时经注入的 Loader 回源（dao 用 GetAPIKeyByHash 实现），
// 命中结果与"确实不存在"分别进正/负缓存。api_keys 发生变更时由管理端调用 Invalidate 失效缓存。
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync"
)

// keyPrefix 是平台发放入站 key 的可读前缀（仅明文展示用，不参与哈希语义）。
const keyPrefix = "sk-ph-"

// HashKey 返回明文 key 的 sha256 十六进制摘要（DB 只存此摘要）。
func HashKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// GenerateKey 生成一个新的入站 key 明文（前缀 + 32 字节随机十六进制）。明文仅创建时返回一次。
func GenerateKey() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return keyPrefix + hex.EncodeToString(b[:]), nil
}

// KeyMeta 是一条入站 key 的鉴权相关元数据（不含明文/摘要本身）。
type KeyMeta struct {
	ID      int64
	Group   string
	Enabled bool
}

// KeyInfo 是入站 key 的可展示信息（管理端列表用；绝不含明文或哈希）。
type KeyInfo struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Group      string `json:"group"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

// Loader 按 hash 回源查 key 元数据；ok=false 表示该 hash 不存在。
type Loader func(hash string) (KeyMeta, bool, error)

// Cache 是 hash -> KeyMeta 的内存缓存，含负缓存。回源失败时 fail-closed（视为不存在）。
type Cache struct {
	mu       sync.RWMutex
	positive map[string]KeyMeta
	negative map[string]struct{}
	load     Loader
}

// NewCache 创建缓存。load 为回源函数（不可为 nil）。
func NewCache(load Loader) *Cache {
	return &Cache{
		positive: map[string]KeyMeta{},
		negative: map[string]struct{}{},
		load:     load,
	}
}

// Lookup 按 hash 查 key 元数据：先查正/负缓存，未命中回源并写入对应缓存。
// 回源出错时返回 ok=false（fail-closed，不缓存错误以便下次重试）。
func (c *Cache) Lookup(hash string) (KeyMeta, bool) {
	c.mu.RLock()
	if m, ok := c.positive[hash]; ok {
		c.mu.RUnlock()
		return m, true
	}
	if _, neg := c.negative[hash]; neg {
		c.mu.RUnlock()
		return KeyMeta{}, false
	}
	c.mu.RUnlock()

	m, ok, err := c.load(hash)
	if err != nil {
		slog.Warn("回源查询入站 key 失败", "error", err)
		return KeyMeta{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if ok {
		c.positive[hash] = m
		return m, true
	}
	c.negative[hash] = struct{}{}
	return KeyMeta{}, false
}

// Invalidate 清空全部缓存（api_keys 增删改后调用，简单且渠道/ key 量小时代价可忽略）。
func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.positive = map[string]KeyMeta{}
	c.negative = map[string]struct{}{}
}
