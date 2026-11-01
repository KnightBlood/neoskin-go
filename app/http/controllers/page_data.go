package controllers

import (
	"encoding/json"
	"html/template"
	"strings"
	"time"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
	"neoskin/internal/plugin"
	"neoskin/internal/service"
	"neoskin/internal/view"
)

func pluginMenuItems(category string) []view.MenuItem {
	items := plugin.DefaultHook.MenuItems(category)
	result := make([]view.MenuItem, 0, len(items))
	for _, item := range items {
		result = append(result, view.MenuItem{
			Title: item.Title, Link: item.Link, Icon: item.Icon,
		})
	}
	eventName := map[string]string{
		"user":    plugin.EventConfigureUserMenu,
		"admin":   plugin.EventConfigureAdminMenu,
		"explore": plugin.EventConfigureExploreMenu,
	}[category]
	if eventName != "" {
		plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: eventName, Payload: &result})
	}
	return result
}

// buildBaseData 组装页面公共 head/foot 数据。
func buildBaseData(ctx goravelhttp.Context, options *service.Option, locale, siteName string) view.BaseData {
	cdnBase := options.Get("cdn_address")
	blessing, _ := json.Marshal(map[string]any{
		"version":   facades.Config().GetString("app.version", "6.0.11_patch"),
		"locale":    locale,
		"base_url":  "http://" + ctx.Request().Host(),
		"site_name": siteName,
		"route":     strings.Trim(ctx.Request().Path(), "/"),
	})

	header := []string{}
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventRenderingHeader, Payload: &header})
	footer := []string{}
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventRenderingFooter, Payload: &footer})

	base := view.BaseData{
		Locale:          locale,
		SiteName:        siteName,
		SiteDescription: localizedOption(options, "site_description", locale, ""),
		CSRF:            csrfToken(ctx),
		ThemeColor:      view.ThemeColor(options.Get("navbar_color")),
		Keywords:        options.Get("meta_keywords"),
		Description:     options.Get("meta_description"),
		MetaExtra:       template.HTML(metaTags(options.Get("meta_extras"))),
		BlessingJSON:    template.JS(blessing),
		StyleCSS:        view.AssetURL(cdnBase, view.Asset("style.", ".css", "style.1dd8a8a.css")),
		HomeCSS:         view.AssetURL(cdnBase, view.Asset("home-css.", ".css", "home-css.6c76352.css")),
		SpectreCSS:      view.AssetURL(cdnBase, view.Asset("spectre.", ".css", "spectre.e885dd0.css")),
		AppJS:           view.AssetURL(cdnBase, view.Asset("app.", ".js", "app.7278b90.js")),
		I18nURL:         "/lang/" + locale + ".js",
		Favicon:         faviconURL(options.Get("favicon_url")),
		CustomCSS:       template.HTML(stripTags(options.Get("custom_css"))),
		CustomJS:        template.HTML(safeInlineScript(options.Get("custom_js"))),
		PluginStyles:    plugin.DefaultHook.StylesFor("*"),
		PluginScripts:   plugin.DefaultHook.ScriptsFor("*"),
		ExtraHead:       template.HTML(strings.Join(header, "\n")),
		ExtraFoot:       template.HTML(strings.Join(footer, "\n")),

		CopyrightPreference: localizedOptionInt(options, "copyright_prefer", locale, 0),
		SiteURL:             options.Get("site_url"),
		Year:                time.Now().Year(),
	}
	base.CustomCopyright = template.HTML(customCopyright(options, locale, siteName,
		base.SiteURL, base.Year))
	return base
}
