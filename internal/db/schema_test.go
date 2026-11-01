package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestMatrix 对应 Java/.NET 的 DatabaseMatrixTest：
// 建表 → 用户/纹理 CRUD → 时间戳格式校验。仅当 DB_CONNECTION 指向真实库时运行。
func TestMatrix(t *testing.T) {
	connection := os.Getenv("DB_CONNECTION")
	switch connection {
	case "mysql", "mariadb", "pgsql", "sqlsrv":
	default:
		t.Skip("DB_CONNECTION 不是 mysql/mariadb/pgsql/sqlsrv")
	}

	ctx := context.Background()
	conn, dialect, err := OpenFromEnv(os.Getenv)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	installer := NewInstaller(conn, dialect)
	if err := installer.DropAll(ctx); err != nil {
		t.Fatalf("drop all: %v", err)
	}

	created, err := installer.Install(ctx)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if created != 16 {
		t.Fatalf("created=%d, want 16", created)
	}

	ok, err := installer.HasTable(ctx, "users")
	if err != nil || !ok {
		t.Fatalf("users table missing: ok=%v err=%v", ok, err)
	}

	email := fmt.Sprintf("matrix-%d@%s.local", time.Now().UnixNano(), connection)
	now := time.Now().Format("2006-01-02 15:04:05")
	insertUser := fmt.Sprintf(`INSERT INTO users
		(email, nickname, score, avatar, password, ip, permission,
		 last_sign_at, register_at, verified, verification_token, remember_token,
		 locale, is_dark_mode)
		VALUES (%s,%s,0,0,%s,%s,0,%s,%s,%s,'',NULL,%s,%s)`,
		ph(dialect, 1), ph(dialect, 2), ph(dialect, 3), ph(dialect, 4),
		timestampArg(dialect, 5), timestampArg(dialect, 6),
		ph(dialect, 7), ph(dialect, 8), ph(dialect, 9))

	if _, err := conn.ExecContext(ctx, insertUser,
		email, "matrix", "x", "127.0.0.1", now, now, false, "", false); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer conn.ExecContext(ctx, "DELETE FROM users WHERE email = "+ph(dialect, 1), email)

	var uid int
	var gotEmail string
	var registerAt any
	selectUser := fmt.Sprintf("SELECT uid, email, register_at FROM users WHERE email = %s",
		ph(dialect, 1))
	if err := conn.QueryRowContext(ctx, selectUser, email).
		Scan(&uid, &gotEmail, &registerAt); err != nil {
		t.Fatalf("select user: %v", err)
	}
	if uid <= 0 || gotEmail != email {
		t.Fatalf("uid=%d email=%q", uid, gotEmail)
	}
	if value := formatTime(registerAt); !regexp.MustCompile(
		`^[0-9]{4}-[0-9]{2}-[0-9]{2}[ T][0-9]{2}:[0-9]{2}:[0-9]{2}`).MatchString(value) {
		t.Fatalf("register_at=%q", value)
	}

	hash := strings.Repeat("a", 64)
	insertTexture := fmt.Sprintf(`INSERT INTO textures
		(name, type, hash, size, uploader, %s, upload_at, likes)
		VALUES (%s,%s,%s,%s,%s,%s,%s,1)`,
		dialect.Quote("public"),
		ph(dialect, 1), ph(dialect, 2), ph(dialect, 3), ph(dialect, 4),
		ph(dialect, 5), ph(dialect, 6), timestampArg(dialect, 7))
	if _, err := conn.ExecContext(ctx, insertTexture,
		"matrix", "steve", hash, 1, uid, 1, now); err != nil {
		t.Fatalf("insert texture: %v", err)
	}
	defer conn.ExecContext(ctx, "DELETE FROM textures WHERE hash = "+ph(dialect, 1), hash)

	var tid int
	var public int
	selectTexture := fmt.Sprintf("SELECT tid, %s FROM textures WHERE hash = %s",
		dialect.Quote("public"), ph(dialect, 1))
	if err := conn.QueryRowContext(ctx, selectTexture, hash).Scan(&tid, &public); err != nil {
		t.Fatalf("select texture: %v", err)
	}
	if tid <= 0 || public != 1 {
		t.Fatalf("tid=%d public=%d", tid, public)
	}
}

func ph(dialect *Dialect, index int) string {
	switch dialect.Name {
	case "pgsql":
		return fmt.Sprintf("$%d", index)
	case "sqlsrv":
		return fmt.Sprintf("@p%d", index)
	}

	return "?"
}

func timestampArg(dialect *Dialect, index int) string {
	if dialect.Name == "pgsql" {
		return ph(dialect, index) + "::timestamp"
	}

	return ph(dialect, index)
}

func formatTime(value any) string {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02 15:04:05")
	case []byte:
		return string(typed)
	case string:
		return typed
	case sql.NullTime:
		if typed.Valid {
			return typed.Time.Format("2006-01-02 15:04:05")
		}
	}

	return fmt.Sprint(value)
}

func TestInstallSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "neoskin.sqlite")
	conn, dialect, err := Open(Config{Connection: "sqlite", Database: path})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	installer := NewInstaller(conn, dialect)
	created, err := installer.Install(context.Background())
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if created != 16 {
		t.Fatalf("created=%d, want 16", created)
	}

	again, err := installer.Install(context.Background())
	if err != nil || again != 0 {
		t.Fatalf("second install created=%d err=%v", again, err)
	}
}
