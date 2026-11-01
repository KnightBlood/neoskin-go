package repo

import (
	"context"
	"path/filepath"
	"testing"

	"neoskin/internal/db"
	"neoskin/internal/php"
)

func TestRepositoriesSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo.sqlite")
	conn, dialect, err := db.Open(db.Config{Connection: "sqlite", Database: path})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := db.NewInstaller(conn, dialect).Install(context.Background()); err != nil {
		t.Fatal(err)
	}

	users := NewUserRepository(conn, dialect)
	password, err := php.HashPassword("secret123", 4)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := users.Insert("user@example.com", "Steve", password, "127.0.0.1", 1000)
	if err != nil {
		t.Fatal(err)
	}

	user, err := users.FindByEmail("user@example.com")
	if err != nil || user == nil {
		t.Fatalf("find user: user=%v err=%v", user, err)
	}
	if user.UID != uid || !php.VerifyPassword(user.Password, "secret123") {
		t.Fatalf("unexpected user: %+v", user)
	}

	players := NewPlayerRepository(conn, dialect)
	pid, err := players.Insert(uid, "Steve")
	if err != nil {
		t.Fatal(err)
	}

	textures := NewTextureRepository(conn, dialect)
	hash := php.Sha256Hex([]byte("texture"))
	tid, err := textures.Insert("Steve skin", "steve", hash, 1, uid, true, php.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := players.UpdateTextures(pid, tid, 0); err != nil {
		t.Fatal(err)
	}

	detail, err := players.FindDetailByName("Steve")
	if err != nil || detail == nil {
		t.Fatalf("find player: detail=%v err=%v", detail, err)
	}
	if detail.TidSkin != tid || detail.Permission != 0 {
		t.Fatalf("unexpected player: %+v", detail)
	}

	texture, err := textures.FindByTid(tid)
	if err != nil || texture == nil {
		t.Fatalf("find texture: texture=%v err=%v", texture, err)
	}
	if texture.Hash != hash || !texture.Public || texture.TextureModel() != "default" {
		t.Fatalf("unexpected texture: %+v", texture)
	}
}
