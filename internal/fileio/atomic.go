// Package fileio 提供对外部客户端配置文件的安全写入：按路径加锁下「读现状 → transform → 原子落盘」，
// 首写前 .bak 备份、父目录缺失则跳过。锁内读当前再 transform，使活跃客户端在读-写之间的改动不被覆盖。
//
// 用于 MCP 投影写入 ~/.claude.json 与 ~/.codex/config.toml 等外部文件（NFR-5：绝不破坏无关内容）。
package fileio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrParentMissing 表示目标父目录不存在（客户端未安装）：跳过写入，不创建目录树。
var ErrParentMissing = errors.New("父目录不存在，跳过写入")

var (
	locksMu sync.Mutex
	locks   = map[string]*sync.Mutex{}

	bakMu   sync.Mutex
	bakDone = map[string]bool{} // 本进程已为该 path 生成过 .bak
)

func normalize(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

func lockFor(path string) *sync.Mutex {
	key := normalize(path)
	locksMu.Lock()
	defer locksMu.Unlock()
	m := locks[key]
	if m == nil {
		m = &sync.Mutex{}
		locks[key] = m
	}
	return m
}

// UpdateFile 在按 path 加锁下完成「读现状 → transform → 原子落盘」：
//   - 父目录缺失 ⇒ 返回 ErrParentMissing（跳过，不创建目录树）。
//   - 读当前内容（不存在视为空 []byte）；首次触碰已存在文件先写一次 <path>.bak（恢复点）。
//   - transform(current) 得 next；temp+rename 原子写（沿用原文件权限，新建用 0600）。
//
// 锁内「读当前 → transform」保证活跃客户端在读-写之间的外部改动不被覆盖（写前重读语义）。
func UpdateFile(path string, transform func(current []byte) ([]byte, error)) error {
	m := lockFor(path)
	m.Lock()
	defer m.Unlock()

	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ErrParentMissing
	}

	var current []byte
	usePerm := os.FileMode(0o600)
	existed := false
	if fi, err := os.Stat(path); err == nil {
		existed = true
		usePerm = fi.Mode().Perm()
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return fmt.Errorf("读取当前内容失败: %w", rerr)
		}
		current = b
	}

	next, err := transform(current)
	if err != nil {
		return err
	}

	if existed {
		if err := backupOnce(path); err != nil {
			return err
		}
	}
	return atomicReplace(path, dir, next, usePerm)
}

func atomicReplace(path, dir string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // rename 成功后为 no-op
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	_ = os.Chmod(tmpName, perm)
	if err := renameWithRetry(tmpName, path); err != nil {
		return fmt.Errorf("重命名到目标失败: %w", err)
	}
	return nil
}

// backupOnce 在本进程首次触碰某 path 时，把其当前内容拷贝到 <path>.bak。
func backupOnce(path string) error {
	key := normalize(path)
	bakMu.Lock()
	already := bakDone[key]
	bakDone[key] = true
	bakMu.Unlock()
	if already {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取原文件以备份失败: %w", err)
	}
	if err := os.WriteFile(path+".bak", data, 0o600); err != nil {
		return fmt.Errorf("写入 .bak 备份失败: %w", err)
	}
	return nil
}

// renameWithRetry 对 Windows 上目标被占用的瞬时 sharing violation 做有界重试（参考 credstore 经验）。
func renameWithRetry(oldPath, newPath string) error {
	const attempts = 20
	var err error
	for i := 0; i < attempts; i++ {
		if err = os.Rename(oldPath, newPath); err == nil {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return err
}
