package service

import (
	"context"
	"path/filepath"
	"testing"

	"neoskin/internal/db"
)

func TestOptionService(t *testing.T) {
	path := filepath.Join(t.TempDir(), "option.sqlite")
	conn, dialect, err := db.Open(db.Config{Connection: "sqlite", Database: path})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := db.NewInstaller(conn, dialect).Install(context.Background()); err != nil {
		t.Fatal(err)
	}

	options := NewOption(conn, dialect)
	if got := options.Int("user_initial_score", 0); got != 1000 {
		t.Fatalf("default score=%d", got)
	}
	if !options.Bool("register_with_player_name", false) {
		t.Fatal("default register_with_player_name should be true")
	}

	if err := options.Set("user_initial_score", "42"); err != nil {
		t.Fatal(err)
	}
	if got := options.Int("user_initial_score", 0); got != 42 {
		t.Fatalf("updated score=%d", got)
	}

	if err := options.Set("user_initial_score", "7"); err != nil {
		t.Fatal(err)
	}
	fresh := NewOption(conn, dialect)
	if got := fresh.Int("user_initial_score", 0); got != 7 {
		t.Fatalf("persisted score=%d", got)
	}
}
