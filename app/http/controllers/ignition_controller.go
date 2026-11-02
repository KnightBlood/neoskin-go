package controllers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
)

// IgnitionController 对应 spatie/ignition 的三个管理端点。
// 与 .NET 参考实现一致：仅 APP_DEBUG 且 APP_ENV 为 local/development 时启用；
// 没有可执行解决方案注册表，因此 execute-solution 恒返回 404。
type IgnitionController struct{}

func NewIgnitionController() *IgnitionController {
	return &IgnitionController{}
}

func ignitionEnabled() bool {
	if !facades.Config().GetBool("app.debug", false) {
		return false
	}
	env := strings.ToLower(facades.Config().GetString("app.env", "production"))
	return env == "local" || env == "development"
}

// HealthCheck 处理 GET /_ignition/health-check。
func (r *IgnitionController) HealthCheck(ctx goravelhttp.Context) goravelhttp.Response {
	if !ignitionEnabled() {
		return ctx.Response().String(http.StatusForbidden, "Forbidden")
	}
	return ctx.Response().Json(http.StatusOK, goravelhttp.Json{
		"can_execute_commands": false,
	})
}

// ExecuteSolution 处理 POST /_ignition/execute-solution。
func (r *IgnitionController) ExecuteSolution(ctx goravelhttp.Context) goravelhttp.Response {
	if !ignitionEnabled() {
		return ctx.Response().String(http.StatusForbidden, "Forbidden")
	}
	if strings.TrimSpace(ctx.Request().Input("solution")) == "" {
		return validationFailed(ctx, map[string][]string{
			"solution": {vRequired(ctx, "solution")},
		})
	}
	return ctx.Response().Json(http.StatusNotFound, goravelhttp.Json{
		"message": "Solution could not be found",
	})
}

// UpdateConfig 处理 POST /_ignition/update-config。
func (r *IgnitionController) UpdateConfig(ctx goravelhttp.Context) goravelhttp.Response {
	if !ignitionEnabled() {
		return ctx.Response().String(http.StatusForbidden, "Forbidden")
	}
	theme := strings.TrimSpace(ctx.Request().Input("theme"))
	editor := strings.TrimSpace(ctx.Request().Input("editor"))
	hideSolutions := ctx.Request().Input("hide_solutions")

	errs := map[string][]string{}
	if theme == "" {
		errs["theme"] = []string{vRequired(ctx, "theme")}
	} else if theme != "light" && theme != "dark" && theme != "auto" {
		errs["theme"] = []string{vIn(ctx, "theme")}
	}
	if editor == "" {
		errs["editor"] = []string{vRequired(ctx, "editor")}
	}
	if hideSolutions == "" {
		errs["hide_solutions"] = []string{vRequired(ctx, "hide_solutions")}
	} else if !validBooleanValue(hideSolutions) {
		errs["hide_solutions"] = []string{vBoolean(ctx, "hide_solutions")}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ctx.Response().Json(http.StatusOK, false)
	}
	payload, _ := json.Marshal(map[string]any{
		"theme": theme, "editor": editor,
		"hide_solutions": parseBoolValue(hideSolutions),
	})
	if err := os.WriteFile(filepath.Join(home, ".ignition.json"), payload, 0o644); err != nil {
		return ctx.Response().Json(http.StatusOK, false)
	}
	return ctx.Response().Json(http.StatusOK, true)
}

func parseBoolValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

func validBooleanValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "0", "true", "false":
		return true
	}
	return false
}
