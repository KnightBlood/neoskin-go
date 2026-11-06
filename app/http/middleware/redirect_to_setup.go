package middleware

import (
	"fmt"
	nethttp "net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/goravel/framework/contracts/http"

	"neoskin/internal/runtime"
	"neoskin/internal/update"
)

// RedirectToSetup 对应 PHP `App\Http\Middleware\RedirectToSetup`。
//
//   - 没有 install.lock：除 setup* 之外一律 302 /setup；
//   - 已安装且 option('version') 落后：请求期执行一次 `update`
//     （迁移 + 版本号 + install.lock + 清缓存）。
type RedirectToSetup struct{}

// NewRedirectToSetup 返回全局中间件实例。
func NewRedirectToSetup() http.Middleware {
	return &RedirectToSetup{}
}

func (m *RedirectToSetup) Handle(ctx http.Context) {
	path := strings.TrimPrefix(ctx.Request().Path(), "/")

	if isSetupPath(path) || strings.HasPrefix(path, "__neoskin/") {
		ctx.Request().Next()
		return
	}

	if installed() {
		if err := update.EnsureUpgraded(ctx.Request().Origin().Context()); err != nil {
			fmt.Fprintln(os.Stderr, "auto update skipped:", err)
		}
		ctx.Request().Next()
		return
	}

	host := ctx.Request().Host()
	if host == "" {
		host = "localhost"
	}
	// Goravel 的 Redirect 只有调用 Abort() 才会真正结束请求。
	if err := ctx.Response().Redirect(nethttp.StatusFound, "http://"+host+"/setup").Abort(); err != nil {
		fmt.Fprintln(os.Stderr, "redirect to setup failed:", err)
	}
}

func (m *RedirectToSetup) Signature() string {
	return "neoskin:redirect_to_setup"
}

// isSetupPath 等价于 `$request->is('setup*')`：按路径段匹配，setupx 不算。
func isSetupPath(path string) bool {
	path = strings.TrimPrefix(path, "/")

	return path == "setup" || strings.HasPrefix(path, "setup/")
}

func installed() bool {
	_, err := os.Stat(filepath.Join(runtime.StorageRoot(), "install.lock"))

	return err == nil
}
