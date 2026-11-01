package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Dialect 生成五种数据库的建表 SQL（与 .NET SchemaInstaller 对齐）。
type Dialect struct {
	Name string
}

func NewDialect(name string) *Dialect {
	switch strings.ToLower(name) {
	case "postgres", "postgresql":
		name = "pgsql"
	case "sqlserver", "mssql":
		name = "sqlsrv"
	}

	return &Dialect{Name: strings.ToLower(name)}
}

func (d *Dialect) Quote(identifier string) string {
	switch d.Name {
	case "mysql", "mariadb":
		return "`" + identifier + "`"
	case "sqlsrv":
		return "[" + identifier + "]"
	default:
		return `"` + identifier + `"`
	}
}

func (d *Dialect) SupportsColumnPosition() bool {
	return d.Name == "mysql" || d.Name == "mariadb"
}

func (d *Dialect) Varchar(length int) string {
	switch d.Name {
	case "sqlsrv":
		return fmt.Sprintf("NVARCHAR(%d)", length)
	default:
		return fmt.Sprintf("VARCHAR(%d)", length)
	}
}

func (d *Dialect) Text() string {
	if d.Name == "sqlsrv" {
		return "NVARCHAR(MAX)"
	}

	return "TEXT"
}

func (d *Dialect) LongText() string {
	if d.Name == "mysql" || d.Name == "mariadb" {
		return "LONGTEXT"
	}

	return d.Text()
}

func (d *Dialect) IntType() string {
	switch d.Name {
	case "pgsql":
		return "INTEGER"
	case "sqlite":
		return "INTEGER"
	default:
		return "INT"
	}
}

func (d *Dialect) UnsignedInt() string {
	if d.Name == "mysql" || d.Name == "mariadb" {
		return "INT UNSIGNED"
	}

	return d.IntType()
}

func (d *Dialect) BigInt() string {
	if d.Name == "sqlite" {
		return "INTEGER"
	}

	return "BIGINT"
}

func (d *Dialect) UnsignedBigInt() string {
	if d.Name == "mysql" || d.Name == "mariadb" {
		return "BIGINT UNSIGNED"
	}

	return d.BigInt()
}

func (d *Dialect) TinyInt() string {
	switch d.Name {
	case "pgsql":
		return "SMALLINT"
	case "sqlite":
		return "INTEGER"
	default:
		return "TINYINT"
	}
}

func (d *Dialect) UnsignedTinyInt() string {
	return d.TinyInt()
}

func (d *Dialect) DateTime() string {
	switch d.Name {
	case "pgsql":
		return "TIMESTAMP(0) WITHOUT TIME ZONE"
	case "sqlsrv":
		return "DATETIME"
	case "sqlite":
		return "DATETIME"
	default:
		return "DATETIME"
	}
}

func (d *Dialect) Timestamp() string {
	if d.Name == "mysql" || d.Name == "mariadb" {
		return "TIMESTAMP"
	}

	return d.DateTime()
}

func (d *Dialect) Bool() string {
	switch d.Name {
	case "pgsql":
		return "BOOLEAN"
	case "sqlsrv":
		return "BIT"
	case "sqlite":
		return "INTEGER"
	default:
		return "TINYINT(1)"
	}
}

func (d *Dialect) BoolLiteral(value bool) string {
	if d.Name == "pgsql" {
		if value {
			return "TRUE"
		}
		return "FALSE"
	}

	if value {
		return "1"
	}

	return "0"
}

func (d *Dialect) AutoIncrementPK(column string, big bool) string {
	quoted := d.Quote(column)
	switch d.Name {
	case "sqlite":
		return quoted + " INTEGER PRIMARY KEY AUTOINCREMENT"
	case "mysql", "mariadb":
		if big {
			return quoted + " BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY"
		}
		return quoted + " INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY"
	case "pgsql":
		if big {
			return quoted + " BIGSERIAL NOT NULL PRIMARY KEY"
		}
		return quoted + " SERIAL NOT NULL PRIMARY KEY"
	case "sqlsrv":
		if big {
			return quoted + " BIGINT NOT NULL IDENTITY PRIMARY KEY"
		}
		return quoted + " INT NOT NULL IDENTITY PRIMARY KEY"
	}

	return quoted
}

