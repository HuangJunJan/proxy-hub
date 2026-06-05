-- 0001_init：确立迁移框架。
-- M1 仅创建 meta 表用于存储 schema_version，业务表由后续里程碑的迁移新增。

CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
