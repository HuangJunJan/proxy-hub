package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAcceptsMinimalSetupConfig(t *testing.T) {
	cfg := validConfig()
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsDuplicateChannelNames(t *testing.T) {
	cfg := validConfig()
	cfg.OpenAIAPI = append(cfg.OpenAIAPI, cfg.OpenAIAPI[0])
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate error")
	}
	if !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("Validate() error = %v, want duplicate message", err)
	}
}

func TestNormalizeOmitsDefaultsOnSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	m := NewManager(path, nil)

	err := m.Save(func(cfg *Config) error {
		*cfg = *validConfig()
		cfg.Server.Host = DefaultHost
		cfg.Server.Port = DefaultPort
		cfg.RequestLog.RetentionDays = DefaultRetentionDays
		cfg.RequestLog.BodyMode = DefaultRequestBodyMode
		cfg.RequestLog.MaxBodyBytes = DefaultMaxBodyBytes
		cfg.Scheduler.MaxRetries = DefaultMaxRetries
		cfg.Scheduler.CircuitCooldownSec = DefaultCircuitCooldownSec
		cfg.Scheduler.CircuitFailureThreshold = DefaultCircuitFailureThreshold
		cfg.OpenAIAPI[0].Priority = DefaultPriority
		cfg.OpenAIAPI[0].TimeoutSec = DefaultTimeoutSec
		return nil
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	for _, unwanted := range []string{
		"host: 0.0.0.0",
		"port: 8787",
		"retention-days: 7",
		"body-mode: failed_only",
		"max-body-bytes: 65536",
		"priority: 100",
		"timeout-sec: 120",
		"max-retries: 2",
		"circuit-cooldown-sec: 60",
		"circuit-failure-threshold: 3",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("saved config contains default %q:\n%s", unwanted, text)
		}
	}
}

func TestLoadMissingFileEntersSetupMode(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "missing.yaml"), nil)
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !m.SetupNeeded() {
		t.Fatal("SetupNeeded() = false, want true")
	}
}

func validConfig() *Config {
	return &Config{
		Server: ServerConfig{Port: 8787},
		Admin: &AdminConfig{
			Username:     "admin",
			PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$abc$def",
		},
		APIKeys: []APIKeyConfig{
			{Token: "sk_proxy_hub_12345678901234567890", Name: "local"},
		},
		OpenAIAPI: []OpenAIAPIChannel{
			{
				Name:    "openai",
				BaseURL: "https://api.openai.com",
				APIKeyEntries: []APIKeyEntry{
					{APIKey: "sk-test"},
				},
				Models: []ModelEntry{
					{Name: "gpt-4o"},
					{Name: "gpt-4o", Alias: "gpt-5.4"},
				},
			},
		},
	}
}
