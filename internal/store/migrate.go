package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
)

// migrationsFS 内嵌所有迁移 SQL 文件。文件名以数字前缀排序（0001_、0002_ …）。
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// schemaVersionKey 是 meta 表中存储当前 schema 版本号的键。
const schemaVersionKey = "schema_version"

// migration 是一条已解析的迁移：版本号 + SQL 内容。
type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations 解析内嵌迁移文件并按版本号升序排序。
//
// 文件名形如 NNNN_desc.sql，前缀 NNNN 即版本号。
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("读取内嵌迁移目录失败: %w", err)
	}

	var migs []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		// 取下划线前的数字前缀作为版本号。
		prefix, _, found := strings.Cut(e.Name(), "_")
		if !found {
			return nil, fmt.Errorf("迁移文件名缺少版本前缀: %s", e.Name())
		}
		ver, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("迁移文件名版本前缀非法: %s: %w", e.Name(), err)
		}
		content, err := fs.ReadFile(migrationsFS, path.Join("migrations", e.Name()))
		if err != nil {
			return nil, fmt.Errorf("读取迁移文件 %s 失败: %w", e.Name(), err)
		}
		migs = append(migs, migration{version: ver, name: e.Name(), sql: string(content)})
	}

	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })

	// 校验版本号唯一、连续从 1 递增。
	for i, m := range migs {
		if m.version != i+1 {
			return nil, fmt.Errorf("迁移版本号不连续：期望 %d，实际 %d（%s）", i+1, m.version, m.name)
		}
	}
	return migs, nil
}

// Run 在 write 句柄上应用所有待执行迁移。
//
// 流程：
//  1. 建 meta 表（IF NOT EXISTS）；读取当前 schema_version（缺省 0）。
//  2. 若有待应用迁移（version > 当前），先做预迁移备份（VACUUM INTO '<db>.bak-<当前版本>'）。
//  3. 在单个事务内按序执行每条待应用迁移并更新 schema_version；任一失败回滚整个事务。
//
// dbPath 仅用于构造备份文件名。
func Run(write *sql.DB, dbPath string) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// 1. 建 meta 表 + 读当前版本。
	if _, err := write.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("创建 meta 表失败: %w", err)
	}
	current, err := readSchemaVersion(ctx, write)
	if err != nil {
		return err
	}

	// 计算待应用迁移。
	var pending []migration
	for _, m := range migs {
		if m.version > current {
			pending = append(pending, m)
		}
	}
	if len(pending) == 0 {
		slog.Info("数据库 schema 已是最新，无需迁移", "schema_version", current)
		return nil
	}

	// 2. 预迁移备份：VACUUM INTO 生成一致性快照（优于直接拷文件，避免 WAL 半态）。
	if dbPath != "" {
		backupPath := fmt.Sprintf("%s.bak-%d", dbPath, current)
		if _, err := write.ExecContext(ctx, fmt.Sprintf("VACUUM INTO %s", quoteSQLString(backupPath))); err != nil {
			return fmt.Errorf("预迁移备份失败（%s）: %w", backupPath, err)
		}
		slog.Info("已生成预迁移备份", "backup", backupPath, "from_version", current)
	}

	// 3. 事务内按序执行迁移并更新版本。
	tx, err := write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启迁移事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // 已提交后 Rollback 为 no-op。

	for _, m := range pending {
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("执行迁移 %s 失败（事务回滚）: %w", m.name, err)
		}
		if err := setSchemaVersion(ctx, tx, m.version); err != nil {
			return fmt.Errorf("更新 schema_version 至 %d 失败: %w", m.version, err)
		}
		slog.Info("已应用迁移", "name", m.name, "version", m.version)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交迁移事务失败: %w", err)
	}
	slog.Info("迁移完成", "from_version", current, "to_version", pending[len(pending)-1].version)
	return nil
}

// readSchemaVersion 读取当前 schema 版本号；meta 中无记录时返回 0。
func readSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var raw string
	err := db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, schemaVersionKey).Scan(&raw)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, nil
	case err != nil:
		return 0, fmt.Errorf("读取 schema_version 失败: %w", err)
	}
	ver, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("meta.schema_version 值非法 %q: %w", raw, err)
	}
	return ver, nil
}

// setSchemaVersion 在事务内 UPSERT schema 版本号。
func setSchemaVersion(ctx context.Context, tx *sql.Tx, version int) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		schemaVersionKey, strconv.Itoa(version))
	return err
}

// quoteSQLString 把字符串安全地包成 SQL 单引号字面量（用于 VACUUM INTO 这类不支持参数占位的语句）。
func quoteSQLString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// rebuildTable 是表重建迁移模式的 helper，供后续里程碑改表用，规避 SQLite ALTER 局限。
//
// 流程整体应包在迁移事务（*sql.Tx）内执行：
//
//	CREATE 新表（createNewSQL 已含完整新表定义，表名为 tmpName）
//	→ INSERT INTO tmpName (cols) SELECT cols FROM oldName
//	→ DROP TABLE oldName
//	→ ALTER TABLE tmpName RENAME TO oldName
//
// 参数：
//   - tx：迁移事务。
//   - oldName：现有表名（重建后保持此名）。
//   - tmpName：临时新表名（须与 createNewSQL 中的表名一致）。
//   - createNewSQL：创建临时新表的完整 SQL。
//   - copyColumns：从旧表拷贝到新表的列名（按相同顺序映射）。为空表示不拷数据。
func rebuildTable(ctx context.Context, tx *sql.Tx, oldName, tmpName, createNewSQL string, copyColumns []string) error {
	if _, err := tx.ExecContext(ctx, createNewSQL); err != nil {
		return fmt.Errorf("表重建：创建临时表 %s 失败: %w", tmpName, err)
	}
	if len(copyColumns) > 0 {
		cols := strings.Join(copyColumns, ", ")
		copySQL := fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s", tmpName, cols, cols, oldName)
		if _, err := tx.ExecContext(ctx, copySQL); err != nil {
			return fmt.Errorf("表重建：从 %s 拷贝数据到 %s 失败: %w", oldName, tmpName, err)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", oldName)); err != nil {
		return fmt.Errorf("表重建：删除旧表 %s 失败: %w", oldName, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tmpName, oldName)); err != nil {
		return fmt.Errorf("表重建：重命名 %s 为 %s 失败: %w", tmpName, oldName, err)
	}
	return nil
}
