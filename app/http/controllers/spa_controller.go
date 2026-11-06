package controllers

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/auth"
	"neoskin/internal/model"
	"neoskin/internal/plugin"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
)

// SPA 壳与 bootstrap 契约（见 NeoSkinFrontend/docs/BACKEND-CONTRACT.md）。
const (
	contractVersion = 1
	appNextRoot     = "public/app-next"
)

// siteLocales 与 resources/lang 目录保持一致。
var siteLocales = []string{
	"zh_CN", "zh_TW", "en", "de_DE", "es_ES", "fr_FR", "it_IT",
	"ja_JP", "ko_KR", "nl_NL", "pt_PT", "ru_RU", "el_GR",
}

// SPAController 托管 app-next 单页前端并输出契约 v1 bootstrap。
type SPAController struct{}

func NewSPAController() *SPAController { return &SPAController{} }

// bootstrapPayload 组装 GET /api/bootstrap 与壳注入共用的数据。
func bootstrapPayload(ctx goravelhttp.Context, mode string) map[string]any {
	locale := localeMiddleware.LocaleOf(ctx)
	conn, dialect, err := runtime.Connection()
	options := service.NewOption(conn, dialect)
	if err != nil {
		options = service.NewOption(nil, nil)
	}

	siteName := localizedOption(options, "site_name", locale, "Blessing Skin")
	var user any
	if conn != nil {
		if current, err := auth.CurrentUser(ctx, repo.NewUserRepository(conn, dialect)); err == nil && current != nil {
			user = bootstrapUser(current)
		}
	}

	return map[string]any{
		"contractVersion": contractVersion,
		"apiBase":         "",
		"locale":          locale,
		"localeFallback":  "en",
		"siteName":        siteName,
		"version":         facades.Config().GetString("app.version", "6.0.11_patch"),
		"basePath":        "/app-next/" + mode + "/",
		"mode":            mode,
		"csrfToken":       csrfToken(ctx),
		"locales":         siteLocales,
		"user":            user,
		"options":         bootstrapOptions(options, locale),
		"features":        bootstrapFeatures(options),
		"plugins":         bootstrapPlugins(mode),
	}
}

func bootstrapUser(user *model.User) map[string]any {
	return map[string]any{
		"uid":        user.UID,
		"email":      user.Email,
		"nickname":   user.Nickname,
		"avatar":     fmt.Sprintf("/avatar/%d", user.Avatar),
		"score":      user.Score,
		"permission": user.Permission,
		"verified":   user.Verified,
		"isAdmin":    user.IsAdmin(),
		"isBanned":   user.Permission == model.PermissionBanned,
		"darkMode":   user.DarkMode,
	}
}

func bootstrapBool(options *service.Option, key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(options.Get(key)))
	switch raw {
	case "true", "1", "on", "yes":
		return true
	case "false", "0", "off", "no":
		return false
	default:
		return fallback
	}
}

func bootstrapInt(options *service.Option, key string, fallback int) int {
	if parsed, err := strconv.Atoi(strings.TrimSpace(options.Get(key))); err == nil {
		return parsed
	}
	return fallback
}

func bootstrapOptions(options *service.Option, locale string) map[string]any {
	return map[string]any{
		"site_name":                       localizedOption(options, "site_name", locale, "Blessing Skin"),
		"site_description":                localizedOption(options, "site_description", locale, ""),
		"user_can_register":               true,
		"register_with_player_name":       bootstrapBool(options, "register_with_player_name", true),
		"captcha":                         false,
		"recaptcha":                       options.Get("recaptcha_sitekey") != "",
		"recaptcha_site_key":              options.Get("recaptcha_sitekey"),
		"max_upload_file_size":            bootstrapInt(options, "max_upload_file_size", 1024),
		"max_texture_width":               bootstrapInt(options, "max_texture_width", 8192),
		"score_per_storage":               bootstrapInt(options, "score_per_storage", 1),
		"private_score_per_storage":       bootstrapInt(options, "private_score_per_storage", 10),
		"score_award_per_texture":         bootstrapInt(options, "score_award_per_texture", 0),
		"score_per_player":                bootstrapInt(options, "score_per_player", 100),
		"score_per_closet_item":           bootstrapInt(options, "score_per_closet_item", 0),
		"sign_score":                      options.Get("sign_score"),
		"take_back_scores_after_deletion": bootstrapBool(options, "take_back_scores_after_deletion", true),
		"player_name_rule":                defaultString(options.Get("player_name_rule"), "official"),
		"custom_player_name_regexp":       options.Get("custom_player_name_regexp"),
		"navbar_color":                    defaultString(options.Get("navbar_color"), "cyan"),
		"sidebar_color":                   defaultString(options.Get("sidebar_color"), "dark-maroon"),
		"copyright_prefer":                defaultString(options.Get("copyright_prefer"), "0"),
		"copyright_text":                  options.Get("copyright_text"),
		"update_source":                   options.Get("update_source"),
		"plugin_market_source":            options.Get("plugin_market_source"),
	}
}

