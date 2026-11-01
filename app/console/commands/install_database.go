package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"

	"neoskin/app/facades"
	"neoskin/internal/db"
)

// InstallDatabase 创建 NeoSkin 的 16 张业务表 + migrations 表（幂等）。
type InstallDatabase struct{}

func (r *InstallDatabase) Signature() string {
	return "neoskin:install"
}

func (r *InstallDatabase) Description() string {
	return "Create NeoSkin database tables (idempotent)"
}

func (r *InstallDatabase) Extend() command.Extend {
	return command.Extend{}
}

func (r *InstallDatabase) Handle(ctx console.Context) error {
	conn, dialect, err := db.OpenFromEnv(env)
	if err != nil {
		return err
	}
	defer conn.Close()

	created, err := db.NewInstaller(conn, dialect).Install(context.Background())
	if err != nil {
		return err
	}

	ctx.Info(fmt.Sprintf("database=%s created=%d tables", dialect.Name, created))

	return nil
}

// env 先读进程环境，再读 Goravel 的 .env 配置。
func env(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	if value := facades.Config().Env(key); value != nil {
		return fmt.Sprint(value)
	}

	return ""
}
