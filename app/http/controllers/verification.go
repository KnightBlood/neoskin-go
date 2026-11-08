package controllers

import (
	nethttp "net/http"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/model"
	"neoskin/internal/service"
)

// requireVerified 对应 PHP 的 `verified` 中间件（CheckUserVerified）。
//
// 当 `require_verification` 打开且当前用户尚未验证邮箱时返回 403 JSON，
// 与 PHP `abort(403, trans('auth.check.verified'))` 保持一致。
func requireVerified(ctx goravelhttp.Context, user *model.User, options *service.Option) goravelhttp.Response {
	if user == nil || user.Verified {
		return nil
	}
	if !options.Bool("require_verification", false) {
		return nil
	}
	return ctx.Response().Json(nethttp.StatusForbidden, goravelhttp.Json{
		"message": t(ctx, "auth.check.verified"),
	})
}

// requireVerifiedOnUserRoutes 仅作用于 `/user/*`：同一处理器也服务 `/api/*`，
// 而 Laravel 的 API 路由没有 `verified` 中间件。
func requireVerifiedOnUserRoutes(ctx goravelhttp.Context, user *model.User,
	options *service.Option) goravelhttp.Response {
	if !strings.HasPrefix(ctx.Request().Path(), "/user/") {
		return nil
	}
	return requireVerified(ctx, user, options)
}
