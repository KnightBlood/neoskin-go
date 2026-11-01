package controllers

import (
	"fmt"
	nethttp "net/http"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
	localeMiddleware "neoskin/app/http/middleware"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
)

type AdminOptionsController struct{}

func NewAdminOptionsController() *AdminOptionsController {
	return &AdminOptionsController{}
}

var optionBoolFields = map[string]bool{
	"transparent_navbar":              true,
	"hide_intro":                      true,
	"fixed_bg":                        true,
	"register_with_player_name":       true,
	"require_verification":            true,
	"auto_del_invalid_texture":        true,
	"allow_downloading_texture":       true,
	"recaptcha_invisible":             true,
	"return_score":                    true,
	"sign_after_zero":                 true,
	"take_back_scores_after_deletion": true,
	"enable_avatar_cache":             true,
	"enable_preview_cache":            true,
}

// Options 处理 GET|POST /admin/options。
func (r *AdminOptionsController) Options(ctx goravelhttp.Context) goravelhttp.Response {
	return r.handle(ctx, []string{
		"site_name", "site_description", "site_url", "register_with_player_name",
		"require_verification", "regs_per_ip", "max_upload_file_size", "max_texture_width",
		"player_name_rule", "custom_player_name_regexp", "player_name_length_min",
		"player_name_length_max", "auto_del_invalid_texture", "allow_downloading_texture",
		"status_code_for_private", "texture_name_regexp", "content_policy",
		"meta_keywords", "meta_description", "meta_extras", "recaptcha_sitekey",
		"recaptcha_secretkey", "recaptcha_invisible", "update_source",
		"plugin_market_source",
	})
}

// OptionsPage 处理 GET /admin/options（POST 仍由 Options 保存）。
func (r *AdminOptionsController) OptionsPage(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)
	locale := localeMiddleware.LocaleOf(ctx)

	if ctx.Request().Method() == nethttp.MethodPost {
		for _, key := range []string{"site_name", "site_description", "content_policy"} {
			if value := ctx.Request().Input(key); value != "" {
				_ = options.Set(key+"_"+locale, value)
			}
		}
		return r.Options(ctx)
	}

	data, translator, resp := adminPageData(ctx, "general.options", nil)
	if resp != nil {
		return resp
	}
	keys := []string{
		"site_name", "site_description", "site_url", "register_with_player_name",
		"require_verification", "regs_per_ip", "max_upload_file_size", "max_texture_width",
		"player_name_rule", "custom_player_name_regexp", "player_name_length_min",
		"player_name_length_max", "auto_del_invalid_texture", "allow_downloading_texture",
		"status_code_for_private", "texture_name_regexp", "content_policy",
		"meta_keywords", "meta_description", "meta_extras", "recaptcha_sitekey",
		"recaptcha_secretkey", "recaptcha_invisible", "update_source", "plugin_market_source",
	}
	data.Options = map[string]string{}
	for _, key := range keys {
		data.Options[key] = options.Get(key)
	}
	if data.Options["update_source"] == "" {
		data.Options["update_source"] = facades.Config().GetString("app.update_source")
	}
	return renderAdminPage(ctx, "admin_options", data, translator)
}

// Customize 处理 GET|POST /admin/customize。
func (r *AdminOptionsController) Customize(ctx goravelhttp.Context) goravelhttp.Response {
	return r.handle(ctx, []string{
		"home_pic_url", "favicon_url", "transparent_navbar", "hide_intro", "fixed_bg",
		"copyright_prefer", "copyright_text", "custom_css", "custom_js",
		"navbar_color", "sidebar_color",
	})
}

// CustomizePage 处理 GET /admin/customize（POST 仍由 Customize 保存）。
func (r *AdminOptionsController) CustomizePage(ctx goravelhttp.Context) goravelhttp.Response {
	if ctx.Request().Method() == nethttp.MethodPost {
		return r.Customize(ctx)
	}
	data, translator, resp := adminPageData(ctx, "general.customize", nil)
	if resp != nil {
		return resp
	}
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)
	keys := []string{
		"home_pic_url", "favicon_url", "transparent_navbar", "hide_intro", "fixed_bg",
		"copyright_prefer", "copyright_text", "custom_css", "custom_js",
		"navbar_color", "sidebar_color",
	}
	data.Options = map[string]string{}
	for _, key := range keys {
		data.Options[key] = options.Get(key)
	}
	data.NavbarColors = []string{
		"primary", "secondary", "success", "danger", "indigo", "purple", "pink",
		"teal", "cyan", "dark", "gray", "fuchsia", "maroon", "olive", "navy",
		"lime", "light", "warning", "white", "orange",
	}
	data.SidebarColors = []string{
		"primary", "warning", "info", "danger", "success", "indigo", "navy",
		"purple", "fuchsia", "pink", "maroon", "orange", "lime", "teal", "olive",
	}
	return renderAdminPage(ctx, "admin_customize", data, translator)
}

