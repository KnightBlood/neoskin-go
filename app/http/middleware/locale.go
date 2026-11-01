package middleware

import (
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"

	"neoskin/internal/i18n"
)

type localeKey struct{}

// Locale 复刻 DetectLanguagePrefer：lang 参数 → cookie → Accept-Language → 默认值，
// 并回写 locale cookie（120 分钟）。
type Locale struct {
	Default string
}

func DetectLocale(defaultLocale string) http.Middleware {
	return &Locale{Default: defaultLocale}
}

func (m *Locale) Handle(ctx http.Context) {
	locale := strings.TrimSpace(ctx.Request().Input("lang"))
	if locale == "" {
		locale = ctx.Request().Cookie("locale")
	}
	if locale == "" {
		locale = preferredLanguage(ctx.Request().Header("Accept-Language"))
	}
	if locale == "" {
		locale = m.Default
	}
	locale = i18n.Canonical(locale)

	ctx.WithValue(localeKey{}, locale)
	ctx.Response().Cookie(http.Cookie{
		Name: "locale", Value: locale, Path: "/",
		Expires: time.Now().Add(120 * time.Minute),
	})
	ctx.Request().Next()
}

func (m *Locale) Signature() string {
	return "neoskin:detect_locale"
}

// LocaleOf 返回当前请求的 locale。
func LocaleOf(ctx http.Context) string {
	if value := ctx.Value(localeKey{}); value != nil {
		if locale, ok := value.(string); ok && locale != "" {
			return locale
		}
	}
	return "zh_CN"
}

func preferredLanguage(header string) string {
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(strings.Split(part, ";")[0])
		if tag == "" {
			continue
		}
		normalized := strings.ReplaceAll(tag, "-", "_")
		switch {
		case strings.HasPrefix(normalized, "zh_Hans"):
			return "zh_CN"
		case strings.HasPrefix(normalized, "zh_Hant"):
			return "zh_TW"
		case strings.HasPrefix(normalized, "zh"):
			return "zh_CN"
		case strings.HasPrefix(normalized, "en"):
			return "en"
		case strings.HasPrefix(normalized, "es"):
			return "es_ES"
		case strings.HasPrefix(normalized, "ru"):
			return "ru_RU"
		case strings.HasPrefix(normalized, "de"):
			return "de_DE"
		}
	}
	return ""
}
