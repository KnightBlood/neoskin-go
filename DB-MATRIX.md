# NeoSkinGo 数据库矩阵（本机 + CI）

> 跨语言共享手册（坑位/症状/新端口步骤）见工作区根目录
> `..\DATABASE-MATRIX-PLAYBOOK.md`（不随仓库提交）。本文件是 Go 端的自包含版本。

## 1. 本机数据库

`D:\WORKspace\NeoSkin\tools` 下有免安装二进制，数据在 `tools\data\`：

```powershell
..\tools\scripts\db.cmd init all
..\tools\scripts\db.cmd start all
..\tools\scripts\db.cmd status
```

| `DB_CONNECTION` | 版本 | 端口 | 账号 | 库 |
| --- | --- | --- | --- | --- |
| `mysql` | MySQL 26.10 | 3307 | blessing / 123456 | `blessing_skin` |
| `mariadb` | MariaDB 13.0.2 | 3308 | blessing / 123456 | `blessing_skin` |
| `pgsql` | PostgreSQL 18.6 | 5433 | postgres / 123456 | `blessing_skin` |
| `sqlsrv` | WiltonDB（TDS） | 1433 | wilton / 123456 | `blessing_skin`（测试自动建库） |
| `sqlite` | 文件 | — | — | 临时文件 / `:memory:` |

WiltonDB 只有本地替身：能验证 T-SQL DDL、`IDENTITY`、`OFFSET … FETCH`、自增回填；
真 SQL Server 的过滤唯一索引、系统视图、优化器/锁语义仍要 CI 的 `sqlsrv` job。

## 2. 矩阵测试契约

Go 端实现：`internal/db/schema_test.go` 的 `TestMatrix`，环境变量门控：

```go
func TestMatrix(t *testing.T) {
    dbc := os.Getenv("DB_CONNECTION")
    if dbc != "mysql" && dbc != "mariadb" && dbc != "pgsql" && dbc != "sqlsrv" {
        t.Skip("DB_CONNECTION 不是真实数据库（本机默认跳过）")
    }
    // 1) 迁移 17 张表（幂等）
    // 2) 插入/查询/删除用户
    // 3) 插入/查询/删除纹理（public 小整数 + upload_at 时间戳 + 自增主键）
    // 4) 整行 JSON 的 register_at 必须是 Y-m-d H:i:s
}
```

要求：

- 迁移必须幂等；重复跑不报错。
- 唯一键用随机值，`defer` 清理，避免污染共享库。
- 外部数据（`conformance/data`、示例插件产物）缺失时 **Skip 不要 Fail**；
  不要把 `/tmp/...` 之类本机调试路径写进测试。
- 发布产物（`go build` 的二进制）再做一次 HTTP 冒烟：
  `/skinlib/list?filter=skin&page=1`、`/auth/login`、`/api` 应为 200。

## 3. Go / GORM 易错点

- **不要用 GORM `AutoMigrate` 造正式表**：源项目有 17 张表的精确列类型/默认值/索引，
  用显式 SQL 迁移 + 方言分支，`AutoMigrate` 会按 GORM 约定改类型/加索引，破坏兼容。
- **SQL Server 保留字**：`public` 是保留字，GORM 生成的 SQL 不一定会引用它；
  手写/扫描列名时用 `[public]` 或让 GORM 的 naming/quoting 生效。
- **SQL Server 自增回填**：`SCOPE_IDENTITY()` 必须与 `INSERT` 同一批次
  （`INSERT …; SELECT SCOPE_IDENTITY()`）或使用 `OUTPUT INSERTED.[uid]`；
  单独命令返回 `NULL`（WiltonDB 可能掩盖）。
- **PostgreSQL 类型严格**：不要把 Go `string` 直接写入 `timestamp` 列：
  `pgx` 需要 `time.Time`，或在 SQL 里显式 `@p::timestamp`。
  读取 timestamp 用 `time.Time`，输出统一格式化为 `Y-m-d H:i:s`。
- **`boolean` 与 `tinyInteger` 不同**：`users.verified` 是 `boolean`
  （PG `boolean` / SQL Server `BIT`），`textures.public` 是 `tinyInteger`
  （PG `smallint` / SQL Server `TINYINT`）；参数分别用 bool 与 `0/1`。
- **整型宽度**：`INT` / `SERIAL` / `IDENTITY` 可能是 int32，`COUNT(*)` 是 int64；
  扫描到 `int64`，不要假设 `int`。
- **MySQL DSN**：`charset=utf8mb4&parseTime=true&loc=Local`；MySQL 8 的
  `caching_sha2_password` 在非 TLS 下需要 `allowNativePasswords=true`
  （或 `allowCleartextPasswords` 仅限测试）。
- **SQL Server 加密**：驱动默认 `encrypt=true`；本地 WiltonDB 用
  `encrypt=disable`（或 `TrustServerCertificate=true`），CI 与源配置保持一致。
- **SQLite 会掩盖问题**：重复打开连接、布尔/时间戳字符串写入都能过；
  必须用真实库再跑一遍。

## 4. 本机运行

```powershell
# MySQL
$env:DB_CONNECTION='mysql'; $env:DB_HOST='127.0.0.1'; $env:DB_PORT='3307'
$env:DB_DATABASE='blessing_skin'; $env:DB_USERNAME='blessing'; $env:DB_PASSWORD='123456'
go test ./internal/db -run TestMatrix -v