func (d *Dialect) TableSuffix() string {
	if d.Name == "mysql" || d.Name == "mariadb" {
		return " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci"
	}

	return ""
}

// Column 是一列的定义（已包含 NOT NULL / DEFAULT）。
type Column struct {
	Definition string
	Primary    string // 非空表示字符串主键名（PG/SQL Server 需单独 ALTER）
}

type Index struct {
	Name    string
	Columns []string
	Unique  bool
}

type Table struct {
	Name    string
	Columns []Column
	Indexes []Index
}

// Tables 返回 16 张业务表的定义（migrations 由 ensureMigrationsTable 创建）。
func (d *Dialect) Tables() []Table {
	return []Table{
		d.usersTable(),
		d.playersTable(),
		d.texturesTable(),
		d.optionsTable(),
		d.userClosetTable(),
		d.reportsTable(),
		d.notificationsTable(),
		d.jobsTable(),
		d.languageLinesTable(),
		d.scopesTable(),
		d.oauthAuthCodesTable(),
		d.oauthAccessTokensTable(),
		d.oauthRefreshTokensTable(),
		d.oauthClientsTable(),
		d.oauthPersonalAccessClientsTable(),
		d.communicationsTable(),
	}
}

func (d *Dialect) usersTable() Table {
	q := d.Quote
	positioned := d.SupportsColumnPosition()
	columns := []Column{
		{Definition: d.AutoIncrementPK("uid", false)},
		{Definition: q("email") + " " + d.Varchar(100) + " NOT NULL"},
		{Definition: q("nickname") + " " + d.Varchar(50) + " NOT NULL DEFAULT ''"},
	}
	if positioned {
		columns = append(columns, Column{Definition: q("locale") + " " + d.Varchar(255) + " NULL"})
	}
	columns = append(columns,
		Column{Definition: q("score") + " " + d.IntType() + " NOT NULL"},
		Column{Definition: q("avatar") + " " + d.IntType() + " NOT NULL DEFAULT 0"},
		Column{Definition: q("password") + " " + d.Varchar(255) + " NOT NULL"},
		Column{Definition: q("ip") + " " + d.Varchar(45) + " NOT NULL"},
	)
	if positioned {
		columns = append(columns, Column{Definition: q("is_dark_mode") + " " + d.Bool() + " NOT NULL DEFAULT " + d.BoolLiteral(false)})
	}
	columns = append(columns,
		Column{Definition: q("permission") + " " + d.IntType() + " NOT NULL DEFAULT 0"},
		Column{Definition: q("last_sign_at") + " " + d.DateTime() + " NOT NULL"},
		Column{Definition: q("register_at") + " " + d.DateTime() + " NOT NULL"},
		Column{Definition: q("verified") + " " + d.Bool() + " NOT NULL DEFAULT " + d.BoolLiteral(false)},
		Column{Definition: q("verification_token") + " " + d.Varchar(255) + " NOT NULL DEFAULT ''"},
		Column{Definition: q("remember_token") + " " + d.Varchar(100) + " NULL"},
	)
	if !positioned {
		columns = append(columns,
			Column{Definition: q("locale") + " " + d.Varchar(255) + " NULL"},
			Column{Definition: q("is_dark_mode") + " " + d.Bool() + " NOT NULL DEFAULT " + d.BoolLiteral(false)},
		)
	}

	return Table{Name: "users", Columns: columns}
}

func (d *Dialect) playersTable() Table {
	q := d.Quote
	return Table{Name: "players", Columns: []Column{
		{Definition: d.AutoIncrementPK("pid", false)},
		{Definition: q("uid") + " " + d.IntType() + " NOT NULL"},
		{Definition: q("name") + " " + d.Varchar(50) + " NOT NULL"},
		{Definition: q("tid_cape") + " " + d.IntType() + " NOT NULL DEFAULT 0"},
		{Definition: q("last_modified") + " " + d.DateTime() + " NOT NULL"},
		{Definition: q("tid_skin") + " " + d.IntType() + " NOT NULL DEFAULT -1"},
	}}
}

