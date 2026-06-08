# 数据库规范（Database Guidelines）

> proxy-hub 的 SQLite 使用约定。由 M1 确立；M2+ 新增业务表与查询时遵循。

---

## 概览

- 引擎：**内嵌 SQLite，纯 Go 驱动 `modernc.org/sqlite`**（注册驱动名 `sqlite`），保持 `CGO_ENABLED=0`。
- DB 访问：M1 用手写 `database/sql`；自 M2 起按 OQ-2 决定引入 `sqlc` 生成类型化查询。**不使用 GORM/AutoMigrate。**
- 金额：用 `shopspring/decimal`、以 TEXT 存储（M3 引入），**绝不用 float**。
- 并发：**多读单写**——见下。

---

## 并发模型（核心，勿改）

SQLite 只允许单写者。`internal/store/db.go` 用**两个 `*sql.DB` 句柄指向同一文件**落地：

```go
read.SetMaxOpenConns(max(4, GOMAXPROCS)) // 读连接池
write.SetMaxOpenConns(1)                 // 写串行化为单连接，天然契合 SQLite 单写者
```

- 所有 `SELECT` 走 `store.Read()`；所有 `INSERT/UPDATE/DELETE/DDL` 走 `store.Write()`。
- 高频写（M3 统计采集器）在 `write` 句柄上做**批量**，不要每事件一事务。
- 不要再引入第三个写句柄或手写 `Mutex<Connection>`（参考项目 cc-switch 的争用教训）。

DSN（modernc 用 `_pragma=` 查询参数，逐连接生效）——见 `buildDSN`：

```
file:<dataDir>/proxy-hub.db?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)
```

- `WAL`：读写并发；`synchronous=NORMAL`：WAL 下安全且更快；`busy_timeout=30000`：兜底偶发锁等待；`foreign_keys=ON`：强制外键。

---

## 迁移（Migrations）

框架在 `internal/store/migrate.go`，由 `store.Open` 在启动时调用 `Run(write, dbPath)`：

1. 迁移文件 `internal/store/migrations/NNNN_desc.sql`，经 `//go:embed migrations/*.sql` 内嵌。
   - **注意**：embed 路径相对于 `migrate.go`，故迁移目录**必须**在 `internal/store/` 下，不能放仓库顶层（不能用 `../`）。
   - 文件名数字前缀即版本号；`loadMigrations` 校验版本**从 1 连续递增**，否则报错。
2. `meta(key TEXT PRIMARY KEY, value TEXT NOT NULL)` 存 `schema_version`。
3. 有待应用迁移时，**先做预迁移备份**：`VACUUM INTO '<db>.bak-<当前版本>'`（一致性快照，优于拷文件，避免 WAL 半态）。
4. 所有待应用迁移在**单个事务**内按序执行并更新 `schema_version`；任一失败 → 整体回滚（备份即恢复点）。
5. 改表用 `rebuildTable` helper（`CREATE 新表 → INSERT…SELECT → DROP → RENAME`，事务内），规避 SQLite `ALTER` 局限——**不要**只靠 `ALTER`/`IF NOT EXISTS`。

新增迁移：在 `migrations/` 加 `000N_xxx.sql`（N=上一版本+1），纯 SQL，幂等不是必须（版本控制保证只执行一次）。**测试断言最新版本用 `len(loadMigrations())`（已校验连续 1..N），勿硬编码具体数字**（加新迁移就会变）。

---

## sqlc 代码生成（自 M2）

`internal/store/queries/*.sql` 经 `sqlc generate` 产出类型化 Go 代码到 `internal/store/dbgen`（package `dbgen`），**提交入库**（Docker/CI 不需 sqlc 二进制）。配置 `sqlc.yaml`：`engine: sqlite`、schema 指向 `internal/store/migrations`（复用迁移作 schema 源）、queries 指向 `internal/store/queries`。**改查询后必须本地 `sqlc generate` 再提交**；校验 `git diff --exit-code internal/store/dbgen`。

### 字段名映射（务必照此命名，否则手写 dao/manager 编译失败）

sqlc 默认 initialisms **只有 `["id"]`**。列名→Go 名＝「下划线转驼峰、仅 `id` 全大写」：

| 列 / 表 | 生成的 Go 标识 | 易错点 |
|---|---|---|
| `base_url` | `BaseUrl`（**非** `BaseURL`） | URL 不在缩写表 |
| `proxy_url` | `ProxyUrl` | 同上 |
| `group_name` | `GroupName` | `group` 是 SQL 保留字，列名用 `group_name` |
| `id` | `ID` | 唯一全大写 |
| 表 `api_keys` | 结构体 `ApiKey`（**非** `APIKey`） | 单数化 + 非缩写 |
| 方法名 | 逐字取自 `-- name: Xxx` | 你写的 `CreateAPIKey` 大写被保留 |

### 类型映射

- `INTEGER NOT NULL` → `int64`（**`enabled`/`is_healthy` 是 `int64`，不是 `bool`**；dao 层用 `b2i`/`i2b` 互转）。
- `TEXT NOT NULL` → `string`；可空 `TEXT` → `sql.NullString`（dao 用 `timeToNull`/`nullToTime` 转领域 `time.Time`）。

### 调用约定

- `dbgen.New(db DBTX) *Queries`；`DBTX` 由 `*sql.DB` 与 `*sql.Tx` 同时满足。读 `New(store.Read())`；写/事务 `New(store.Write())` 或 `New(tx)`（保存渠道 = 写 channels + 同事务重建 abilities）。
- **单参**查询走位置参数（`GetChannel(ctx, id int64)`）；**≥2 参**用 `XxxParams` 结构体，且 `UpdateXxx`/`SetXxx` 的 **`ID` 放在 Params 末尾**。

> **Warning（致命坑）**：`queries/*.sql` 里的 **CJK 注释会破坏 sqlc 的 ANTLR 查询改写器**（字节偏移错算）→ `generate` 报错或生成错代码。**查询文件一律 ASCII 注释**。迁移文件走 schema 解析、CJK 风险低，但 M2 已把 `0002` 也转 ASCII 防万一。这是本项目 `.sql` 文件**豁免「注释一律中文」规则**的唯一原因（见 `AGENTS.md`）。

---

## 命名约定

- 表名/列名：小写 + 下划线（`request_logs`、`api_key_id`、`cooldown_until`）。
- 主键：单列用 `id`；派生/关系表用复合 PK（如 `abilities` 的 `(group, alias_model, channel_id)`、`channel_model_health` 的 `(channel_id, model)`）。
- 时间列：`created_at`/`updated_at`/`*_at`，存毫秒/秒整数或 RFC3339 文本（M3 统一）。
- 索引显式命名并随表迁移一起创建。

---

## 常见错误

- ❌ 用 `read` 句柄做写、或在 `write` 句柄上开大并发 → 破坏单写者模型，触发 `database is locked`。
- ❌ 把迁移目录放仓库顶层 → `go:embed` 编译失败。
- ❌ 用 float 存金额 → 漂移。用 decimal TEXT。
- ❌ 把上游 API key / admin_key 写进任何表 → 密钥只存 `data/auths/*.json`（0600）；DB 只存哈希（入站 key）。
- ❌ 在 `VACUUM INTO` 等不支持占位符的语句里拼接未转义字符串 → 用 `quoteSQLString`。
