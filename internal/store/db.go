// Package store 负责打开内嵌 SQLite、应用版本化迁移，并提供读/写句柄与健康检查。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/huangjunjan/proxy-hub/internal/config"

	// 纯 Go SQLite 驱动，注册的驱动名为 "sqlite"，保持 CGO_ENABLED=0。
	_ "modernc.org/sqlite"
)

// pingTimeout 是打开/健康检查时 Ping 的超时。
const pingTimeout = 5 * time.Second

// Store 封装指向同一 SQLite 文件的两个连接句柄，落地“多读单写”模型：
//   - read：默认连接池，承载所有 SELECT。
//   - write：MaxOpenConns(1)，串行化所有 INSERT/UPDATE/DELETE/DDL，契合 SQLite 单写者模型。
type Store struct {
	read  *sql.DB
	write *sql.DB
}

// buildDSN 根据数据库文件路径构造 modernc DSN，启用 WAL 等 pragma。
func buildDSN(dbPath string) string {
	// modernc 用 _pragma= 查询参数在每个连接上执行 PRAGMA。
	return "file:" + dbPath +
		"?_pragma=busy_timeout(30000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(ON)"
}

// Open 确保数据目录存在、打开读写句柄、校验连通性并应用迁移。
func Open(cfg *config.Config) (*Store, error) {
	// 1. 确保数据目录存在（0700）。
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录 %s 失败: %w", cfg.DataDir, err)
	}
	// auths 目录预创建（M2 起写入凭证文件，权限 0700）。
	if err := os.MkdirAll(cfg.AuthsDir(), 0o700); err != nil {
		return nil, fmt.Errorf("创建凭证目录 %s 失败: %w", cfg.AuthsDir(), err)
	}

	dsn := buildDSN(cfg.DBPath())

	// 2. 打开读写两个句柄（同一 DSN）。
	read, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开读句柄失败: %w", err)
	}
	write, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = read.Close()
		return nil, fmt.Errorf("打开写句柄失败: %w", err)
	}

	// 读池大小：max(4, GOMAXPROCS)；写句柄串行化为单连接。
	maxRead := runtime.GOMAXPROCS(0)
	if maxRead < 4 {
		maxRead = 4
	}
	read.SetMaxOpenConns(maxRead)
	write.SetMaxOpenConns(1)

	// 3. 校验连通性。
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := read.PingContext(ctx); err != nil {
		_ = read.Close()
		_ = write.Close()
		return nil, fmt.Errorf("读句柄 ping 失败: %w", err)
	}
	if err := write.PingContext(ctx); err != nil {
		_ = read.Close()
		_ = write.Close()
		return nil, fmt.Errorf("写句柄 ping 失败: %w", err)
	}

	// 4. 应用迁移（仅在写句柄上执行，串行安全）。
	if err := Run(write, cfg.DBPath()); err != nil {
		_ = read.Close()
		_ = write.Close()
		return nil, fmt.Errorf("应用迁移失败: %w", err)
	}

	return &Store{read: read, write: write}, nil
}

// Read 返回读连接池句柄（用于 SELECT）。
func (s *Store) Read() *sql.DB {
	return s.read
}

// Write 返回写句柄（用于所有 INSERT/UPDATE/DELETE/DDL，已串行化为单连接）。
func (s *Store) Write() *sql.DB {
	return s.write
}

// HealthCheck 对读写句柄做 Ping，验证数据库可用。
func (s *Store) HealthCheck(ctx context.Context) error {
	if s == nil || s.read == nil || s.write == nil {
		return errors.New("store 未初始化")
	}
	if err := s.read.PingContext(ctx); err != nil {
		return fmt.Errorf("读句柄不可用: %w", err)
	}
	if err := s.write.PingContext(ctx); err != nil {
		return fmt.Errorf("写句柄不可用: %w", err)
	}
	return nil
}

// Close 关闭读写句柄。即便其中一个出错也会尝试关闭另一个。
func (s *Store) Close() error {
	var errRead, errWrite error
	if s.read != nil {
		errRead = s.read.Close()
	}
	if s.write != nil {
		errWrite = s.write.Close()
	}
	return errors.Join(errRead, errWrite)
}