func (d *Dialect) texturesTable() Table {
	q := d.Quote
	return Table{Name: "textures", Columns: []Column{
		{Definition: d.AutoIncrementPK("tid", false)},
		{Definition: q("name") + " " + d.Varchar(50) + " NOT NULL"},
		{Definition: q("type") + " " + d.Varchar(10) + " NOT NULL"},
		{Definition: q("hash") + " " + d.Varchar(64) + " NOT NULL"},
		{Definition: q("size") + " " + d.IntType() + " NOT NULL"},
		{Definition: q("uploader") + " " + d.IntType() + " NOT NULL"},
		{Definition: q("public") + " " + d.TinyInt() + " NOT NULL"},
		{Definition: q("upload_at") + " " + d.DateTime() + " NOT NULL"},
		{Definition: q("likes") + " " + d.UnsignedInt() + " NOT NULL DEFAULT 0"},
	}}
}

func (d *Dialect) optionsTable() Table {
	q := d.Quote
	return Table{Name: "options", Columns: []Column{
		{Definition: d.AutoIncrementPK("id", false)},
		{Definition: q("option_name") + " " + d.Varchar(50) + " NOT NULL"},
		{Definition: q("option_value") + " " + d.LongText() + " NOT NULL"},
	}}
}

func (d *Dialect) userClosetTable() Table {
	q := d.Quote
	return Table{Name: "user_closet", Columns: []Column{
		{Definition: q("user_uid") + " " + d.IntType() + " NOT NULL"},
		{Definition: q("texture_tid") + " " + d.IntType() + " NOT NULL"},
		{Definition: q("item_name") + " " + d.Text() + " NULL"},
	}}
}

func (d *Dialect) reportsTable() Table {
	q := d.Quote
	return Table{Name: "reports", Columns: []Column{
		{Definition: d.AutoIncrementPK("id", false)},
		{Definition: q("tid") + " " + d.IntType() + " NOT NULL"},
		{Definition: q("uploader") + " " + d.IntType() + " NOT NULL"},
		{Definition: q("reporter") + " " + d.IntType() + " NOT NULL"},
		{Definition: q("reason") + " " + d.LongText() + " NOT NULL"},
		{Definition: q("status") + " " + d.IntType() + " NOT NULL"},
		{Definition: q("report_at") + " " + d.DateTime() + " NOT NULL"},
	}}
}

func (d *Dialect) notificationsTable() Table {
	q := d.Quote
	return Table{
		Name: "notifications",
		Columns: []Column{
			{Definition: q("id") + " " + d.Varchar(36) + " NOT NULL", Primary: "id"},
			{Definition: q("type") + " " + d.Varchar(255) + " NOT NULL"},
			{Definition: q("notifiable_type") + " " + d.Varchar(255) + " NOT NULL"},
			{Definition: q("notifiable_id") + " " + d.UnsignedBigInt() + " NOT NULL"},
			{Definition: q("data") + " " + d.Text() + " NOT NULL"},
			{Definition: q("read_at") + " " + d.Timestamp() + " NULL"},
			{Definition: q("created_at") + " " + d.Timestamp() + " NULL"},
			{Definition: q("updated_at") + " " + d.Timestamp() + " NULL"},
		},
		Indexes: []Index{{Name: "notifications_notifiable_type_notifiable_id_index",
			Columns: []string{"notifiable_type", "notifiable_id"}}},
	}
}

func (d *Dialect) jobsTable() Table {
	q := d.Quote
	return Table{
		Name: "jobs",
		Columns: []Column{
			{Definition: d.AutoIncrementPK("id", true)},
			{Definition: q("queue") + " " + d.Varchar(255) + " NOT NULL"},
			{Definition: q("payload") + " " + d.LongText() + " NOT NULL"},
			{Definition: q("attempts") + " " + d.UnsignedTinyInt() + " NOT NULL"},
			{Definition: q("reserved_at") + " " + d.UnsignedInt() + " NULL"},
			{Definition: q("available_at") + " " + d.UnsignedInt() + " NOT NULL"},
			{Definition: q("created_at") + " " + d.UnsignedInt() + " NOT NULL"},
		},
		Indexes: []Index{{Name: "jobs_queue_index", Columns: []string{"queue"}}},
	}
}

