package middleware

import (
	nethttp "net/http"
	"strings"

	"github.com/goravel/framework/contracts/http"

	"neoskin/internal/auth"
)

// OAuthScope 对应 Passport 的 CheckTokenForAnyScope：Bearer token 至少命中一个 scope。
// 通过 session 登录的请求不受限制（等价于 TransientToken 的 "*"）。
type OAuthScope struct {
	Any []string
}

func RequireScopes(scopes ...string) http.Middleware {
	return &OAuthScope{Any: scopes}
}

func (m *OAuthScope) Handle(ctx http.Context) {
	_ = auth.Authenticate(ctx)
	if auth.HasAnyScope(ctx, m.Any...) {
		ctx.Request().Next()
		return
	}
	// 注意：Goravel 的中间件必须显式 Abort，否则后续 handler 仍会执行。
	_ = ctx.Response().Json(nethttp.StatusForbidden, http.Json{
		"message": "Invalid scope(s) provided.",
	}).Abort()
}

func (m *OAuthScope) Signature() string {
	return "neoskin.oauth.scope:" + strings.Join(m.Any, ",")
}
