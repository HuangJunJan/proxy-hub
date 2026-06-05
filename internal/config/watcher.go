package config

import (
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// debounceInterval 是文件变更的防抖间隔：避免编辑器多次写触发多次重载。
const debounceInterval = 200 * time.Millisecond

// Watcher 监听配置文件变更并触发热重载回调。
type Watcher struct {
	path    string
	watcher *fsnotify.Watcher
	// onReload 在防抖后收到新配置时被调用。仅传入热生效后的配置。
	onReload func(*Config)

	mu     sync.Mutex
	closed bool
	done   chan struct{}
}

// Watch 监听 path 指向的配置文件；文件变更经 200ms 防抖后重新 Load，
// 并把**非密钥**键（server 超时、log level、retention 等）合并进回调。
//
// 密钥类（admin_key）不热重载——变更需重启，避免半态。
// 返回的 Watcher 需在不再使用时调用 Close。
func Watch(path string, onReload func(*Config)) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// fsnotify 监听目录比监听单文件更可靠（很多编辑器是“写临时文件 + rename”，
	// 直接监听文件会在 rename 后丢失 inode）。故监听父目录，再按文件名过滤事件。
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := fw.Add(dir); err != nil {
		_ = fw.Close()
		return nil, err
	}

	w := &Watcher{
		path:     path,
		watcher:  fw,
		onReload: onReload,
		done:     make(chan struct{}),
	}
	go w.loop()
	return w, nil
}

// loop 是事件循环：对目标文件的写/创建/重命名事件做防抖后触发重载。
func (w *Watcher) loop() {
	target, _ := filepath.Abs(w.path)
	var timer *time.Timer
	// debounceC 在防抖窗口结束后触发；初始为 nil 表示无待处理重载。
	var debounceC <-chan time.Time

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case ev, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			evPath, _ := filepath.Abs(ev.Name)
			if evPath != target {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			// 重置防抖定时器。
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(debounceInterval)
			debounceC = timer.C

		case <-debounceC:
			debounceC = nil
			w.reload()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("配置文件监听出错", "error", err)
		}
	}
}

// reload 重新加载配置并把非密钥键交给回调。
func (w *Watcher) reload() {
	cfg, err := Load(w.path)
	if err != nil {
		slog.Warn("配置热重载失败，保留旧配置", "path", w.path, "error", err)
		return
	}
	// 显式丢弃热重载阶段的密钥类字段，避免半态：admin_key 不热生效，重启才接管。
	cfg.AdminKey = ""
	cfg.adminKeyGenerated = false

	slog.Info("配置已热重载（非密钥键生效）",
		"path", w.path,
		"server.addr", cfg.Server.Addr,
		"log.level", cfg.Log.Level,
		"retention_days", cfg.RetentionDays,
	)
	if w.onReload != nil {
		w.onReload(cfg)
	}
}

// Close 停止监听并释放底层资源。可重复调用。
func (w *Watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	close(w.done)
	return w.watcher.Close()
}
