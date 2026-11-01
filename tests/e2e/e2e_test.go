package e2e

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHTTPEndToEnd(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	name := "neoskin-e2e"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(work, name)
	if err := run(t, root, "go", "build", "-o", binary, "."); err != nil {
		t.Skipf("cannot build binary: %v", err)
	}

	port := freePort(t)
	storage := filepath.Join(work, "storage")
	dbPath := filepath.Join(work, "e2e.sqlite")
	env := []string{
		"APP_ENV=local",
		"APP_KEY=0123456789abcdef0123456789abcdef",
		"APP_DEBUG=true",
		"APP_HOST=127.0.0.1",
		"APP_PORT=" + port,
		"DB_CONNECTION=sqlite",
		"DB_DATABASE=" + dbPath,
		"NEOSKIN_STORAGE_ROOT=" + storage,
	}

	// 建表（artisan neoskin:install 幂等）。
	install := exec.Command(binary, "artisan", "neoskin:install")
	install.Dir = root
	install.Env = append(os.Environ(), env...)
	if output, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, output)
	}

	server := exec.Command(binary)
	server.Dir = root
	server.Env = append(os.Environ(), env...)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = server.Process.Kill()
		_, _ = server.Process.Wait()
	}()

	base := "http://127.0.0.1:" + port
	waitHealthy(t, base+"/__neoskin/health")

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request := func(path string) (*http.Response, string) {
		response, err := client.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		return response, string(body)
	}

	cases := []struct {
		path        string
		status      int
		contains    string
		contentType string
	}{
		{"/", 200, "window.blessing", "text/html"},
		{"/auth/login", 200, "csrf-token", "text/html"},
		{"/auth/register", 200, "csrf-token", "text/html"},
		{"/setup", 200, "Blessing Skin", "text/html"},
		{"/lang/zh_CN.js", 200, "blessing.i18n =", "application/javascript"},
		{"/api", 200, "blessing_skin", "application/json"},
		{"/skinlib/list", 200, "[", "application/json"},
		{"/avatar/0?png=1", 200, "", "image/png"},
		{"/avatar/0", 200, "", "image/webp"},
		{"/user", 302, "", ""},
	}
	for _, item := range cases {
		response, body := request(item.path)
		if response.StatusCode != item.status {
			t.Fatalf("%s status=%d want %d", item.path, response.StatusCode, item.status)
		}
		if item.contains != "" && !strings.Contains(body, item.contains) {
			t.Fatalf("%s missing %q", item.path, item.contains)
		}
		if item.contentType != "" {
			contentType := response.Header.Get("Content-Type")
			if !strings.HasPrefix(contentType, item.contentType) {
				t.Fatalf("%s content-type=%q want %q", item.path, contentType, item.contentType)
			}
		}
	}
}

func run(t *testing.T, dir, name string, args ...string) error {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Logf("%s %v: %s", name, args, output)
	}
	return err
}

func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return fmt.Sprint(listener.Addr().(*net.TCPAddr).Port)
}

func waitHealthy(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("server did not become healthy: %s", url)
}