// Score 处理 GET|POST /admin/score。
func (r *AdminOptionsController) Score(ctx goravelhttp.Context) goravelhttp.Response {
	return r.handle(ctx, []string{
		"score_per_storage", "private_score_per_storage", "score_per_closet_item",
		"return_score", "score_per_player", "user_initial_score",
		"reporter_score_modification", "reporter_reward_score",
		"sign_gap_time", "sign_after_zero", "score_award_per_texture",
		"take_back_scores_after_deletion", "score_award_per_like",
	})
}

// ScorePage 处理 GET /admin/score（POST 仍由 Score 保存）。
func (r *AdminOptionsController) ScorePage(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)

	if ctx.Request().Method() == nethttp.MethodPost {
		from := ctx.Request().Input("sign_score_from")
		to := ctx.Request().Input("sign_score_to")
		if from != "" && to != "" {
			if err := options.Set("sign_score", from+","+to); err != nil {
				return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
			}
		}
		return r.Score(ctx)
	}

	data, translator, resp := adminPageData(ctx, "general.score-options", nil)
	if resp != nil {
		return resp
	}
	keys := []string{
		"score_per_storage", "private_score_per_storage", "score_per_closet_item",
		"return_score", "score_per_player", "user_initial_score",
		"reporter_score_modification", "reporter_reward_score",
		"sign_gap_time", "sign_after_zero", "score_award_per_texture",
		"take_back_scores_after_deletion", "score_award_per_like",
	}
	data.Options = map[string]string{}
	for _, key := range keys {
		data.Options[key] = options.Get(key)
	}
	parts := strings.Split(options.Get("sign_score"), ",")
	if len(parts) == 2 {
		data.Options["sign_score_from"] = strings.TrimSpace(parts[0])
		data.Options["sign_score_to"] = strings.TrimSpace(parts[1])
	}
	return renderAdminPage(ctx, "admin_score", data, translator)
}

// Resource 处理 GET|POST /admin/resource。
func (r *AdminOptionsController) Resource(ctx goravelhttp.Context) goravelhttp.Response {
	return r.handle(ctx, []string{
		"force_ssl", "auto_detect_asset_url",
		"cdn_address", "cache_expire_time", "enable_avatar_cache", "enable_preview_cache",
		"allow_downloading_texture",
	})
}

// ResourcePage 处理 GET /admin/resource（POST 仍由 Resource 保存）。
func (r *AdminOptionsController) ResourcePage(ctx goravelhttp.Context) goravelhttp.Response {
	if ctx.Request().Method() == nethttp.MethodPost {
		return r.Resource(ctx)
	}
	data, translator, resp := adminPageData(ctx, "general.res-options", nil)
	if resp != nil {
		return resp
	}
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)
	keys := []string{
		"force_ssl", "auto_detect_asset_url", "cache_expire_time", "cdn_address",
		"enable_avatar_cache", "enable_preview_cache",
	}
	data.Options = map[string]string{}
	for _, key := range keys {
		data.Options[key] = options.Get(key)
	}
	return renderAdminPage(ctx, "admin_resource", data, translator)
}

func (r *AdminOptionsController) handle(ctx goravelhttp.Context, keys []string) goravelhttp.Response {
	_, conn, dialect, resp := requireAdmin(ctx)
	if resp != nil {
		return resp
	}
	options := service.NewOption(conn, dialect)

	if ctx.Request().Method() != nethttp.MethodPost {
		values := make(goravelhttp.Json, len(keys))
		for _, key := range keys {
			values[key] = options.Get(key)
		}
		return ctx.Response().Json(nethttp.StatusOK, values)
	}

	input := ctx.Request().All()
	for _, key := range keys {
		if optionBoolFields[key] {
			value := "false"
			if raw, ok := input[key]; ok && truthy(raw) {
				value = "true"
			}
			if err := options.Set(key, value); err != nil {
				return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
			}
			continue
		}

		raw, ok := input[key]
		if !ok {
			continue
		}
		value := strings.TrimSpace(toString(raw))
		if key == "site_url" {
			value = strings.TrimSuffix(value, "/")
			value = strings.TrimSuffix(value, "/index.php")
		}
		if err := options.Set(key, value); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
	}

	// 签到分数是 sign_score_from + sign_score_to 组合。
	if from, ok := input["sign_score_from"]; ok {
		to := input["sign_score_to"]
		if err := options.Set("sign_score", toString(from)+","+toString(to)); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "Options saved.",
	})
}

func truthy(value any) bool {
	text := strings.ToLower(strings.TrimSpace(toString(value)))
	return text == "1" || text == "true" || text == "on" || text == "yes"
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
