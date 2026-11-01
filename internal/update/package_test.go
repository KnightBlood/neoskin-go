package update

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func buildPackage(t *testing.T, version string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	prefix := "blessing-skin-server/"
	entries := map[string]string{
		prefix + "composer.json":                   `{"name":"bs-community/blessing-skin-server"}`,
		prefix + "artisan":                         "artisan",
		prefix + "bootstrap/app.php":               "app",
		prefix + "config/app.php":                  `<?php return ['version' => '` + version + `'];`,
		prefix + "public/index.php":                "index",
		prefix + "vendor/autoload.php":             "autoload",
		prefix + "resources/views/assets/app.twig": "<html></html>",
		prefix + "public/app/main.js":              "console.log(1);",
		prefix + "app/Test.php":                    "new",
		prefix + "storage/ignored.txt":             "ignored",
		prefix + "public/textures/keep.txt":        "keep",
	}
	for name, body := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPrepareAndInstall(t *testing.T) {
	root := t.TempDir()
	storage := t.TempDir()
	manager := NewManager(root, storage, "6.0.11_patch")

	info, err := manager.Prepare(buildPackage(t, "6.0.12_patch"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "6.0.12_patch" || info.Count != 9 {
		t.Fatalf("info=%+v", info)
	}
	found := false
	for _, name := range info.Files {
		if name == "app/Test.php" {
			found = true
		}
		if name == "storage/ignored.txt" || name == "public/textures/keep.txt" {
			t.Fatalf("disallowed file extracted: %s", name)
		}
	}
	if !found {
		t.Fatal("app/Test.php missing")
	}
	if len(info.SHA256) != 64 {
		t.Fatalf("sha256=%q", info.SHA256)
	}

	target := filepath.Join(root, "app", "Test.php")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.Install(info.ID); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(target)
	if err != nil || string(body) != "new" {
		t.Fatalf("body=%q err=%v", body, err)
	}
	if _, err := os.Stat(filepath.Join(storage, "app", "system-updates", info.ID, "installed.json")); err != nil {
		t.Fatal("installed.json missing")
	}
	if err := manager.Install(info.ID); err == nil {
		t.Fatal("second install must fail")
	}
}

func TestPrepareRejectsOldVersion(t *testing.T) {
	manager := NewManager(t.TempDir(), t.TempDir(), "6.0.11_patch")
	if _, err := manager.Prepare(buildPackage(t, "6.0.10")); err == nil {
		t.Fatal("old package must be rejected")
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"6.0.12", "6.0.11_patch", 1},
		{"6.0.11", "6.0.11_patch", 0},
		{"v6.0.9", "6.0.11", -1},
	}
	for _, item := range cases {
		if got := Compare(item.left, item.right); got != item.want {
			t.Fatalf("Compare(%q,%q)=%d want %d", item.left, item.right, got, item.want)
		}
	}
}

func TestPrepareRejectsTraversal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	file, _ := os.Create(path)
	writer := zip.NewWriter(file)
	entry, _ := writer.Create("../evil.txt")
	_, _ = entry.Write([]byte("x"))
	_ = writer.Close()
	_ = file.Close()

	manager := NewManager(t.TempDir(), t.TempDir(), "6.0.11_patch")
	if _, err := manager.Prepare(path); err == nil {
		t.Fatal("path traversal must be rejected")
	}
}
