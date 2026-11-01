package controllers

import (
	"sync"

	goravelhttp "github.com/goravel/framework/contracts/http"

	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/i18n"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
)

type translatorCtxKey struct{}

// translatorFor 返回当前请求的翻译器（含 language_lines 覆盖），并按请求缓存。
func translatorFor(ctx goravelhttp.Context) *i18n.Translator {
	if value := ctx.Value(translatorCtxKey{}); value != nil {
		if translator, ok := value.(*i18n.Translator); ok {
			return translator
		}
	}
	locale := localeMiddleware.LocaleOf(ctx)
	var overrides map[string]string
	if conn, dialect, err := runtime.Connection(); err == nil {
		overrides = repo.NewLanguageLineRepository(conn, dialect).Overrides(locale)
	}
	translator := i18n.New(locale, overrides)
	ctx.WithValue(translatorCtxKey{}, translator)
	return translator
}

// t 按 "group.key" 读取语言包，参数为 key/value 交替（如 "name", "Steve"）。
func t(ctx goravelhttp.Context, key string, params ...string) string {
	pairs := make(map[string]string, len(params)/2)
	for index := 0; index+1 < len(params); index += 2 {
		pairs[params[index]] = params[index+1]
	}
	return translatorFor(ctx).Trans(key, pairs)
}

var _ sync.Locker // 保留 sync 导入供未来缓存扩展
