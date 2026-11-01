package repo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"neoskin/internal/db"
	"neoskin/internal/php"
)

// TestRepositoriesMatrix 在真实数据库上验证仓储层的方言差异（自增回填/时间戳/布尔）。
func TestRepositoriesMatrix(t *testing.T) {
	switch os.Getenv("DB_CONNECTION") {
	case "mysql", "mariadb", "pgsql", "sqlsrv":
	default:
		t.Skip("DB_CONNECTION 不是 mysql/mariadb/pgsql/sqlsrv")
	}

	conn, dialect, err := db.OpenFromEnv(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	installer := db.NewInstaller(conn, dialect)
	if err := installer.DropAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install(context.Background()); err != nil {
		t.Fatal(err)
	}

	users := NewUserRepository(conn, dialect)
	password, err := php.HashPassword("secret123", 4)
	if err != nil {
		t.Fatal(err)
	}
	email := fmt.Sprintf("repo-%d@%s.local", time.Now().UnixNano(), dialect.Name)
	uid, err := users.Insert(email, "Matrix", password, "127.0.0.1", 1000)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	defer users.DeleteWithPlayers(uid)

	user, err := users.FindByEmail(email)
	if err != nil || user == nil {
		t.Fatalf("find user: %v %v", user, err)
	}
	if !php.VerifyPassword(user.Password, "secret123") {
		t.Fatal("password verify failed")
	}

	players := NewPlayerRepository(conn, dialect)
	pid, err := players.Insert(uid, "MatrixPlayer")
	if err != nil {
		t.Fatalf("insert player: %v", err)
	}

	textures := NewTextureRepository(conn, dialect)
	hash := php.Sha256Hex([]byte(email))
	tid, err := textures.Insert("Matrix skin", "alex", hash, 1, uid, true, php.Now())
	if err != nil {
		t.Fatalf("insert texture: %v", err)
	}
	defer textures.DeleteByTid(tid)

	if err := players.UpdateTextures(pid, tid, 0); err != nil {
		t.Fatalf("update textures: %v", err)
	}

	detail, err := players.FindDetailByName("MatrixPlayer")
	if err != nil || detail == nil {
		t.Fatalf("find player: %v %v", detail, err)
	}
	if detail.TidSkin != tid || detail.Permission != 0 {
		t.Fatalf("unexpected player: %+v", detail)
	}
	if detail.LastModified == "" {
		t.Fatal("last_modified is empty")
	}

	texture, err := textures.FindByTid(tid)
	if err != nil || texture == nil {
		t.Fatalf("find texture: %v %v", texture, err)
	}
	if texture.TextureModel() != "slim" || !texture.Public || texture.UploadAt == "" {
		t.Fatalf("unexpected texture: %+v", texture)
	}
}