func (d *Dialect) languageLinesTable() Table {
	q := d.Quote
	return Table{
		Name: "language_lines",
		Columns: []Column{
			{Definition: d.AutoIncrementPK("id", false)},
			{Definition: q("group") + " " + d.Varchar(255) + " NOT NULL"},
			{Definition: q("key") + " " + d.Varchar(255) + " NOT NULL"},
			{Definition: q("text") + " " + d.Text() + " NOT NULL"},
			{Definition: q("created_at") + " " + d.Timestamp() + " NULL"},
			{Definition: q("updated_at") + " " + d.Timestamp() + " NULL"},
		},
		Indexes: []Index{{Name: "language_lines_group_index", Columns: []string{"group"}}},
	}
}

func (d *Dialect) scopesTable() Table {
	q := d.Quote
	return Table{
		Name: "scopes",
		Columns: []Column{
			{Definition: d.AutoIncrementPK("id", false)},
			{Definition: q("name") + " " + d.Varchar(255) + " NOT NULL"},
			{Definition: q("description") + " " + d.Varchar(255) + " NOT NULL"},
		},
		Indexes: []Index{{Name: "scopes_name_unique", Columns: []string{"name"}, Unique: true}},
	}
}

func (d *Dialect) oauthAuthCodesTable() Table {
	q := d.Quote
	return Table{
		Name: "oauth_auth_codes",
		Columns: []Column{
			{Definition: q("id") + " " + d.Varchar(100) + " NOT NULL", Primary: "id"},
			{Definition: q("user_id") + " " + d.UnsignedBigInt() + " NOT NULL"},
			{Definition: q("client_id") + " " + d.UnsignedBigInt() + " NOT NULL"},
			{Definition: q("scopes") + " " + d.Text() + " NULL"},
			{Definition: q("revoked") + " " + d.Bool() + " NOT NULL"},
			{Definition: q("expires_at") + " " + d.DateTime() + " NULL"},
		},
		Indexes: []Index{{Name: "oauth_auth_codes_user_id_index", Columns: []string{"user_id"}}},
	}
}

func (d *Dialect) oauthAccessTokensTable() Table {
	q := d.Quote
	return Table{
		Name: "oauth_access_tokens",
		Columns: []Column{
			{Definition: q("id") + " " + d.Varchar(100) + " NOT NULL", Primary: "id"},
			{Definition: q("user_id") + " " + d.UnsignedBigInt() + " NULL"},
			{Definition: q("client_id") + " " + d.UnsignedBigInt() + " NOT NULL"},
			{Definition: q("name") + " " + d.Varchar(255) + " NULL"},
			{Definition: q("scopes") + " " + d.Text() + " NULL"},
			{Definition: q("revoked") + " " + d.Bool() + " NOT NULL"},
			{Definition: q("created_at") + " " + d.Timestamp() + " NULL"},
			{Definition: q("updated_at") + " " + d.Timestamp() + " NULL"},
			{Definition: q("expires_at") + " " + d.DateTime() + " NULL"},
		},
		Indexes: []Index{{Name: "oauth_access_tokens_user_id_index", Columns: []string{"user_id"}}},
	}
}

func (d *Dialect) oauthRefreshTokensTable() Table {
	q := d.Quote
	return Table{
		Name: "oauth_refresh_tokens",
		Columns: []Column{
			{Definition: q("id") + " " + d.Varchar(100) + " NOT NULL", Primary: "id"},
			{Definition: q("access_token_id") + " " + d.Varchar(100) + " NOT NULL"},
			{Definition: q("revoked") + " " + d.Bool() + " NOT NULL"},
			{Definition: q("expires_at") + " " + d.DateTime() + " NULL"},
		},
		Indexes: []Index{{Name: "oauth_refresh_tokens_access_token_id_index",
			Columns: []string{"access_token_id"}}},
	}
}

