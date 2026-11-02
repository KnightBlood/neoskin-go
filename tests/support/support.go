// Package support 提供 E2E/浏览器测试共用的服务启动器。
package support

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Server 是已启动的 NeoSkin 测试实例。
type Server struct {
	BaseURL  string
	DBPath   string
	Root     string
	WorkDir  string
	Binary   string
	Database string
}

// Start 构建二进制、建表并启动服务，测试结束时自动清理。
// 环境变量与 tests/e2e 的既有约定保持一致（SQLite + 临时 storage）。
func Start(t *testing.T) *Server {
	t.Helper()
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
	if err := Run(t, root, "go", "build", "-o", binary, "."); err != nil {
		t.Skipf("cannot build binary: %v", err)
	}

	port := FreePort(t)
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
	t.Cleanup(func() {
		_ = server.Process.Kill()
		_, _ = server.Process.Wait()
	})

	base := "http://127.0.0.1:" + port
	WaitHealthy(t, base+"/__neoskin/health")
	return &Server{
		BaseURL:  base,
		DBPath:   dbPath,
		Root:     root,
		WorkDir:  work,
		Binary:   binary,
		Database: "sqlite",
	}
}

// Run 在指定目录执行命令并把输出写进测试日志。
func Run(t *testing.T, dir, name string, args ...string) error {
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

// FreePort 申请一个空闲端口。
func FreePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return fmt.Sprint(listener.Addr().(*net.TCPAddr).Port)
}

// WaitHealthy 轮询健康检查直到服务就绪。
func WaitHealthy(t *testing.T, url string) {
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
