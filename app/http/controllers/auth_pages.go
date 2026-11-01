package controllers

import (
	"encoding/json"
	"fmt"
	"html/template"
	nethttp "net/http"

	goravelhttp "github.com/goravel/framework/contracts/http"

	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/i18n"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
	"neoskin/internal/view"
)

// renderAuthPage 渲染 auth-* 页面（登录/注册/找回/绑定/验证）。
func renderAuthPage(ctx goravelhttp.Context, page, titleKey string,
	extra map[string]any, message, errMessage, nickname string) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)
	locale := localeMiddleware.LocaleOf(ctx)
	translator := i18n.New(locale, repo.NewLanguageLineRepository(conn, dialect).Overrides(locale))
	siteName := localizedOption(options, "site_name", locale, "Blessing Skin")

	extraJSON, _ := json.Marshal(extra)
	data := view.AuthData{
		BaseData:  buildBaseData(ctx, options, locale, siteName),
		Title:     translator.Trans(titleKey),
		Message:   message,
		ExtraJSON: template.JS(extraJSON),
		Error:     errMessage,
		Nickname:  nickname,
	}
	body, err := view.Render(page, data, translator)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Data(nethttp.StatusOK, "text/html; charset=utf-8", body)
}

// pullFlash 读取并移除 session 中的一次性消息。
func pullFlash(ctx goravelhttp.Context, key string) string {
	session := ctx.Request().Session()
	if session == nil {
		return ""
	}
	value := session.Pull(key)
	if value == nil {
		return ""
	}
	text := fmt.Sprint(value)
	if text == "<nil>" {
		return ""
	}
	return text
}

// pageOptions 返回 options 服务；数据库不可用时返回带默认值的空配置。
func pageOptions() *service.Option {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return service.NewOption(nil, nil)
	}
	return service.NewOption(conn, dialect)
}
