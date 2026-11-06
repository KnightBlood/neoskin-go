package controllers

import (
	"context"
	"database/sql"
	"fmt"
	nethttp "net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/db"
	"neoskin/internal/i18n"
	"neoskin/internal/model"
	"neoskin/internal/oauth"
	"neoskin/internal/php"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
	"neoskin/internal/view"
)

// SetupController 对应源 SetupController（安装向导）。
type SetupController struct{}

func NewSetupController() *SetupController {
	return &SetupController{}
}

// lockPath 对应 PHP storage_path('install.lock')。
func lockPath() string {
	return filepath.Join(runtime.StorageRoot(), "install.lock")
}

func installed() bool {
	_, err := os.Stat(lockPath())
	return err == nil
}

// Show 处理 GET /setup。
func (r *SetupController) Show(ctx goravelhttp.Context) goravelhttp.Response {
	if installed() {
		return lockedResponse(ctx)
	}
	return renderSetupPage(ctx, "setup_welcome", "setup.wizard.welcome.title", nil)
}

func lockedResponse(ctx goravelhttp.Context) goravelhttp.Response {
	// PHP `CheckInstallation`：已安装时渲染 setup.locked 视图（不是 JSON）。
	return renderSetupPage(ctx, "setup_locked", "setup.locked.title", nil)
}

// Database 处理 ANY /setup/database。
func (r *SetupController) Database(ctx goravelhttp.Context) goravelhttp.Response {
	if installed() {
		return lockedResponse(ctx)
	}

	if ctx.Request().Method() == nethttp.MethodGet {
		values := envFileValues(".env")
		if _, _, err := openSetupConnection(values); err == nil {
			return ctx.Response().Redirect(nethttp.StatusFound, "/setup/info")
		}
		return renderSetupPage(ctx, "setup_database", "setup.wizard.database.title", values)
	}

	input := func(key string) string { return strings.TrimSpace(ctx.Request().Input(key)) }
	values := map[string]string{
		"DB_CONNECTION": input("type"),
		"DB_HOST":       input("host"),
		"DB_PORT":       input("port"),
		"DB_DATABASE":   input("db"),
		"DB_USERNAME":   input("username"),
		"DB_PASSWORD":   input("password"),
		"DB_PREFIX":     input("prefix"),
	}
	conn, _, err := openSetupConnection(values)
	if err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}
	_ = conn.Close()

	if err := updateEnvFile(".env", values); err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}
	return ctx.Response().Redirect(nethttp.StatusFound, "/setup/info")
}

// Info 处理 GET /setup/info。
func (r *SetupController) Info(ctx goravelhttp.Context) goravelhttp.Response {
	if installed() {
		return lockedResponse(ctx)
	}
	return renderSetupPage(ctx, "setup_info", "setup.wizard.info.title", nil)
}