func bootstrapFeatures(options *service.Option) map[string]any {
	return map[string]any{
		"registration":      true,
		"emailVerification": bootstrapBool(options, "require_verification", false),
		"passwordReset":     options.Get("mail_host") != "" || options.Get("mail_driver") != "",
		"oauth":             true,
		"reports":           true,
		"plugins":           true,
	}
}

func bootstrapPlugins(mode string) map[string]any {
	area := "user"
	if mode == "admin" {
		area = "admin"
	}
	menus := pluginMenuItems(area)
	entries := make([]map[string]any, 0, len(menus))
	for _, item := range menus {
		entries = append(entries, map[string]any{
			"area": area, "title": item.Title, "url": item.Link, "icon": item.Icon,
		})
	}
	return map[string]any{
		"menus":   entries,
		"badges":  []any{},
		"styles":  plugin.DefaultHook.StylesFor("*"),
		"scripts": plugin.DefaultHook.ScriptsFor("*"),
	}
}

// Bootstrap 处理 GET /api/bootstrap。
func (r *SPAController) Bootstrap(ctx goravelhttp.Context) goravelhttp.Response {
	mode := strings.TrimSpace(ctx.Request().Input("mode"))
	if mode != "site" && mode != "admin" {
		mode = "site"
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "ok", "data": bootstrapPayload(ctx, mode),
	})
}

// Shell 处理 GET /app-next/{mode}/*：静态文件直出，其余回退 index.html 并注入 bootstrap。
func (r *SPAController) Shell(ctx goravelhttp.Context) goravelhttp.Response {
	mode := ctx.Request().Route("mode")
	if mode != "site" && mode != "admin" {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	relative := ctx.Request().Route("path")
	if relative == "" || relative == "/" {
		relative = "/index.html"
	}

	base, err := filepath.Abs(filepath.Join(appNextRoot, mode))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	cleaned := path.Clean("/" + strings.ReplaceAll(relative, "\\", "/"))
	target := filepath.Join(base, filepath.FromSlash(cleaned))

	info, statErr := os.Stat(target)
	if statErr == nil && !info.IsDir() {
		if strings.HasSuffix(strings.ToLower(cleaned), ".html") {
			return r.shell(ctx, mode, target)
		}
		ctx.Response().Header("Cache-Control", "public, max-age=31536000, immutable")
		return ctx.Response().File(target)
	}

	// 有扩展名的缺失资源必须 404，不能回退 HTML。
	if ext := path.Ext(cleaned); ext != "" && !strings.EqualFold(ext, ".html") {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}

	index := filepath.Join(base, "index.html")
	if _, err := os.Stat(index); err != nil {
		return ctx.Response().String(nethttp.StatusNotFound,
			"app-next 前端产物缺失：请先运行 pnpm -r build 并执行 scripts/publish.ps1")
	}
	return r.shell(ctx, mode, index)
}

func (r *SPAController) shell(ctx goravelhttp.Context, mode, index string) goravelhttp.Response {
	raw, err := os.ReadFile(index)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	payload, err := json.Marshal(bootstrapPayload(ctx, mode))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	csrf := csrfToken(ctx)
	injection := fmt.Sprintf(
		"<meta name=\"csrf-token\" content=\"%s\">\n"+
			"<meta name=\"neoskin-mode\" content=\"%s\">\n"+
			"<script id=\"neoskin-bootstrap\" type=\"application/json\">%s</script>\n",
		htmlEscape(csrf), mode, escapeScriptJSON(payload),
	)
	html := strings.Replace(string(raw), "</head>", injection+"</head>", 1)
	ctx.Response().Header("Cache-Control", "no-cache")
	return ctx.Response().Data(nethttp.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(value)
}

// escapeScriptJSON 转义 </script> 截断与 U+2028/2029。
func escapeScriptJSON(raw []byte) string {
	text := strings.ReplaceAll(string(raw), "<", `\u003c`)
	text = strings.ReplaceAll(text, ">", `\u003e`)
	text = strings.ReplaceAll(text, "&", `\u0026`)
	text = strings.ReplaceAll(text, "\u2028", `\u2028`)
	text = strings.ReplaceAll(text, "\u2029", `\u2029`)
	return text
}

// FrontendMode 返回 FRONTEND_MODE（legacy|next），默认 legacy。
func FrontendMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("FRONTEND_MODE")))
	if mode == "next" {
		return "next"
	}
	return "legacy"
}
