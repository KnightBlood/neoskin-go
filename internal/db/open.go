package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// Config 是对 DB_* / DATABASE_URL 的解析结果。
type Config struct {
	Connection string
	Host       string
	Port       string
	Database   string
	Username   string
	Password   string
	Encrypt    bool
	Trust      bool
	URL        string
}

func ConfigFromEnv(getenv func(string) string) Config {
	cfg := Config{
		Connection: envOr(getenv, "DB_CONNECTION", "mysql"),
		Host:       envOr(getenv, "DB_HOST", "127.0.0.1"),
		Port:       getenv("DB_PORT"),
		Database:   envOr(getenv, "DB_DATABASE", "blessingskin"),
		Username:   envOr(getenv, "DB_USERNAME", "root"),
		Password:   getenv("DB_PASSWORD"),
		Encrypt:    envBool(getenv, "DB_ENCRYPT", true),
		Trust:      envBool(getenv, "DB_TRUST_SERVER_CERTIFICATE", false),
		URL:        getenv("DATABASE_URL"),
	}
	cfg.applyURL()

	return cfg
}

func envOr(getenv func(string) string, key, fallback string) string {
	if value := getenv(key); value != "" {
		return value
	}

	return fallback
}

func envBool(getenv func(string) string, key string, fallback bool) bool {
	value := strings.ToLower(strings.Trim(getenv(key), "()"))
	switch value {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off", "null", "empty":
		return false
	}

	return fallback
}

func (c *Config) applyURL() {
	if c.URL == "" {
		return
	}

	parsed, err := url.Parse(c.URL)
	if err != nil || parsed.Host == "" {
		return
	}

	switch strings.ToLower(parsed.Scheme) {
	case "mysql":
		c.Connection = "mysql"
	case "mariadb":
		c.Connection = "mariadb"
	case "postgres", "postgresql", "pgsql":
		c.Connection = "pgsql"
	case "sqlserver", "mssql", "sqlsrv":
		c.Connection = "sqlsrv"
	case "sqlite", "sqlite3":
		c.Connection = "sqlite"
		c.Database = strings.TrimPrefix(parsed.Path, "/")
		return
	}

	c.Host = parsed.Hostname()
	if port := parsed.Port(); port != "" {
		c.Port = port
	}
	if name := strings.TrimPrefix(parsed.Path, "/"); name != "" {
		c.Database = name
	}
	if parsed.User != nil {
		c.Username = parsed.User.Username()
		if password, ok := parsed.User.Password(); ok {
			c.Password = password
		}
	}
}

func defaultPort(connection, port string) string {
	if port != "" {
		return port
	}

	switch connection {
	case "mysql", "mariadb":
		return "3306"
	case "pgsql":
		return "5432"
	case "sqlsrv":
		return "1433"
	}

	return ""
}

// OpenFromEnv 打开连接并返回方言。
func OpenFromEnv(getenv func(string) string) (*sql.DB, *Dialect, error) {
	return Open(ConfigFromEnv(getenv))
}

func Open(cfg Config) (*sql.DB, *Dialect, error) {
	dialect := NewDialect(cfg.Connection)
	var driverName, dsn string

	switch dialect.Name {
	case "sqlite":
		path := cfg.Database
		if path == "" {
			path = filepath.Join("database", "database.sqlite")
		}
		if path != ":memory:" {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, nil, err
			}
		}
		driverName, dsn = "sqlite3", path
	case "mysql", "mariadb":
		mysqlCfg := mysqlDriver.NewConfig()
		mysqlCfg.User = cfg.Username
		mysqlCfg.Passwd = cfg.Password
		mysqlCfg.Net = "tcp"
		mysqlCfg.Addr = net.JoinHostPort(cfg.Host, defaultPort(dialect.Name, cfg.Port))
		mysqlCfg.DBName = cfg.Database
		mysqlCfg.ParseTime = true
		mysqlCfg.Loc = time.Local
		mysqlCfg.Params = map[string]string{"charset": "utf8mb4"}
		driverName, dsn = "mysql", mysqlCfg.FormatDSN()
	case "pgsql":
		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(cfg.Username, cfg.Password),
			Host:   net.JoinHostPort(cfg.Host, defaultPort(dialect.Name, cfg.Port)),
			Path:   "/" + cfg.Database,
		}
		query := u.Query()
		query.Set("sslmode", "prefer")
		query.Set("TimeZone", "Asia/Shanghai")
		u.RawQuery = query.Encode()
		driverName, dsn = "pgx", u.String()
	case "sqlsrv":
		u := &url.URL{
			Scheme: "sqlserver",
			User:   url.UserPassword(cfg.Username, cfg.Password),
			Host:   net.JoinHostPort(cfg.Host, defaultPort(dialect.Name, cfg.Port)),
		}
		query := u.Query()
		query.Set("database", cfg.Database)
		if cfg.Encrypt {
			query.Set("encrypt", "true")
		} else {
			query.Set("encrypt", "disable")
		}
		query.Set("TrustServerCertificate", strconv.FormatBool(cfg.Trust))
		u.RawQuery = query.Encode()
		driverName, dsn = "sqlserver", u.String()
	default:
		return nil, nil, fmt.Errorf("unsupported DB_CONNECTION: %s", cfg.Connection)
	}

	conn, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, nil, err
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	return conn, dialect, nil
}
