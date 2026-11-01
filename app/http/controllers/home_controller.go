package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	nethttp "net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/auth"
	"neoskin/internal/i18n"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
	"neoskin/internal/view"
)

// HomeController 对应 HomeController@index 与前端语言包。
type HomeController struct{}

func NewHomeController() *HomeController {
	return &HomeController{}
}

// Index 处理 GET /。
func (r *HomeController) Index(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)
	locale := localeMiddleware.LocaleOf(ctx)
	translator := i18n.New(locale, repo.NewLanguageLineRepository(conn, dialect).Overrides(locale))
	users := repo.NewUserRepository(conn, dialect)
	user, _ := auth.CurrentUser(ctx, users)

	siteName := localizedOption(options, "site_name", locale, "Blessing Skin")
	navbarColor := options.Get("navbar_color")
	if navbarColor == "" {
		navbarColor = "cyan"
	}

	extra, _ := json.Marshal(map[string]any{
		"transparent_navbar": parseOptionBool(options.Get("transparent_navbar")),
	})

	data := view.HomeData{
		BaseData:          buildBaseData(ctx, options, locale, siteName),
		ExtraJSON:         template.JS(extra),
		NavbarColor:       navbarColor,
		ColorMode:         view.ColorMode(navbarColor),
		TransparentNavbar: parseOptionBool(options.Get("transparent_navbar")),
		FixedBG:           parseOptionBool(options.Get("fixed_bg")),
		HideIntro:         parseOptionBool(options.Get("hide_intro")),
		HomePicURL:        defaultString(options.Get("home_pic_url"), "./app/bg.webp"),
		Languages:         languageOptions(ctx, locale),
		CurrentLanguage:   i18n.Name(locale),
	}
	data.CustomCopyright = data.BaseData.CustomCopyright

	if user != nil {
		data.LoggedIn = true
		data.Nickname = user.Nickname
		if data.Nickname == "" {
			data.Nickname = user.Email
		}
		data.AvatarURL = fmt.Sprintf("/avatar/%d?size=36", user.Avatar)
		data.AvatarPNGURL = fmt.Sprintf("/avatar/%d?size=36&png=1", user.Avatar)
		data.IsAdmin = user.IsAdmin()
	}

	body, err := view.Render("home", data, translator)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Data(nethttp.StatusOK, "text/html; charset=utf-8", body)
}

// LangJS 处理 GET /lang/{locale}.js：生成 blessing.i18n = {...}。
func (r *HomeController) LangJS(ctx goravelhttp.Context) goravelhttp.Response {
	file := ctx.Request().Route("file")
	if !strings.HasSuffix(file, ".js") {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	locale := i18n.Canonical(strings.TrimSuffix(file, ".js"))
	translator := i18n.New(locale, nil)
	payload := translator.Group("front-end")

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	body := "blessing.i18n = " + strings.TrimSpace(buffer.String())
	return ctx.Response().Data(nethttp.StatusOK, "application/javascript; charset=utf-8",
		[]byte(body))
}

// localizedOption 读取 <key>_<locale>，回退到 <key>。
func localizedOption(options *service.Option, key, locale, fallback string) string {
	if value := options.Get(key + "_" + locale); value != "" {
		return value
	}
	return defaultString(options.Get(key), fallback)
}

func localizedOptionInt(options *service.Option, key, locale string, fallback int) int {
	if raw := options.Get(key + "_" + locale); raw != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			return parsed
		}
	}
	return options.Int(key, fallback)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func parseOptionBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

func faviconURL(value string) string {
	if value == "" {
		value = "app/favicon.ico"
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "/" + strings.TrimLeft(value, "/")
}

var metaTagPattern = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
var tagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

// metaTags 对应 Twig 的 striptags('<meta>')。
func metaTags(value string) string {
	return strings.Join(metaTagPattern.FindAllString(value, -1), "\n")
}

func stripTags(value string) string {
	return tagPattern.ReplaceAllString(value, "")
}

// safeInlineScript 避免自定义脚本提前闭合 <script>。
func safeInlineScript(value string) string {
	return strings.ReplaceAll(value, "</script", `<\/script`)
}

// customCopyright 对应 shared.copyright 的 {site_name}/{site_url}/{year} 替换。
func customCopyright(options *service.Option, locale, siteName, siteURL string, year int) string {
	text := localizedOption(options, "copyright_text", locale,
		`<b>Copyright &copy; {year} <a href="{site_url}">{site_name}</a>.</b> All rights reserved.`)
	return strings.NewReplacer(
		"{site_name}", siteName,
		"{site_url}", siteURL,
		"{year}", fmt.Sprint(year),
	).Replace(text)
}

func languageOptions(ctx goravelhttp.Context, _ string) []view.Language {
	queries := ctx.Request().Queries()
	path := ctx.Request().Path()
	host := ctx.Request().Host()
	result := make([]view.Language, 0, len(i18n.Locales))
	for _, locale := range i18n.Locales {
		values := url.Values{}
		for key, value := range queries {
			if key == "lang" {
				continue
			}
			values.Set(key, value)
		}
		values.Set("lang", locale.Code)
		result = append(result, view.Language{
			Name: locale.Name,
			URL:  "http://" + host + path + "?" + values.Encode(),
		})
	}
	return result
}
