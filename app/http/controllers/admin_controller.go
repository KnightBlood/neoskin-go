package controllers

import (
	"encoding/json"
	"fmt"
	"html/template"
	nethttp "net/http"
	"os"
	"runtime"
	"strings"
	"time"

	goravelhttp "github.com/goravel/framework/contracts/http"

	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/i18n"
	"neoskin/internal/plugin"
	"neoskin/internal/repo"
	"neoskin/internal/service"
	"neoskin/internal/view"
)

type AdminController struct{}

func NewAdminController() *AdminController {
	return &AdminController{}
}

func buildAdminMenu(translator *i18n.Translator, current string) []view.MenuItem {
	item := func(key, link, icon string) view.MenuItem {
		return view.MenuItem{
			Title: translator.Trans(key), Link: "/" + link, Icon: icon,
			Active: current == link || strings.HasPrefix(current, link+"/"),
		}
	}
	return []view.MenuItem{
		item("general.dashboard", "admin", "fa-tachometer-alt"),
		{Title: "通知与公告", Link: "/admin/communications", Icon: "fa-bullhorn",
			Active: strings.HasPrefix(current, "admin/communications")},
		item("general.user-manage", "admin/users", "fa-users"),
		item("general.player-manage", "admin/players", "fa-gamepad"),
		item("general.report-manage", "admin/reports", "fa-flag"),
		item("general.customize", "admin/customize", "fa-paint-brush"),
		item("general.i18n", "admin/i18n", "fa-globe"),
		item("general.score-options", "admin/score", "fa-credit-card"),
		item("general.options", "admin/options", "fa-cog"),
		item("general.res-options", "admin/resource", "fa-atom"),
		item("general.status", "admin/status", "fa-battery-three-quarters"),
		item("general.plugin-manage", "admin/plugins/manage", "fa-plug"),
		item("general.plugin-market", "admin/plugins/market", "fa-shopping-bag"),
		item("general.check-update", "admin/update", "fa-arrow-up"),
	}
}

// adminPage 构建管理端页面（admin.base）数据。
func adminPageData(ctx goravelhttp.Context, titleKey string,
	extra map[string]any) (*view.UserData, *i18n.Translator, goravelhttp.Response) {
	admin, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return nil, nil, resp
	}
	options := service.NewOption(conn, dialect)
	locale := localeMiddleware.LocaleOf(ctx)
	translator := i18n.New(locale, repo.NewLanguageLineRepository(conn, dialect).Overrides(locale))
	siteName := localizedOption(options, "site_name", locale, "Blessing Skin")
	navbarColor := options.Get("navbar_color")
	if navbarColor == "" {
		navbarColor = "cyan"
	}
	sidebarColor := options.Get("sidebar_color")
	if sidebarColor == "" {
		sidebarColor = "dark-maroon"
	}
	if admin.DarkMode {
		sidebarColor = strings.ReplaceAll(sidebarColor, "light", "dark")
	}
	extraJSON, _ := json.Marshal(extra)
	data := &view.UserData{
		BaseData:        buildBaseData(ctx, options, locale, siteName),
		Title:           translator.Trans(titleKey),
		DarkMode:        admin.DarkMode,
		NavbarColor:     navbarColor,
		ColorMode:       view.ColorMode(navbarColor),
		SidebarColor:    sidebarColor,
		Nickname:        admin.Nickname,
		Email:           admin.Email,
		AvatarURL:       fmt.Sprintf("/avatar/%d?size=45", admin.Avatar),
		AvatarPNGURL:    fmt.Sprintf("/avatar/%d?size=45&png=1", admin.Avatar),
		IsAdmin:         true,
		MenuItems:       buildAdminMenu(translator, strings.Trim(ctx.Request().Path(), "/")),
		Languages:       languageOptions(ctx, locale),
		CurrentLanguage: i18n.Name(locale),
		ExtraJSON:       template.JS(extraJSON),
	}
	data.MenuItems = append(data.MenuItems, pluginMenuItems("admin")...)
	badges := plugin.DefaultHook.UserBadges()
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventRenderingBadges, Payload: &badges})
	data.Badges = badges
	if data.Nickname == "" {
		data.Nickname = admin.Email
	}
	return data, translator, nil
}

func renderAdminPage(ctx goravelhttp.Context, name string, data *view.UserData,
	translator *i18n.Translator) goravelhttp.Response {
	body, err := view.Render(name, data, translator)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Data(nethttp.StatusOK, "text/html; charset=utf-8", body)
}

// IndexPage 处理 GET /admin：仪表盘。
func (r *AdminController) IndexPage(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	users := repo.NewUserRepository(conn, dialect)
	players := repo.NewPlayerRepository(conn, dialect)
	textures := repo.NewTextureRepository(conn, dialect)
	userCount, _ := users.CountAll()
	playerCount, _ := players.CountAll()
	textureCount, _ := textures.CountAll()
	storage, _ := textures.SumSize()

	data, translator, resp := adminPageData(ctx, "general.dashboard", nil)
	if resp != nil {
		return resp
	}
	data.StatsUsers = fmt.Sprint(userCount)
	data.StatsPlayers = fmt.Sprint(playerCount)
	data.StatsTextures = fmt.Sprint(textureCount)
	if storage > 1024 {
		data.StatsStorage = fmt.Sprintf("%.1fMB", float64(storage)/1024)
	} else {
		data.StatsStorage = fmt.Sprintf("%dKB", storage)
	}
	return renderAdminPage(ctx, "admin_index", data, translator)
}