func (d *Dialect) oauthClientsTable() Table {
	q := d.Quote
	return Table{
		Name: "oauth_clients",
		Columns: []Column{
			{Definition: d.AutoIncrementPK("id", true)},
			{Definition: q("user_id") + " " + d.UnsignedBigInt() + " NULL"},
			{Definition: q("name") + " " + d.Varchar(255) + " NOT NULL"},
			{Definition: q("secret") + " " + d.Varchar(100) + " NULL"},
			{Definition: q("provider") + " " + d.Varchar(255) + " NULL"},
			{Definition: q("redirect") + " " + d.Text() + " NOT NULL"},
			{Definition: q("personal_access_client") + " " + d.Bool() + " NOT NULL"},
			{Definition: q("password_client") + " " + d.Bool() + " NOT NULL"},
			{Definition: q("revoked") + " " + d.Bool() + " NOT NULL"},
			{Definition: q("created_at") + " " + d.Timestamp() + " NULL"},
			{Definition: q("updated_at") + " " + d.Timestamp() + " NULL"},
		},
		Indexes: []Index{{Name: "oauth_clients_user_id_index", Columns: []string{"user_id"}}},
	}
}

func (d *Dialect) oauthPersonalAccessClientsTable() Table {
	q := d.Quote
	return Table{Name: "oauth_personal_access_clients", Columns: []Column{
		{Definition: d.AutoIncrementPK("id", true)},
		{Definition: q("client_id") + " " + d.UnsignedBigInt() + " NOT NULL"},
		{Definition: q("created_at") + " " + d.Timestamp() + " NULL"},
		{Definition: q("updated_at") + " " + d.Timestamp() + " NULL"},
	}}
}

func (d *Dialect) communicationsTable() Table {
	q := d.Quote
	return Table{
		Name: "communications",
		Columns: []Column{
			{Definition: d.AutoIncrementPK("id", true)},
			{Definition: q("author_id") + " " + d.UnsignedInt() + " NULL"},
			{Definition: q("legacy_key") + " " + d.Varchar(100) + " NULL"},
			{Definition: q("delivery_token") + " " + d.Varchar(32) + " NULL"},
			{Definition: q("title") + " " + d.Varchar(100) + " NOT NULL"},
			{Definition: q("kind") + " " + d.Varchar(16) + " NOT NULL DEFAULT 'announcement'"},
			{Definition: q("status") + " " + d.Varchar(16) + " NOT NULL DEFAULT 'draft'"},
			{Definition: q("locale") + " " + d.Varchar(20) + " NOT NULL DEFAULT ''"},
			{Definition: q("editor") + " " + d.Varchar(16) + " NOT NULL DEFAULT 'blocks'"},
			{Definition: q("source") + " " + d.LongText() + " NULL"},
			{Definition: q("blocks") + " " + d.LongText() + " NULL"},
			{Definition: q("audience") + " " + d.Text() + " NULL"},
			{Definition: q("published_at") + " " + d.Timestamp() + " NULL"},
			{Definition: q("created_at") + " " + d.Timestamp() + " NULL"},
			{Definition: q("updated_at") + " " + d.Timestamp() + " NULL"},
		},
		Indexes: []Index{
			{Name: "communications_legacy_key_unique", Columns: []string{"legacy_key"}, Unique: true},
			{Name: "communications_status_index", Columns: []string{"status"}},
		},
	}
}

