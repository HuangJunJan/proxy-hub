package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/huangjunjan/proxy-hub/internal/config"
)

// openTestDB 在临时目录打开一个写句柄（MaxOpenConns(1)）。
func openTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", buildDSN(dbPath))
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db, dbPath
}

// TestRunFreshDB 验证全新库应用迁移后 schema_version=1。
func TestRunFreshDB(t *testing.T) {
	db, dbPath := openTestDB(t)
	if err := Run(db, dbPath); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	ver, err := readSchemaVersion(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if ver != 1 {
		t.Errorf("迁移后版本应为 1，实际 %d", ver)
	}
	// meta 表应存在且可查询。
	var name string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='meta'`).Scan(&name)
	if err != nil {
		t.Fatalf("meta 表应已创建: %v", err)
	}
}

// TestRunIdempotent 验证重复运行为 no-op，且不产生新备份。
func TestRunIdempotent(t *testing.T) {
	db, dbPath := openTestDB(t)
	if err := Run(db, dbPath); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	// 第二次运行：版本不变、无新增备份。
	if err := Run(db, dbPath); err != nil {
		t.Fatalf("二次迁移应 no-op，却出错: %v", err)
	}
	ver, _ := readSchemaVersion(context.Background(), db)
	if ver != 1 {
		t.Errorf("二次运行后版本仍应为 1，实际 %d", ver)
	}
	// 由 v0 升级时生成了 .bak-0；no-op 不应再生成其它备份。
	if _, err := os.Stat(dbPath + ".bak-1"); !os.IsNotExist(err) {
		t.Errorf("no-op 不应生成 .bak-1 备份")
	}
}

// TestRunBackupCreated 验证首次升级前生成预迁移备份 .bak-0。
func TestRunBackupCreated(t *testing.T) {
	db, dbPath := openTestDB(t)
	if err := Run(db, dbPath); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if _, err := os.Stat(dbPath + ".bak-0"); err != nil {
		t.Errorf("应生成预迁移备份 .bak-0: %v", err)
	}
}

// TestRunBadMigrationRollback 模拟坏迁移：事务回滚、版本不变、备份仍在。
func TestRunBadMigrationRollback(t *testing.T) {
	db, dbPath := openTestDB(t)
	ctx := context.Background()

	// 建 meta（模拟已有库）。
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}

	// 直接复用内部流程难以注入坏 SQL，这里用一个内联的“坏迁移”验证事务回滚语义：
	// 在事务中执行一条非法 SQL，确认整体回滚、版本未被写入。
	if _, err := db.ExecContext(ctx, `VACUUM INTO `+quoteSQLString(dbPath+".bak-0")); err != nil {
		t.Fatalf("预迁移备份失败: %v", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, execErr := tx.ExecContext(ctx, `CREATE TABLE 合法表 (id INTEGER PRIMARY KEY)`)
	if execErr != nil {
		t.Fatal(execErr)
	}
	// 坏 SQL 触发回滚。
	if _, err := tx.ExecContext(ctx, `THIS IS NOT VALID SQL`); err == nil {
		t.Fatal("坏 SQL 应报错")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// 版本应仍为 0（未写入）。
	ver, _ := readSchemaVersion(ctx, db)
	if ver != 0 {
		t.Errorf("坏迁移回滚后版本应仍为 0，实际 %d", ver)
	}
	// 回滚后“合法表”不应存在。
	var cnt int
	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='合法表'`).Scan(&cnt)
	if cnt != 0 {
		t.Error("事务回滚后表不应残留")
	}
	// 备份文件应存在（恢复点）。
	if _, err := os.Stat(dbPath + ".bak-0"); err != nil {
		t.Errorf("坏迁移后预迁移备份应仍在: %v", err)
	}
}

// TestRebuildTable 验证表重建 helper：建新表→拷数据→删旧→改名。
func TestRebuildTable(t *testing.T) {
	db, _ := openTestDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE 旧表 (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO 旧表 (id, name) VALUES (1, '甲'), (2, '乙')`); err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = rebuildTable(ctx, tx, "旧表", "旧表_new",
		`CREATE TABLE 旧表_new (id INTEGER PRIMARY KEY, name TEXT, extra TEXT DEFAULT '')`,
		[]string{"id", "name"})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("rebuildTable 失败: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	// 数据应被保留，且新增列存在。
	var cnt int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM 旧表`).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 2 {
		t.Errorf("重建后行数应为 2，实际 %d", cnt)
	}
	var extra string
	if err := db.QueryRowContext(ctx, `SELECT extra FROM 旧表 WHERE id=1`).Scan(&extra); err != nil {
		t.Fatalf("新列 extra 应存在: %v", err)
	}
}

// TestHealthCheck 验证正常返回 nil、Close 后返回错误。
func TestHealthCheck(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()

	st, err := Open(cfg)
	if err != nil {
		t.Fatalf("打开 store 失败: %v", err)
	}

	if err := st.HealthCheck(context.Background()); err != nil {
		t.Errorf("正常 store 健康检查应返回 nil，实际 %v", err)
	}

	if err := st.Close(); err != nil {
		t.Fatalf("关闭 store 失败: %v", err)
	}
	if err := st.HealthCheck(context.Background()); err == nil {
		t.Error("关闭后健康检查应返回错误")
	}
}

// TestOpenCreatesDBAndAuths 验证 Open 创建数据库文件与 auths 目录。
func TestOpenCreatesDBAndAuths(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()

	st, err := Open(cfg)
	if err != nil {
		t.Fatalf("打开 store 失败: %v", err)
	}
	defer func() { _ = st.Close() }()

	if _, err := os.Stat(cfg.DBPath()); err != nil {
		t.Errorf("数据库文件应被创建: %v", err)
	}
	if info, err := os.Stat(cfg.AuthsDir()); err != nil || !info.IsDir() {
		t.Errorf("auths 目录应被创建: %v", err)
	}
}