// UsersPage 处理 GET /admin/users。
func (r *AdminController) UsersPage(ctx goravelhttp.Context) goravelhttp.Response {
	admin, _, _, _ := requireAdmin(ctx)
	data, translator, resp := adminPageData(ctx, "general.user-manage", map[string]any{
		"currentUser": userToJSON(admin),
	})
	if resp != nil {
		return resp
	}
	return renderAdminPage(ctx, "admin_users", data, translator)
}

// PlayersPage 处理 GET /admin/players。
func (r *AdminController) PlayersPage(ctx goravelhttp.Context) goravelhttp.Response {
	data, translator, resp := adminPageData(ctx, "general.player-manage", nil)
	if resp != nil {
		return resp
	}
	return renderAdminPage(ctx, "admin_players", data, translator)
}

// ReportsPage 处理 GET /admin/reports。
func (r *AdminController) ReportsPage(ctx goravelhttp.Context) goravelhttp.Response {
	data, translator, resp := adminPageData(ctx, "general.report-manage", nil)
	if resp != nil {
		return resp
	}
	return renderAdminPage(ctx, "admin_reports", data, translator)
}

// PluginsPage 处理 GET /admin/plugins/manage。
func (r *AdminController) PluginsPage(ctx goravelhttp.Context) goravelhttp.Response {
	data, translator, resp := adminPageData(ctx, "general.plugin-manage", nil)
	if resp != nil {
		return resp
	}
	return renderAdminPage(ctx, "admin_plugins", data, translator)
}

// MarketPage 处理 GET /admin/plugins/market。
func (r *AdminController) MarketPage(ctx goravelhttp.Context) goravelhttp.Response {
	data, translator, resp := adminPageData(ctx, "general.plugin-market", nil)
	if resp != nil {
		return resp
	}
	return renderAdminPage(ctx, "admin_market", data, translator)
}

// I18nPage 处理 GET /admin/i18n。
func (r *AdminController) I18nPage(ctx goravelhttp.Context) goravelhttp.Response {
	data, translator, resp := adminPageData(ctx, "general.i18n", nil)
	if resp != nil {
		return resp
	}
	data.Flash = pullFlash(ctx, "success")
	return renderAdminPage(ctx, "admin_i18n", data, translator)
}

// Index 处理 GET /admin（当前返回统计 JSON；模板后替换）。
func (r *AdminController) Index(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	users := repo.NewUserRepository(conn, dialect)
	players := repo.NewPlayerRepository(conn, dialect)
	textures := repo.NewTextureRepository(conn, dialect)
	userCount, _ := users.CountAll()
	playerCount, _ := players.CountAll()
	textureCount, _ := textures.CountAll()
	storage, _ := textures.SumSize()

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"sum": goravelhttp.Json{
			"users": userCount, "players": playerCount, "textures": textureCount,
			"storage": storage,
		},
	})
}

// Chart 处理 GET /admin/chart：最近 31 天的注册/上传曲线。
func (r *AdminController) Chart(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	location, _ := time.LoadLocation("Asia/Shanghai")
	today := time.Now().In(location)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)
	start := today.AddDate(0, 0, -30)

	users := repo.NewUserRepository(conn, dialect)
	textures := repo.NewTextureRepository(conn, dialect)
	registrations, _ := users.RegisterTimesSince(start.Format("2006-01-02 15:04:05"))
	uploads, _ := textures.UploadTimesSince(start.Format("2006-01-02 15:04:05"))
	registrationCounts := countByDay(registrations)
	uploadCounts := countByDay(uploads)

	labels := make([]string, 0, 31)
	userSeries := make([]int, 0, 31)
	textureSeries := make([]int, 0, 31)
	for i := 0; i < 31; i++ {
		day := start.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		labels = append(labels, day.Format("Monday"))
		userSeries = append(userSeries, registrationCounts[key])
		textureSeries = append(textureSeries, uploadCounts[key])
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"labels": []string{"User Registration", "Texture Uploads"},
		"xAxis":  labels,
		"data":   [][]int{userSeries, textureSeries},
	})
}

// Status 处理 GET /admin/status。
func (r *AdminController) Status(ctx goravelhttp.Context) goravelhttp.Response {
	_, _, _, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	host, _ := os.Hostname()
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"bs": goravelhttp.Json{
			"version": "6.0.11_patch",
			"env":     envOr("APP_ENV", "production"),
			"debug":   envOr("APP_DEBUG", "false"),
			"commit":  "",
			"laravel": "Goravel v1.18",
		},
		"server": goravelhttp.Json{
			"php": runtime.Version(),
			"web": "net/http",
			"os":  fmt.Sprintf("%s %s %s", runtime.GOOS, runtime.GOARCH, host),
		},
		"db": goravelhttp.Json{
			"type": envOr("DB_CONNECTION", "mysql"), "host": envOr("DB_HOST", "127.0.0.1"),
			"port": envOr("DB_PORT", ""), "username": envOr("DB_USERNAME", ""),
			"database": envOr("DB_DATABASE", ""), "prefix": envOr("DB_PREFIX", ""),
		},
		"plugins": []any{},
	})
}

func countByDay(values []string) map[string]int {
	result := map[string]int{}
	for _, value := range values {
		key := value
		if len(key) >= 10 {
			key = key[:10]
		}
		result[key]++
	}
	return result
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
