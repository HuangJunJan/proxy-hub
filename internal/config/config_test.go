package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadDefaults 验证缺失文件时返回默认配置。
func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "不存在.yaml"))
	if err != nil {
		t.Fatalf("缺失文件应容错，却返回错误: %v", err)
	}
	if cfg.Server.Addr != ":7777" {
		t.Errorf("默认 server.addr 应为 :7777，实际 %q", cfg.Server.Addr)
	}
	if cfg.DataDir != "./data" {
		t.Errorf("默认 data_dir 应为 ./data，实际 %q", cfg.DataDir)
	}
	if cfg.RetentionDays != 30 {
		t.Errorf("默认 retention_days 应为 30，实际 %d", cfg.RetentionDays)
	}
	if cfg.Log.Level != "info" || cfg.Log.Format != "text" {
		t.Errorf("默认日志应为 info/text，实际 %s/%s", cfg.Log.Level, cfg.Log.Format)
	}
	if cfg.Relay.MaxRetries != 2 || cfg.Relay.EnableCrossDialect || cfg.Relay.UsageBuffer != 16384 {
		t.Errorf("默认 relay 应为 2/false/16384，实际 %+v", cfg.Relay)
	}
}

// TestRelayConfig 验证 relay 段的 yaml 与环境变量覆盖。
func TestRelayConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
relay:
  max_retries: 5
  enable_cross_dialect: true
  usage_buffer: 1024
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Relay.MaxRetries != 5 || !cfg.Relay.EnableCrossDialect || cfg.Relay.UsageBuffer != 1024 {
		t.Errorf("yaml 应覆盖 relay 为 5/true/1024，实际 %+v", cfg.Relay)
	}

	t.Setenv("PROXY_HUB_RELAY_MAX_RETRIES", "0")
	t.Setenv("PROXY_HUB_RELAY_ENABLE_CROSS_DIALECT", "false")
	cfg2, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg2.Relay.MaxRetries != 0 || cfg2.Relay.EnableCrossDialect {
		t.Errorf("环境变量应覆盖 relay，max_retries=0/cross=false，实际 %+v", cfg2.Relay)
	}
}

// TestLoadYAMLOverride 验证 yaml 文件覆盖默认值。
func TestLoadYAMLOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
server:
  addr: ":9090"
  read_timeout: 15s
data_dir: "/srv/data"
log:
  level: "debug"
  format: "json"
retention_days: 7
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Server.Addr != ":9090" {
		t.Errorf("server.addr 应被 yaml 覆盖为 :9090，实际 %q", cfg.Server.Addr)
	}
	if cfg.Server.ReadTimeout != 15*time.Second {
		t.Errorf("read_timeout 应为 15s，实际 %v", cfg.Server.ReadTimeout)
	}
	if cfg.DataDir != "/srv/data" {
		t.Errorf("data_dir 应为 /srv/data，实际 %q", cfg.DataDir)
	}
	if cfg.Log.Level != "debug" || cfg.Log.Format != "json" {
		t.Errorf("日志应为 debug/json，实际 %s/%s", cfg.Log.Level, cfg.Log.Format)
	}
	if cfg.RetentionDays != 7 {
		t.Errorf("retention_days 应为 7，实际 %d", cfg.RetentionDays)
	}
}

// TestLoadEnvOverride 验证环境变量覆盖文件键（优先级最高）。
func TestLoadEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  addr: \":9090\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROXY_HUB_SERVER_ADDR", ":7070")
	t.Setenv("PROXY_HUB_RETENTION_DAYS", "5")
	t.Setenv("PROXY_HUB_LOG_LEVEL", "warn")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Server.Addr != ":7070" {
		t.Errorf("环境变量应覆盖 yaml，server.addr 期望 :7070，实际 %q", cfg.Server.Addr)
	}
	if cfg.RetentionDays != 5 {
		t.Errorf("retention_days 期望 5，实际 %d", cfg.RetentionDays)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("log.level 期望 warn，实际 %q", cfg.Log.Level)
	}
}

// TestLoadValidationError 验证非法配置被拒绝。
func TestLoadValidationError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("log:\n  level: \"verbose\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("非法 log.level 应返回校验错误")
	}
}

// TestDerivedPaths 验证派生路径方法。
func TestDerivedPaths(t *testing.T) {
	cfg := Default()
	cfg.DataDir = filepath.Join("x", "y")
	if got, want := cfg.DBPath(), filepath.Join("x", "y", "proxy-hub.db"); got != want {
		t.Errorf("DBPath 期望 %q，实际 %q", want, got)
	}
	if got, want := cfg.AuthsDir(), filepath.Join("x", "y", "auths"); got != want {
		t.Errorf("AuthsDir 期望 %q，实际 %q", want, got)
	}
}

// TestEnsureAdminKey 验证空 key 自动生成并标记，已设置 key 原样保留。
func TestEnsureAdminKey(t *testing.T) {
	cfg := Default()
	if err := cfg.EnsureAdminKey(); err != nil {
		t.Fatalf("生成 admin_key 失败: %v", err)
	}
	if len(cfg.AdminKey) != 64 { // 32 字节 hex = 64 字符
		t.Errorf("自动生成的 admin_key 应为 64 字符 hex，实际长度 %d", len(cfg.AdminKey))
	}
	if !cfg.AdminKeyGenerated() {
		t.Error("自动生成时 AdminKeyGenerated 应为 true")
	}

	preset := Default()
	preset.AdminKey = "预设密钥"
	if err := preset.EnsureAdminKey(); err != nil {
		t.Fatal(err)
	}
	if preset.AdminKey != "预设密钥" {
		t.Errorf("已设置的 admin_key 应原样保留，实际 %q", preset.AdminKey)
	}
	if preset.AdminKeyGenerated() {
		t.Error("已设置 key 时 AdminKeyGenerated 应为 false")
	}
}
