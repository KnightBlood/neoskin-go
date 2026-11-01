package config

import (
	"fmt"
	"net/url"
	"strings"

	contractsconfig "github.com/goravel/framework/contracts/config"
	"github.com/goravel/framework/contracts/database/driver"
	mysqlfacades "github.com/goravel/mysql/facades"
	postgresfacades "github.com/goravel/postgres/facades"
	sqlitefacades "github.com/goravel/sqlite/facades"
	sqlserverfacades "github.com/goravel/sqlserver/facades"

	"neoskin/app/facades"
)

func init() {
	config := facades.Config()

	defaultConnection := envString(config, "DB_CONNECTION", "mysql")
	host := envString(config, "DB_HOST", "127.0.0.1")
	port := envString(config, "DB_PORT", "")
	database := envString(config, "DB_DATABASE", "blessingskin")
	username := envString(config, "DB_USERNAME", "root")
	password := envString(config, "DB_PASSWORD", "")
	prefix := envString(config, "DB_PREFIX", "")

	config.Add("database", map[string]any{
		"default": defaultConnection,
		"connections": map[string]any{
			"mysql": map[string]any{
				"host":     host,
				"port":     defaultPort(port, "3306"),
				"database": database,
				"username": username,
				"password": password,
				"charset":  "utf8mb4",
				"loc":      "Local",
				"prefix":   prefix,
				"singular": false,
				"via": func() (driver.Driver, error) {
					return mysqlfacades.Mysql("mysql")
				},
			},
			"mariadb": map[string]any{
				"host":     host,
				"port":     defaultPort(port, "3306"),
				"database": database,
				"username": username,
				"password": password,
				"charset":  "utf8mb4",
				"loc":      "Local",
				"prefix":   prefix,
				"singular": false,
				"via": func() (driver.Driver, error) {
					return mysqlfacades.Mysql("mariadb")
				},
			},
			"pgsql": map[string]any{
				"host":     host,
				"port":     defaultPort(port, "5432"),
				"database": database,
				"username": username,
				"password": password,
				"sslmode":  "prefer",
				"schema":   envString(config, "DB_SCHEMA", "public"),
				"prefix":   prefix,
				"singular": false,
				"via": func() (driver.Driver, error) {
					return postgresfacades.Postgres("pgsql")
				},
			},
			"sqlite": map[string]any{
				"database": sqliteDatabase(database),
				"prefix":   prefix,
				"singular": false,
				"via": func() (driver.Driver, error) {
					return sqlitefacades.Sqlite("sqlite")
				},
			},
			"sqlsrv": map[string]any{
				"host":     host,
				"port":     defaultPort(port, "1433"),
				"database": database,
				"username": username,
				"password": password,
				"charset":  "utf8",
				"prefix":   prefix,
				"singular": false,
				"dsn":      sqlServerDSN(host, defaultPort(port, "1433"), database, username, password, config),
				"via": func() (driver.Driver, error) {
					return sqlserverfacades.Sqlserver("sqlsrv")
				},
			},
		},
		"pool": map[string]any{
			"max_idle_conns":    10,
			"max_open_conns":    100,
			"conn_max_idletime": 3600,
			"conn_max_lifetime": 3600,
		},
		"slow_threshold": 200,
		"migrations": map[string]any{
			"table": "migrations",
		},
	})

	// 与 Laravel 一致：DATABASE_URL 优先级最高，覆盖 DB_*。
	if raw := envString(config, "DATABASE_URL", ""); raw != "" {
		applyDatabaseURL(raw, config)
	}
}

func envString(config contractsconfig.Config, key, fallback string) string {
	value := config.Env(key, fallback)
	if value == nil {
		return fallback
	}

	return fmt.Sprint(value)
}

func envBool(config contractsconfig.Config, key string, fallback bool) bool {
	value := config.Env(key, fallback)
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.Trim(typed, "()")) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off", "null", "empty":
			return false
		}
	}

	return fallback
}

func defaultPort(port, fallback string) string {
	if strings.TrimSpace(port) != "" {
		return port
	}

	return fallback
}

func sqliteDatabase(database string) string {
	if strings.TrimSpace(database) == "" || database == "blessingskin" {
		return "database/database.sqlite"
	}

	return database
}

func sqlServerDSN(host, port, database, username, password string, config contractsconfig.Config) string {
	encrypt := "true"
	if !envBool(config, "DB_ENCRYPT", true) {
		encrypt = "disable"
	}

	trust := envBool(config, "DB_TRUST_SERVER_CERTIFICATE", false)

	return fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s&encrypt=%s&TrustServerCertificate=%t",
		url.QueryEscape(username), url.QueryEscape(password), host, port, database, encrypt, trust)
}

// applyDatabaseURL 解析 mysql://user:pass@host:port/db 并覆盖连接配置。
func applyDatabaseURL(raw string, config contractsconfig.Config) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return
	}

	connection := strings.ToLower(parsed.Scheme)
	if connection == "postgres" || connection == "postgresql" {
		connection = "pgsql"
	}
	if connection == "sqlserver" || connection == "mssql" {
		connection = "sqlsrv"
	}

	config.Add("database.default", connection)
	config.Add("database.connections."+connection+".host", parsed.Hostname())
	if parsed.Port() != "" {
		config.Add("database.connections."+connection+".port", parsed.Port())
	}
	if name := strings.TrimPrefix(parsed.Path, "/"); name != "" {
		config.Add("database.connections."+connection+".database", name)
	}
	if parsed.User != nil {
		config.Add("database.connections."+connection+".username", parsed.User.Username())
		if password, ok := parsed.User.Password(); ok {
			config.Add("database.connections."+connection+".password", password)
		}
	}
}