// CreateSQL 生成建表语句（含字符串主键的独立 ALTER）。
func (d *Dialect) CreateSQL(table Table, prefix string) []string {
	name := prefix + table.Name
	var parts []string
	var primary []string
	for _, column := range table.Columns {
		parts = append(parts, column.Definition)
		if column.Primary != "" {
			primary = append(primary, d.Quote(column.Primary))
		}
	}

	inlinePrimary := d.Name == "mysql" || d.Name == "mariadb" || d.Name == "sqlite"
	if inlinePrimary && len(primary) > 0 {
		parts = append(parts, "PRIMARY KEY ("+strings.Join(primary, ", ")+")")
	}

	statement := "CREATE TABLE " + d.Quote(name) + " (\n  " +
		strings.Join(parts, ",\n  ") + "\n)" + d.TableSuffix()
	statements := []string{statement}

	if !inlinePrimary && len(primary) > 0 {
		statements = append(statements, "ALTER TABLE "+d.Quote(name)+
			" ADD PRIMARY KEY ("+strings.Join(primary, ", ")+")")
	}

	for _, index := range table.Indexes {
		kind := "CREATE INDEX "
		if index.Unique {
			kind = "CREATE UNIQUE INDEX "
		}
		columns := make([]string, len(index.Columns))
		for i, column := range index.Columns {
			columns[i] = d.Quote(column)
		}
		statements = append(statements, kind+d.Quote(prefix+index.Name)+" ON "+
			d.Quote(name)+" ("+strings.Join(columns, ", ")+")")
	}

	return statements
}

// Installer 按方言建表；幂等。
type Installer struct {
	DB      *sql.DB
	Dialect *Dialect
	Prefix  string
}

func NewInstaller(db *sql.DB, dialect *Dialect) *Installer {
	return &Installer{DB: db, Dialect: dialect}
}

func (i *Installer) Install(ctx context.Context) (int, error) {
	if err := i.ensureMigrationsTable(ctx); err != nil {
		return 0, err
	}

	created := 0
	for _, table := range i.Dialect.Tables() {
		exists, err := i.HasTable(ctx, table.Name)
		if err != nil {
			return created, err
		}
		if exists {
			continue
		}

		for _, statement := range i.Dialect.CreateSQL(table, i.Prefix) {
			if _, err := i.DB.ExecContext(ctx, statement); err != nil {
				return created, fmt.Errorf("create %s: %w", table.Name, err)
			}
		}
		created++
	}

	return created, nil
}

func (i *Installer) ensureMigrationsTable(ctx context.Context) error {
	exists, err := i.HasTable(ctx, "migrations")
	if err != nil || exists {
		return err
	}

	table := Table{Name: "migrations", Columns: []Column{
		{Definition: i.Dialect.AutoIncrementPK("id", false)},
		{Definition: i.Dialect.Quote("migration") + " " + i.Dialect.Varchar(255) + " NOT NULL"},
		{Definition: i.Dialect.Quote("batch") + " " + i.Dialect.IntType() + " NOT NULL"},
	}}

	for _, statement := range i.Dialect.CreateSQL(table, i.Prefix) {
		if _, err := i.DB.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}

// DropAll 删除当前库的所有表（仅用于矩阵测试/本地重置）。
func (i *Installer) DropAll(ctx context.Context) error {
	switch i.Dialect.Name {
	case "pgsql":
		if _, err := i.DB.ExecContext(ctx, "DROP SCHEMA public CASCADE"); err != nil {
			return err
		}
		_, err := i.DB.ExecContext(ctx, "CREATE SCHEMA public")
		return err
	}

	var query string
	switch i.Dialect.Name {
	case "sqlite":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	case "mysql", "mariadb":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()"
		if _, err := i.DB.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
			return err
		}
	case "sqlsrv":
		query = "SELECT name FROM sys.objects WHERE type='U'"
	default:
		return fmt.Errorf("unsupported dialect: %s", i.Dialect.Name)
	}

	rows, err := i.DB.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return err
		}
		names = append(names, name)
	}
	_ = rows.Close()

	for _, name := range names {
		if _, err := i.DB.ExecContext(ctx, "DROP TABLE "+i.Dialect.Quote(name)); err != nil {
			return err
		}
	}

	return nil
}

func (i *Installer) HasTable(ctx context.Context, name string) (bool, error) {
	name = i.Prefix + name
	var query string
	switch i.Dialect.Name {
	case "sqlite":
		query = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
	case "mysql", "mariadb":
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name=?"
	case "pgsql":
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name=$1"
	case "sqlsrv":
		query = "SELECT COUNT(*) FROM sys.objects WHERE type='U' AND name=@p1"
	default:
		return false, fmt.Errorf("unsupported dialect: %s", i.Dialect.Name)
	}

	var count int
	if err := i.DB.QueryRowContext(ctx, query, name).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}
