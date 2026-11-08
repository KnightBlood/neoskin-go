package commands

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"

	"neoskin/internal/db"
	"neoskin/internal/model"
	"neoskin/internal/php"
	"neoskin/internal/plugin"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
	"neoskin/internal/update"
)

// Update 对应 PHP `php artisan update`。
type Update struct{}

func (r *Update) Signature() string { return "update" }

func (r *Update) Description() string { return "Execute database and version update" }

func (r *Update) Extend() command.Extend { return command.Extend{} }

func (r *Update) Handle(ctx console.Context) error {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return err
	}

	if err := update.Run(context.Background(), conn, dialect); err != nil {
		return err
	}

	ctx.Info("Update completed.")

	return nil
}

// BsInstall 对应 PHP `php artisan bs:install {email} {password} {nickname}`。
type BsInstall struct{}

func (r *BsInstall) Signature() string { return "bs:install" }

func (r *BsInstall) Description() string {
	return "Create the database schema and a super administrator"
}

func (r *BsInstall) Extend() command.Extend {
	return command.Extend{Arguments: []command.Argument{
		&command.ArgumentString{Name: "email", Usage: "administrator email", Required: true},
		&command.ArgumentString{Name: "password", Usage: "administrator password", Required: true},
		&command.ArgumentString{Name: "nickname", Usage: "administrator nickname", Required: true},
	}}
}

func (r *BsInstall) Handle(ctx console.Context) error {
	lockPath := filepath.Join(runtime.StorageRoot(), "install.lock")
	if _, err := os.Stat(lockPath); err == nil {
		ctx.Info("You have installed NeoSkin. Nothing to do.")
		return nil
	}

	email := ctx.Argument(0)
	password := ctx.Argument(1)
	nickname := ctx.Argument(2)

	conn, dialect, err := runtime.Connection()
	if err != nil {
		return err
	}

	if _, err := db.NewInstaller(conn, dialect).Install(context.Background()); err != nil {
		return err
	}

	options := service.NewOption(conn, dialect)
	_ = options.Set("site_url", "http://localhost/")
	_ = options.Set("version", update.AppVersion())

	hashed, err := php.HashPassword(password, 0)
	if err != nil {
		return err
	}

	users := repo.NewUserRepository(conn, dialect)
	uid, err := users.Insert(email, nickname, hashed, "127.0.0.1",
		options.Int("user_initial_score", 1000))
	if err != nil {
		return err
	}

	yesterday := time.Now().Add(-24 * time.Hour)
	if _, err := conn.Exec(db.Rebind(dialect,
		`UPDATE users SET permission = ?, verified = ?, last_sign_at = ? WHERE uid = ?`),
		model.PermissionSuper, true, php.Format(yesterday), uid); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(lockPath, []byte{}, 0o644); err != nil {
		return err
	}

	ctx.Info("Installation completed!")
	ctx.Info("We recommend to modify your \"Site URL\" option if incorrect.")

	return nil
}

// OptionsCache 对应 PHP `php artisan options:cache`。
type OptionsCache struct{}

func (r *OptionsCache) Signature() string { return "options:cache" }

func (r *OptionsCache) Description() string { return "Cache NeoSkin options" }

func (r *OptionsCache) Extend() command.Extend { return command.Extend{} }

func (r *OptionsCache) Handle(ctx console.Context) error {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return err
	}

	rows, err := conn.Query(db.Rebind(dialect,
		`SELECT option_name, option_value FROM options`))
	if err != nil {
		return err
	}
	defer rows.Close()

	var builder strings.Builder
	builder.WriteString("<?php\n// This is auto-generated. DO NOT edit manually.\nreturn array (\n")
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return err
		}
		fmt.Fprintf(&builder, "  %s => %s,\n", phpExport(name), phpExport(value))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	builder.WriteString(");\n")

	path := filepath.Join(runtime.StorageRoot(), "options.php")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return err
	}

	ctx.Info("Options cached successfully.")

	return nil
}

// PluginEnable 对应 PHP `php artisan plugin:enable {name}`。
type PluginEnable struct{}

func (r *PluginEnable) Signature() string { return "plugin:enable" }

func (r *PluginEnable) Description() string { return "Enable a plugin" }

func (r *PluginEnable) Extend() command.Extend {
	return command.Extend{Arguments: []command.Argument{
		&command.ArgumentString{Name: "name", Usage: "plugin directory name", Required: true},
	}}
}

func (r *PluginEnable) Handle(ctx console.Context) error {
	return setPluginEnabled(ctx, ctx.Argument(0), true)
}

// PluginDisable 对应 PHP `php artisan plugin:disable {name}`。
type PluginDisable struct{}

func (r *PluginDisable) Signature() string { return "plugin:disable" }

func (r *PluginDisable) Description() string { return "Disable a plugin" }

func (r *PluginDisable) Extend() command.Extend {
	return command.Extend{Arguments: []command.Argument{
		&command.ArgumentString{Name: "name", Usage: "plugin directory name", Required: true},
	}}
}

func (r *PluginDisable) Handle(ctx console.Context) error {
	return setPluginEnabled(ctx, ctx.Argument(0), false)
}

func setPluginEnabled(ctx console.Context, name string, enabled bool) error {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return err
	}

	manager := plugin.NewManager("", service.NewOption(conn, dialect))
	found, err := manager.Find(name)
	if err != nil {
		return err
	}
	if found == nil {
		ctx.Error("Plugin [" + name + "] not found.")
		return nil
	}

	if err := manager.SetEnabled(name, enabled); err != nil {
		return err
	}

	if enabled {
		ctx.Info("Plugin [" + name + "] enabled.")
	} else {
		ctx.Info("Plugin [" + name + "] disabled.")
	}

	return nil
}

// SaltRandom 对应 PHP `php artisan salt:random {--show}`。
type SaltRandom struct{}

func (r *SaltRandom) Signature() string { return "salt:random" }

func (r *SaltRandom) Description() string { return "Set or display the application salt" }

func (r *SaltRandom) Extend() command.Extend {
	return command.Extend{Flags: []command.Flag{
		&command.BoolFlag{Name: "show", Usage: "display the salt instead of writing .env"},
	}}
}

func (r *SaltRandom) Handle(ctx console.Context) error {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	salt := hex.EncodeToString(bytes)

	if strings.EqualFold(ctx.Option("show"), "true") {
		ctx.Info(salt)
		return nil
	}

	if err := writeEnv("SALT", salt); err != nil {
		return err
	}
	ctx.Info("Application salt [" + salt + "] set successfully.")

	return nil
}

// phpExport 生成 PHP 单引号字符串（只转义 \ 与 '）。
func phpExport(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)

	return "'" + escaped + "'"
}

// writeEnv 替换或追加 .env 中的键。
func writeEnv(name, value string) error {
	path := ".env"
	content, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(content), "\r\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	replaced := false
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), name+"=") {
			lines[index] = name + "=" + value
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, name+"="+value)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