# MariaDB 3308 / PostgreSQL 5433（postgres）同理
# WiltonDB
$env:DB_CONNECTION='sqlsrv'; $env:DB_HOST='127.0.0.1'; $env:DB_PORT='1433'
$env:DB_DATABASE='blessing_skin'; $env:DB_USERNAME='wilton'; $env:DB_PASSWORD='123456'
$env:DB_ENCRYPT='false'; $env:DB_TRUST_SERVER_CERTIFICATE='true'
go test ./internal/db -run TestMatrix -v
```

`DATABASE_URL` 优先级高于 `DB_*`（与 PHP/Java/.NET 一致）。

## 5. CI

Go 端 CI 参考 `.NET` 仓库的 `.github/workflows/ci.yml`（5 数据库 + 发布）与
Java 的 `NeoSkinSpringBoot/.github/workflows/ci.yml`：

- 镜像：`mysql:8` / `mariadb:11` / `postgres:16` /
  `mcr.microsoft.com/mssql/server:2022-latest`；
- SQL Server：用 sqlcmd 建库/建登录，注入 `DB_ENCRYPT=false`、
  `DB_TRUST_SERVER_CERTIFICATE=true`；
- 矩阵 job 跑 `go test ./...`，另加 `go build` / `golangci-lint`。

## 6. 已知差异

- `DB_PREFIX`：源 Laravel 支持表前缀；Go 实现如果要支持，需在**所有**查询层统一加，
  不能只用于建表（Java/.NET 目前也有此限制，属于已知偏差）。
- WiltonDB 不等价于真 SQL Server，见第 1 节。

## 7. 本机实测（2026-11-01）

`TestMatrix` 已在以下四个库通过（DropAll → Install 16 表 + migrations → 用户/纹理 CRUD
→ 时间戳格式）：

| 驱动 | 端口 | 结果 |
| --- | --- | --- |
| MySQL 26.10 | 3307 | PASS |
| MariaDB 13.0.2 | 3308 | PASS |
| PostgreSQL 18.6 | 5433 | PASS |
| WiltonDB（TDS） | 1433 | PASS |

实现说明：

- 建表用 `internal/db` 的显式 SQL 安装器（`artisan neoskin:install`），
  **没有用** Goravel `migrate:fresh`：它的 SQL Server grammar 依赖
  `sys.allocation_units` / `sp_msforeachtable`，Babelfish 不支持；
  真 SQL Server 上可用，但为了五库一致，统一走 raw installer。
- `textures.public` 在 SQL Server 必须引用为 `[public]`；SQL Server 参数占位符是
  `@pN`（不是 `?`）；PostgreSQL 是 `$N`。
- `app.timezone` 设为 `Asia/Shanghai`，与 PHP 一致。
- `APP_KEY`：Goravel 要求 32 个可打印字符；PHP 的 `base64:` 是二进制密钥，
  **不能**直接塞进 Goravel 的 env 校验。后续用自研 Laravel encrypter +
  `NEOSKIN_APP_KEY` 处理 PHP 兼容，Goravel 自己的 key 只用于框架内部。
