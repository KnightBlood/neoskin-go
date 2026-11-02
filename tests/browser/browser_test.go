package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"neoskin/internal/db"
	"neoskin/internal/model"
	"neoskin/internal/php"
	"neoskin/internal/repo"
	"neoskin/tests/support"
)

// TestBrowserFlows 启动真实服务后运行 Playwright 流程脚本，断言核心页面可用
// 且没有 console error。需要 public/app 前端产物与 tests/browser/node_modules。
// 默认跳过；设置 NEOSKIN_BROWSER_E2E=1 启用（CI 中启用）。
func TestBrowserFlows(t *testing.T) {
	if os.Getenv("NEOSKIN_BROWSER_E2E") != "1" {
		t.Skip("set NEOSKIN_BROWSER_E2E=1 to run browser E2E")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if !hasFrontendAssets(filepath.Join(root, "public", "app")) {
		t.Skip("public/app assets missing; run scripts/copy-frontend.ps1 first")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	if _, err := os.Stat(filepath.Join("node_modules", "playwright-core")); err != nil {
		t.Skip("run `npm install` in tests/browser first")
	}

	server := support.Start(t)
	seedBrowserUser(t, server.DBPath)

	command := exec.Command("node", filepath.Join("tests", "browser", "flows.mjs"))
	command.Dir = server.Root
	command.Env = append(os.Environ(), "NEOSKIN_BASE_URL="+server.BaseURL)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("playwright flows: %v\n%s", err, output)
	} else {
		t.Logf("playwright flows:\n%s", output)
	}
}

func hasFrontendAssets(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".map") {
			return true
		}
	}
	return false
}

func seedBrowserUser(t *testing.T, dbPath string) {
	t.Helper()
	conn, dialect, err := db.Open(db.Config{Connection: "sqlite", Database: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	users := repo.NewUserRepository(conn, dialect)
	password, err := php.HashPassword("secret123", 4)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := users.Insert("admin@example.com", "Admin", password, "127.0.0.1", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if err := users.UpdatePermission(uid, model.PermissionAdmin); err != nil {
		t.Fatal(err)
	}
	if err := users.UpdateVerified(uid, true); err != nil {
		t.Fatal(err)
	}
}
