// Package credstore 管理上游渠道凭证的文件级存储：data/auths/<channelID>.json（权限 0600）。
//
// 凭证 blob **绝不**进 SQLite、绝不记日志、绝不进 UsageEvent。启动时全量加载，运行期经 fsnotify
// 监听 auths 目录增量重载（支持外部编辑/多进程场景）。读写均走内存缓存，写时原子落盘。
package credstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// fileSuffix 是凭证文件后缀。文件名形如 "<channelID>.json"。
const fileSuffix = ".json"

// Cred 是单个渠道的凭证。
//   - api_key 渠道：仅 APIKey（BaseURL 为空，序列化时省略）。
//   - upstream 渠道：BaseURL + APIKey。
type Cred struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url,omitempty"`
}

// Store 是凭证的内存缓存 + 文件后端。
type Store struct {
	dir string

	mu    sync.RWMutex
	creds map[int64]Cred

	watcher *fsnotify.Watcher
	closed  bool
	done    chan struct{}
}

// Open 确保 authsDir 存在（0700），全量加载已有凭证，并启动 fsnotify 监听。
func Open(authsDir string) (*Store, error) {
	if err := os.MkdirAll(authsDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建凭证目录 %s 失败: %w", authsDir, err)
	}
	s := &Store{
		dir:   authsDir,
		creds: make(map[int64]Cred),
		done:  make(chan struct{}),
	}
	if err := s.loadAll(); err != nil {
		return nil, err
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("创建凭证监听器失败: %w", err)
	}
	if err := fw.Add(authsDir); err != nil {
		_ = fw.Close()
		return nil, fmt.Errorf("监听凭证目录失败: %w", err)
	}
	s.watcher = fw
	go s.loop()
	return s, nil
}

// loadAll 全量扫描目录加载所有凭证文件（坏文件跳过并告警，不致命）。
func (s *Store) loadAll() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("读取凭证目录 %s 失败: %w", s.dir, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := parseChannelID(e.Name())
		if !ok {
			continue
		}
		c, err := s.readFile(id)
		if err != nil {
			slog.Warn("跳过无法解析的凭证文件", "file", e.Name(), "error", err)
			continue
		}
		s.creds[id] = c
	}
	return nil
}

// loop 监听目录事件：写/创建 → 重载该文件；删除/重命名 → 从缓存移除。
func (s *Store) loop() {
	for {
		select {
		case <-s.done:
			return
		case ev, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			id, valid := parseChannelID(filepath.Base(ev.Name))
			if !valid {
				continue
			}
			switch {
			case ev.Op&(fsnotify.Write|fsnotify.Create) != 0:
				c, err := s.readFile(id)
				if err != nil {
					slog.Warn("凭证文件重载失败，保留旧值", "channel_id", id, "error", err)
					continue
				}
				s.mu.Lock()
				s.creds[id] = c
				s.mu.Unlock()
			case ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0:
				s.mu.Lock()
				delete(s.creds, id)
				s.mu.Unlock()
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("凭证目录监听出错", "error", err)
		}
	}
}

// readFile 读取并解析单个渠道的凭证文件。
func (s *Store) readFile(channelID int64) (Cred, error) {
	data, err := os.ReadFile(s.pathFor(channelID))
	if err != nil {
		return Cred{}, err
	}
	var c Cred
	if err := json.Unmarshal(data, &c); err != nil {
		return Cred{}, fmt.Errorf("解析凭证 JSON 失败: %w", err)
	}
	return c, nil
}

// Get 返回指定渠道的凭证。
func (s *Store) Get(channelID int64) (Cred, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.creds[channelID]
	return c, ok
}

// Put 原子写入凭证（temp + rename，权限 0600）并更新缓存。
func (s *Store) Put(channelID int64, c Cred) error {
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化凭证失败: %w", err)
	}
	path := s.pathFor(channelID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("写入临时凭证文件失败: %w", err)
	}
	// 确保权限为 0600（WriteFile 受 umask 影响时兜底）。
	_ = os.Chmod(tmp, 0o600)
	if err := withFileOpRetry(func() error { return os.Rename(tmp, path) }); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("重命名凭证文件失败: %w", err)
	}
	s.mu.Lock()
	s.creds[channelID] = c
	s.mu.Unlock()
	return nil
}

// Delete 删除渠道凭证文件与缓存项。文件不存在视为成功。
func (s *Store) Delete(channelID int64) error {
	s.mu.Lock()
	delete(s.creds, channelID)
	s.mu.Unlock()
	if err := withFileOpRetry(func() error { return os.Remove(s.pathFor(channelID)) }); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除凭证文件失败: %w", err)
	}
	return nil
}

// Close 停止监听。可重复调用。
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	if s.watcher != nil {
		return s.watcher.Close()
	}
	return nil
}

// pathFor 返回渠道凭证文件路径。
func (s *Store) pathFor(channelID int64) string {
	return filepath.Join(s.dir, strconv.FormatInt(channelID, 10)+fileSuffix)
}

// withFileOpRetry 对可能遭遇 Windows 瞬时共享冲突的文件操作（remove / rename）做有界重试。
//
// 背景：fsnotify 监听协程会在 Put 触发写事件后用 os.ReadFile 读取同名文件，而 Go 在 Windows 上
// 打开文件未设置 FILE_SHARE_DELETE；若该读取与 Delete 的 os.Remove（或 Put 的 rename 覆盖）并发，
// 会因 ERROR_SHARING_VIOLATION 失败。文件读取极短，有界重试即可让其完成。ErrNotExist 立即返回
// （Delete 视其为成功）；非 Windows 平台首次即成功，无额外开销。
func withFileOpRetry(op func() error) error {
	const attempts = 20
	var err error
	for i := 0; i < attempts; i++ {
		if err = op(); err == nil || errors.Is(err, os.ErrNotExist) {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	return err
}

// parseChannelID 从文件名 "<id>.json" 解析渠道 ID；不匹配（如 .tmp）返回 false。
func parseChannelID(name string) (int64, bool) {
	if !strings.HasSuffix(name, fileSuffix) {
		return 0, false
	}
	idStr := strings.TrimSuffix(name, fileSuffix)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
