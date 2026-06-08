// Package config 负责加载、校验与热重载 proxy-hub 的配置。
//
// 配置来源优先级（后者覆盖前者）：内置默认值 → config.yaml → 环境变量（前缀 PROXY_HUB_）。
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// 环境变量前缀。嵌套键以下划线连接，例如 server.addr 对应 PROXY_HUB_SERVER_ADDR。
const envPrefix = "PROXY_HUB_"

// ServerConfig 是 HTTP 服务器相关配置。
type ServerConfig struct {
	Addr         string        `yaml:"addr"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	// BodyLimit 是请求体大小上限（字节）。0 表示使用默认值 32MB。
	BodyLimit int64 `yaml:"body_limit"`
}

// LogConfig 是日志相关配置。
type LogConfig struct {
	// Level 取值 debug|info|warn|error。
	Level string `yaml:"level"`
	// Format 取值 text|json。
	Format string `yaml:"format"`
}

// RelayConfig 是中转相关配置。
type RelayConfig struct {
	// MaxRetries 是单次请求跨渠道重试的最大次数（默认 2）。
	MaxRetries int `yaml:"max_retries"`
	// EnableCrossDialect 控制是否启用跨方言转换（默认 false；一致性套件绿后再开）。
	EnableCrossDialect bool `yaml:"enable_cross_dialect"`
	// UsageBuffer 是用量事件通道容量（默认 16384；0 表示用默认）。
	UsageBuffer int `yaml:"usage_buffer"`
}

// StatsConfig 是统计采集器相关配置。
type StatsConfig struct {
	// BatchSize 是事实行批量插入阈值（默认 100）。
	BatchSize int `yaml:"batch_size"`
	// BatchIntervalMs 是批量插入的最大等待毫秒（默认 200）。
	BatchIntervalMs int `yaml:"batch_interval_ms"`
	// FlushIntervalS 是滚动汇总 UPSERT flush 周期秒（默认 60）。
	FlushIntervalS int `yaml:"flush_interval_s"`
	// SyncFallbackOnFull 为 true 时用量通道满改同步插入兜底（绝不丢计费，代价是热路径偶发阻塞）；默认 false（计数丢弃）。
	SyncFallbackOnFull bool `yaml:"sync_fallback_on_full"`
}

// BatchInterval 返回批量插入最大等待时长。
func (s StatsConfig) BatchInterval() time.Duration {
	return time.Duration(s.BatchIntervalMs) * time.Millisecond
}

// FlushInterval 返回滚动汇总 flush 周期。
func (s StatsConfig) FlushInterval() time.Duration {
	return time.Duration(s.FlushIntervalS) * time.Second
}

// Config 是 proxy-hub 的完整配置模型。
type Config struct {
	Server ServerConfig `yaml:"server"`
	// DataDir 是数据目录；派生出 db_path、auths_dir 等子路径。
	DataDir string `yaml:"data_dir"`
	// AdminKey 是管理端鉴权密钥。为空则首次运行自动生成并打印一次。
	AdminKey string      `yaml:"admin_key"`
	Log      LogConfig   `yaml:"log"`
	Relay    RelayConfig `yaml:"relay"`
	Stats    StatsConfig `yaml:"stats"`
	// RetentionDays 是原始请求日志保留天数（汇总不受影响）。
	RetentionDays int `yaml:"retention_days"`

	// adminKeyGenerated 标记本次运行 AdminKey 是否为自动生成（需打印一次提示）。不参与序列化。
	adminKeyGenerated bool `yaml:"-"`
}

// Default 返回内置默认配置。
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:         ":7777",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			BodyLimit:    32 << 20, // 32MB
		},
		DataDir:  "./data",
		AdminKey: "",
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Relay: RelayConfig{
			MaxRetries:         2,
			EnableCrossDialect: false,
			UsageBuffer:        16384,
		},
		Stats: StatsConfig{
			BatchSize:          100,
			BatchIntervalMs:    200,
			FlushIntervalS:     60,
			SyncFallbackOnFull: false,
		},
		RetentionDays: 30,
	}
}

// Load 按 默认值 → yaml 文件 → 环境变量 的顺序加载配置，最后做校验。
//
// path 为空或文件不存在时，跳过 yaml 步骤（容错），仅用默认值 + 环境变量。
func Load(path string) (*Config, error) {
	cfg := Default()

	// 1. yaml 文件覆盖（存在才读）。
	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
			}
		case errors.Is(err, os.ErrNotExist):
			// 文件不存在视为正常：使用默认值 + 环境变量。
		default:
			return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
		}
	}

	// 2. 环境变量覆盖。
	applyEnv(cfg)

	// 3. 校验。
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyEnv 用 PROXY_HUB_* 环境变量覆盖配置中的对应键。
func applyEnv(cfg *Config) {
	if v, ok := lookupEnv("SERVER_ADDR"); ok {
		cfg.Server.Addr = v
	}
	if v, ok := lookupEnv("SERVER_READ_TIMEOUT"); ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.ReadTimeout = d
		}
	}
	if v, ok := lookupEnv("SERVER_WRITE_TIMEOUT"); ok {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Server.WriteTimeout = d
		}
	}
	if v, ok := lookupEnv("SERVER_BODY_LIMIT"); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Server.BodyLimit = n
		}
	}
	if v, ok := lookupEnv("DATA_DIR"); ok {
		cfg.DataDir = v
	}
	if v, ok := lookupEnv("ADMIN_KEY"); ok {
		cfg.AdminKey = v
	}
	if v, ok := lookupEnv("LOG_LEVEL"); ok {
		cfg.Log.Level = v
	}
	if v, ok := lookupEnv("LOG_FORMAT"); ok {
		cfg.Log.Format = v
	}
	if v, ok := lookupEnv("RETENTION_DAYS"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RetentionDays = n
		}
	}
	if v, ok := lookupEnv("RELAY_MAX_RETRIES"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Relay.MaxRetries = n
		}
	}
	if v, ok := lookupEnv("RELAY_ENABLE_CROSS_DIALECT"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Relay.EnableCrossDialect = b
		}
	}
	if v, ok := lookupEnv("RELAY_USAGE_BUFFER"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Relay.UsageBuffer = n
		}
	}
	if v, ok := lookupEnv("STATS_BATCH_SIZE"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Stats.BatchSize = n
		}
	}
	if v, ok := lookupEnv("STATS_BATCH_INTERVAL_MS"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Stats.BatchIntervalMs = n
		}
	}
	if v, ok := lookupEnv("STATS_FLUSH_INTERVAL_S"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Stats.FlushIntervalS = n
		}
	}
	if v, ok := lookupEnv("STATS_SYNC_FALLBACK_ON_FULL"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Stats.SyncFallbackOnFull = b
		}
	}
}

// lookupEnv 读取带前缀的环境变量；返回值与是否存在。
func lookupEnv(suffix string) (string, bool) {
	return os.LookupEnv(envPrefix + suffix)
}

// validate 校验关键字段的合法性。
func (c *Config) validate() error {
	if strings.TrimSpace(c.Server.Addr) == "" {
		return errors.New("server.addr 不能为空")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return errors.New("data_dir 不能为空")
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level 非法: %q（应为 debug|info|warn|error）", c.Log.Level)
	}
	switch c.Log.Format {
	case "text", "json":
	default:
		return fmt.Errorf("log.format 非法: %q（应为 text|json）", c.Log.Format)
	}
	if c.RetentionDays < 0 {
		return fmt.Errorf("retention_days 不能为负: %d", c.RetentionDays)
	}
	if c.Relay.MaxRetries < 0 {
		return fmt.Errorf("relay.max_retries 不能为负: %d", c.Relay.MaxRetries)
	}
	if c.Relay.UsageBuffer < 0 {
		return fmt.Errorf("relay.usage_buffer 不能为负: %d", c.Relay.UsageBuffer)
	}
	if c.Stats.BatchSize < 0 {
		return fmt.Errorf("stats.batch_size 不能为负: %d", c.Stats.BatchSize)
	}
	if c.Stats.BatchIntervalMs < 0 {
		return fmt.Errorf("stats.batch_interval_ms 不能为负: %d", c.Stats.BatchIntervalMs)
	}
	if c.Stats.FlushIntervalS < 0 {
		return fmt.Errorf("stats.flush_interval_s 不能为负: %d", c.Stats.FlushIntervalS)
	}
	return nil
}

// DBPath 返回 SQLite 数据库文件路径（data_dir/proxy-hub.db）。
func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "proxy-hub.db")
}

// AuthsDir 返回上游凭证目录路径（data_dir/auths）。
func (c *Config) AuthsDir() string {
	return filepath.Join(c.DataDir, "auths")
}

// AdminKeyGenerated 返回本次运行 AdminKey 是否为自动生成（用于决定是否打印提示）。
func (c *Config) AdminKeyGenerated() bool {
	return c.adminKeyGenerated
}

// EnsureAdminKey 在 AdminKey 为空时随机生成 32 字节（hex 编码 64 字符）密钥，
// 并标记为本次自动生成（调用方据此打印一次）。已设置则原样保留。
func (c *Config) EnsureAdminKey() error {
	if strings.TrimSpace(c.AdminKey) != "" {
		return nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("生成 admin_key 失败: %w", err)
	}
	c.AdminKey = hex.EncodeToString(buf)
	c.adminKeyGenerated = true
	return nil
}
