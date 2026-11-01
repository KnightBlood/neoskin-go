package runtime

import (
	"database/sql"
	"fmt"
	"os"
	"sync"

	"neoskin/app/facades"
	"neoskin/internal/db"
)

var (
	once    sync.Once
	conn    *sql.DB
	dialect *db.Dialect
	err     error
)

// Connection 懒加载应用的 *sql.DB（与 Goravel 的 GORM 连接分开，便于原生 SQL）。
func Connection() (*sql.DB, *db.Dialect, error) {
	once.Do(func() {
		conn, dialect, err = db.OpenFromEnv(env)
	})

	return conn, dialect, err
}

func env(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	if value := facades.Config().Env(key); value != nil {
		return fmt.Sprint(value)
	}

	return ""
}

// StorageRoot 返回纹理/上传的存储根目录（NEOSKIN_STORAGE_ROOT，默认 storage）。
func StorageRoot() string {
	if root := env("NEOSKIN_STORAGE_ROOT"); root != "" {
		return root
	}

	return "storage"
}

// Reset 仅测试用：关闭并清空连接。
func Reset() error {
	if conn != nil {
		_ = conn.Close()
	}
	conn, dialect, err = nil, nil, nil
	once = sync.Once{}

	return nil
}
