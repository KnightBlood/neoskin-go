package update

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"neoskin/internal/db"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
)

// DefaultVersion 对应 config/app.php 的默认值。
const DefaultVersion = "6.0.11_patch"

// AppVersion 返回当前站点版本（PHP config('app.version')）。
func AppVersion() string {
	if value := os.Getenv("APP_VERSION"); strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}

	return DefaultVersion
}

// ParseVersion 把 6.0.11_patch 归一化为 [6, 0, 11]。
func ParseVersion(value string) []int {
	value = strings.TrimSpace(strings.TrimLeft(value, "vV"))
	end := 0
	for index, char := range value {
		if (char >= '0' && char <= '9') || char == '.' {
			end = index + 1
			continue
		}
		break
	}
	if end == 0 {
		return nil
	}

	parts := strings.Split(value[:end], ".")
	numbers := make([]int, 0, 3)
	for _, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			number = 0
		}
		numbers = append(numbers, number)
	}

	return numbers
}

// IsNewer 等价于 Composer\Semver\Comparator::greaterThan(a, b)。
func IsNewer(a, b string) bool {
	left, right := ParseVersion(a), ParseVersion(b)
	if left == nil || right == nil {
		return strings.TrimSpace(a) != strings.TrimSpace(b)
	}
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] != right[index] {
			return left[index] > right[index]
		}
	}

	return len(left) > len(right)
}

// NeedsUpdate：已安装版本落后于程序版本。
//
// PHP `option('version', $version)` 在键缺失时用程序版本兜底，因此缺失不算升级。
func NeedsUpdate(stored string) bool {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return false
	}

	return IsNewer(AppVersion(), stored)
}

var (
	upgradeMu sync.Mutex
	upgraded  bool
)

// EnsureUpgraded 是请求期入口（PHP RedirectToSetup 里的 Artisan::call('update')）。
//
// 只检查一次；失败不置位，下个请求可重试。并发请求由互斥锁串行化。
func EnsureUpgraded(ctx context.Context) error {
	upgradeMu.Lock()
	defer upgradeMu.Unlock()

	if upgraded {
		return nil
	}

	conn, dialect, err := runtime.Connection()
	if err != nil {
		return err
	}
	options := service.NewOption(conn, dialect)
	if !NeedsUpdate(options.Get("version")) {
		upgraded = true
		return nil
	}

	if err := Run(ctx, conn, dialect); err != nil {
		return err
	}
	upgraded = true

	return nil
}

// Run 执行一次完整 update（PHP App\Console\Commands\UpdateCommand）。
func Run(ctx context.Context, conn *sql.DB, dialect *db.Dialect) error {
	// 先补表：`update` 也可能在「代码已升级、库还是空/半旧」时被调用。
	if _, err := db.NewInstaller(conn, dialect).Install(ctx); err != nil {
		return err
	}

	options := service.NewOption(conn, dialect)
	stored := options.Get("version")
	version := AppVersion()

	// 0.0.1：源里仅用于测试的事件；Go 端没有同名事件总线，保留空操作。
	_ = stored

	// 5.0.0：默认主页背景从 JPG 换成 WebP。
	if stored == "" || IsNewer("5.0.0", stored) {
		if options.Get("home_pic_url") == "./app/bg.jpg" {
			if err := options.Set("home_pic_url", "./app/bg.webp"); err != nil {
				return err
			}
		}
	}

	if err := options.Set("version", version); err != nil {
		return err
	}

	clearCaches()

	lockPath := filepath.Join(runtime.StorageRoot(), "install.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(lockPath, []byte{}, 0o644); err != nil {
		return fmt.Errorf("write install.lock: %w", err)
	}

	return nil
}

// clearCaches 等价于 `php artisan view:clear` + `Cache::flush()`。
func clearCaches() {
	_ = os.Remove(filepath.Join(runtime.StorageRoot(), "options.php"))

	root, err := os.Getwd()
	if err != nil {
		return
	}
	for _, name := range []string{"config.php", "routes.php", "routes-v7.php"} {
		_ = os.Remove(filepath.Join(root, "bootstrap", "cache", name))
	}
}