// Finish 处理 POST /setup/finish。
func (r *SetupController) Finish(ctx goravelhttp.Context) goravelhttp.Response {
	if installed() {
		return lockedResponse(ctx)
	}

	email := strings.TrimSpace(ctx.Request().Input("email"))
	nickname := strings.TrimSpace(ctx.Request().Input("nickname"))
	password := ctx.Request().Input("password")
	confirmation := ctx.Request().Input("password_confirmation")
	siteName := strings.TrimSpace(ctx.Request().Input("site_name"))

	errs := map[string][]string{}
	if email == "" {
		errs["email"] = []string{vRequired(ctx, "email")}
	} else if !strings.Contains(email, "@") {
		errs["email"] = []string{vEmail(ctx, "email")}
	}
	if nickname == "" {
		errs["nickname"] = []string{vRequired(ctx, "nickname")}
	}
	if message := vPasswordMessage(ctx, "password", password, 8, 32); message != "" {
		errs["password"] = []string{message}
	} else if password != confirmation {
		errs["password"] = []string{vConfirmed(ctx, "password")}
	}
	if siteName == "" {
		errs["site_name"] = []string{vRequired(ctx, "site_name")}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	conn, dialect, err := openSetupConnection(envFileValues(".env"))
	if err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}
	defer conn.Close()

	if _, err := oauth.RuntimeKeys(); err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}
	if _, err := db.NewInstaller(conn, dialect).Install(context.Background()); err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}

	options := service.NewOption(conn, dialect)
	_ = options.Set("site_name", siteName)
	_ = options.Set("site_url", "http://"+ctx.Request().Host())
	_ = options.Set("version", facades.Config().GetString("app.version", "6.0.11_patch"))

	hashed, err := php.HashPassword(password, 0)
	if err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}
	initialScore := options.Int("user_initial_score", 1000)
	users := repo.NewUserRepository(conn, dialect)
	uid, err := users.Insert(email, nickname, hashed, ctx.Request().Ip(), initialScore)
	if err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}
	yesterday := time.Now().Add(-24 * time.Hour)
	_, err = conn.Exec(db.Rebind(dialect,
		`UPDATE users SET permission = ?, verified = ?, last_sign_at = ? WHERE uid = ?`),
		model.PermissionSuper, true, php.Format(yesterday), uid)
	if err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}

	if err := os.MkdirAll(filepath.Dir(lockPath()), 0o755); err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}
	if err := os.WriteFile(lockPath(), []byte{}, 0o644); err != nil {
		return ctx.Response().Json(nethttp.StatusInternalServerError, goravelhttp.Json{
			"message": err.Error(),
		})
	}

	return renderSetupPage(ctx, "setup_finish", "setup.wizard.finish.title", nil)
}

func openSetupConnection(values map[string]string) (*sql.DB, *db.Dialect, error) {
	cfg := db.Config{
		Connection: values["DB_CONNECTION"],
		Host:       values["DB_HOST"],
		Port:       values["DB_PORT"],
		Database:   values["DB_DATABASE"],
		Username:   values["DB_USERNAME"],
		Password:   values["DB_PASSWORD"],
	}
	if cfg.Connection == "" {
		cfg.Connection = "mysql"
	}
	conn, dialect, err := db.Open(cfg)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, dialect, nil
}

var envLinePattern = regexp.MustCompile(`^([A-Z0-9_]+)=`)

func envFileValues(path string) map[string]string {
	values := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return values
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		match := envLinePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		values[match[1]] = strings.TrimSpace(strings.TrimPrefix(line, match[1]+"="))
	}
	return values
}

func updateEnvFile(path string, values map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		data = []byte{}
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		match := envLinePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		key := match[1]
		value, ok := values[key]
		if !ok {
			continue
		}
		lines[i] = fmt.Sprintf("%s=%s", key, value)
		seen[key] = true
	}
	for key, value := range values {
		if seen[key] {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func csrfToken(ctx goravelhttp.Context) string {
	if session := ctx.Request().Session(); session != nil {
		return session.Token()
	}
	return ""
}

// renderSetupPage 渲染安装向导页面。
func renderSetupPage(ctx goravelhttp.Context, page, titleKey string,
	values map[string]string) goravelhttp.Response {
	locale := localeMiddleware.LocaleOf(ctx)
	translator := i18n.New(locale, nil)
	data := view.SetupData{
		Locale:     locale,
		Title:      translator.Trans(titleKey),
		CSRF:       csrfToken(ctx),
		Version:    facades.Config().GetString("app.version", "6.0.11_patch"),
		Values:     values,
		StyleCSS:   view.AssetURL("", view.Asset("style.", ".css", "style.1dd8a8a.css")),
		SpectreCSS: view.AssetURL("", view.Asset("spectre.", ".css", "spectre.e885dd0.css")),
	}
	body, err := view.Render(page, data, translator)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Data(nethttp.StatusOK, "text/html; charset=utf-8", body)
}
