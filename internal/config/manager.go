package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gofrs/flock"
	"gopkg.in/yaml.v3"
)

type Manager struct {
	path     string
	snapshot atomic.Pointer[Config]
	setup    atomic.Bool
	saveMu   sync.Mutex
	fileLock *flock.Flock

	subMu       sync.RWMutex
	subscribers []func(*Config)

	logger *slog.Logger
}

func NewManager(path string, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		path:     path,
		fileLock: flock.New(path + ".lock"),
		logger:   logger,
	}
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) SetupNeeded() bool {
	return m.setup.Load()
}

func (m *Manager) Snapshot() *Config {
	cfg := m.snapshot.Load()
	if cfg == nil {
		return &Config{}
	}
	return cfg
}

func (m *Manager) Subscribe(fn func(*Config)) {
	if fn == nil {
		return
	}
	m.subMu.Lock()
	defer m.subMu.Unlock()
	m.subscribers = append(m.subscribers, fn)
}

func (m *Manager) Load() error {
	cfg, setupNeeded, err := loadFile(m.path)
	if err != nil {
		return err
	}
	if !setupNeeded {
		if err := Validate(cfg); err != nil {
			return err
		}
	}
	m.snapshot.Store(cfg)
	m.setup.Store(setupNeeded)
	m.notify(cfg)
	return nil
}

func (m *Manager) Save(mutator func(*Config) error) error {
	if mutator == nil {
		return errors.New("config mutator is nil")
	}

	m.saveMu.Lock()
	defer m.saveMu.Unlock()

	next := Clone(m.Snapshot())
	if err := mutator(next); err != nil {
		return err
	}
	Normalize(next)
	if err := Validate(next); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := m.fileLock.Lock(); err != nil {
		return fmt.Errorf("lock config file: %w", err)
	}
	defer func() {
		if err := m.fileLock.Unlock(); err != nil {
			m.logger.Warn("failed to unlock config file", "error", err)
		}
	}()

	data, err := marshal(next)
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config temp file: %w", err)
	}
	if err := os.Rename(tmp, m.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace config file: %w", err)
	}

	m.snapshot.Store(next)
	m.setup.Store(needsSetup(next))
	m.notify(next)
	return nil
}

func (m *Manager) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create config watcher: %w", err)
	}
	defer watcher.Close()

	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config watch directory: %w", err)
	}
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("watch config directory: %w", err)
	}

	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if filepath.Clean(event.Name) != filepath.Clean(m.path) {
				continue
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(200 * time.Millisecond)
				timerC = timer.C
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			m.logger.Warn("config watcher error", "error", err)
		case <-timerC:
			timerC = nil
			if timer != nil {
				timer.Stop()
				timer = nil
			}
			if err := m.reloadExternal(); err != nil {
				m.logger.Warn("config reload rejected", "error", err)
			}
		}
	}
}

func (m *Manager) reloadExternal() error {
	if ok, err := m.fileLock.TryRLock(); err != nil {
		return fmt.Errorf("read-lock config file: %w", err)
	} else if !ok {
		return errors.New("config file is locked for writing")
	}
	defer func() {
		if err := m.fileLock.Unlock(); err != nil {
			m.logger.Warn("failed to unlock config file after reload", "error", err)
		}
	}()

	cfg, setupNeeded, err := loadFile(m.path)
	if err != nil {
		return err
	}
	if !setupNeeded {
		if err := Validate(cfg); err != nil {
			return err
		}
	}
	m.snapshot.Store(cfg)
	m.setup.Store(setupNeeded)
	m.notify(cfg)
	return nil
}

func (m *Manager) notify(cfg *Config) {
	m.subMu.RLock()
	subscribers := append([]func(*Config){}, m.subscribers...)
	m.subMu.RUnlock()
	for _, fn := range subscribers {
		fn(cfg)
	}
}

func loadFile(path string) (*Config, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read config file: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return &Config{}, true, nil
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("parse config yaml: %w", err)
	}
	return &cfg, needsSetup(&cfg), nil
}

func marshal(cfg *Config) ([]byte, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config yaml: %w", err)
	}
	return data, nil
}

func needsSetup(cfg *Config) bool {
	return cfg == nil ||
		cfg.Admin == nil ||
		cfg.Admin.Username == "" ||
		cfg.Admin.PasswordHash == "" ||
		len(cfg.APIKeys) == 0
}
