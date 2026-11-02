package controllers

import (
	"crypto/sha1"
	"fmt"
	nethttp "net/http"
	"strings"
	"time"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
	"neoskin/internal/laravel"
	"neoskin/internal/php"
	"neoskin/internal/repo"
	"neoskin/internal/service"
)

// Forgot 处理 GET /auth/forgot。
func (r *AuthController) Forgot(ctx goravelhttp.Context) goravelhttp.Response {
	if user, _, _, err := currentUser(ctx); err == nil && user != nil {
		return ctx.Response().Redirect(nethttp.StatusFound, "/user")
	}
	options := pageOptions()
	return renderAuthPage(ctx, "auth_forgot", "auth.forgot.title", map[string]any{
		"recaptcha": options.Get("recaptcha_sitekey"),
		"invisible": options.Bool("recaptcha_invisible", false),
	}, pullFlash(ctx, "msg"), "", "")
}

// HandleForgot 处理 POST /auth/forgot。
func (r *AuthController) HandleForgot(ctx goravelhttp.Context) goravelhttp.Response {
	email := strings.TrimSpace(ctx.Request().Input("email"))
	_, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)

	errs := map[string][]string{}
	if email == "" || !strings.Contains(email, "@") {
		errs["email"] = []string{"The email field is required."}
	}
	if !verifyCaptcha(ctx, options, ctx.Request().Input("captcha")) {
		errs["captcha"] = []string{"The captcha is incorrect."}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	ip := ctx.Request().Ip()
	cacheKey := fmt.Sprintf("%x", sha1.Sum([]byte("last_mail_"+ip)))
	if last := facades.Cache().GetInt(cacheKey, 0); last > 0 {
		if remain := 180 + last - int(time.Now().Unix()); remain > 0 {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 2, "message": t(ctx, "auth.forgot.frequent-mail"),
			})
		}
	}

	user, err := repo.NewUserRepository(conn, dialect).FindByEmail(email)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "auth.forgot.unregistered"),
		})
	}

	expires := time.Now().Add(time.Hour)
	signed, err := laravel.SignedPath("/auth/reset/"+fmt.Sprint(user.UID),
		map[string]string{"uid": fmt.Sprint(user.UID)}, &expires)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	url := "http://" + ctx.Request().Host() + signed
	siteName := service.NewOption(conn, dialect).Get("site_name")

	body := "<p>您收到这封邮件，是因为有人在 " + siteName + " 的密码重置功能中使用了您的地址。</p>" +
		`<p>点击此链接重置您的密码：<a href="` + url + `">` + url + `</a></p>` +
		"<p>如果您并没有访问过我们的网站，或没有进行上述操作，请忽略这封邮件。</p>"
	if err := sendMail(email, "重置您在 "+siteName+" 上的账户密码", body); err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 2, "message": t(ctx, "auth.forgot.failed", "msg", err.Error()),
		})
	}
	_ = facades.Cache().Put(cacheKey, int(time.Now().Unix()), time.Hour)
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "auth.forgot.success"),
	})
}

// Reset 处理 GET /auth/reset/{uid}。
func (r *AuthController) Reset(ctx goravelhttp.Context) goravelhttp.Response {
	if !validSignedRequest(ctx) {
		return ctx.Response().Json(nethttp.StatusForbidden, goravelhttp.Json{
			"message": t(ctx, "auth.reset.invalid"),
		})
	}
	_, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	user, err := repo.NewUserRepository(conn, dialect).FindByID(ctx.Request().RouteInt("uid"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	return renderAuthPage(ctx, "auth_reset", "auth.reset.title", map[string]any{},
		"", "", user.Nickname)
}

// HandleReset 处理 POST /auth/reset/{uid}。
func (r *AuthController) HandleReset(ctx goravelhttp.Context) goravelhttp.Response {
	if !validSignedRequest(ctx) {
		return ctx.Response().Json(nethttp.StatusForbidden, goravelhttp.Json{
			"message": t(ctx, "auth.reset.invalid"),
		})
	}
	password := ctx.Request().Input("password")
	if len(password) < 8 || len(password) > 32 {
		return validationFailed(ctx, map[string][]string{
			"password": {"The password must be between 8 and 32 characters."},
		})
	}
	_, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	users := repo.NewUserRepository(conn, dialect)
	user, err := users.FindByID(ctx.Request().RouteInt("uid"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	hashed, err := php.HashPassword(password, 0)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if err := users.UpdatePassword(user.UID, hashed); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "auth.reset.success"),
	})
}

// Verify 处理 GET /auth/verify/{user}。
func (r *AuthController) Verify(ctx goravelhttp.Context) goravelhttp.Response {
	_, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if !service.NewOption(conn, dialect).Bool("require_verification", false) {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.verification.disabled"),
		})
	}
	if !validSignedRequest(ctx) {
		return ctx.Response().Json(nethttp.StatusForbidden, goravelhttp.Json{
			"message": t(ctx, "auth.verify.invalid"),
		})
	}
	return renderAuthPage(ctx, "auth_verify", "auth.verify.title", map[string]any{},
		"", pullFlash(ctx, "errorMessage"), "")
}

// HandleVerify 处理 POST /auth/verify/{user}。
func (r *AuthController) HandleVerify(ctx goravelhttp.Context) goravelhttp.Response {
	if !validSignedRequest(ctx) {
		return ctx.Response().Json(nethttp.StatusForbidden, goravelhttp.Json{
			"message": t(ctx, "auth.verify.invalid"),
		})
	}
	email := strings.TrimSpace(ctx.Request().Input("email"))
	if email == "" || !strings.Contains(email, "@") {
		return validationFailed(ctx, map[string][]string{
			"email": {"The email field is required."},
		})
	}
	_, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	users := repo.NewUserRepository(conn, dialect)
	user, err := users.FindByID(ctx.Request().RouteInt("user"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	if !strings.EqualFold(user.Email, email) {
		return ctx.Response().Json(nethttp.StatusUnprocessableEntity, goravelhttp.Json{
			"message": t(ctx, "auth.verify.not-matched"),
			"errors": goravelhttp.Json{
				"email": []string{"邮箱不匹配"},
			},
		})
	}
	if err := users.UpdateVerified(user.UID, true); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Redirect(nethttp.StatusFound, "/user")
}

// BindPage 处理 GET /auth/bind。
func (r *AuthController) BindPage(ctx goravelhttp.Context) goravelhttp.Response {
	user, _, _, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}
	if user.Email != "" {
		return ctx.Response().Redirect(nethttp.StatusFound, "/user")
	}
	return renderAuthPage(ctx, "auth_bind", "auth.bind.title", map[string]any{},
		"", pullFlash(ctx, "errorMessage"), "")
}

// Bind 处理 POST /auth/bind（AuthController@fillEmail）。
func (r *AuthController) Bind(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}
	if user.Email != "" {
		return ctx.Response().Redirect(nethttp.StatusFound, "/user")
	}
	email := strings.TrimSpace(ctx.Request().Input("email"))
	if email == "" || !strings.Contains(email, "@") {
		return validationFailed(ctx, map[string][]string{
			"email": {"The email field is required."},
		})
	}
	users := repo.NewUserRepository(conn, dialect)
	if count, _ := users.CountByEmail(email); count > 0 {
		return validationFailed(ctx, map[string][]string{
			"email": {"The email has already been taken."},
		})
	}
	if err := users.UpdateEmail(user.UID, email); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Redirect(nethttp.StatusFound, "/user")
}

// SendVerification 处理 POST /user/email-verification。
func (r *AuthController) SendVerification(ctx goravelhttp.Context) goravelhttp.Response {
	user, conn, dialect, err := currentUser(ctx)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}
	options := service.NewOption(conn, dialect)
	if !options.Bool("require_verification", false) {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.verification.disabled"),
		})
	}
	session := ctx.Request().Session()
	last := 0
	if session != nil {
		if value := session.Get("last_mail_time"); value != nil {
			_, _ = fmt.Sscan(fmt.Sprint(value), &last)
		}
	}
	if remain := 60 + last - int(time.Now().Unix()); remain > 0 {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.verification.frequent-mail"),
		})
	}
	if user.Verified {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "user.verification.verified"),
		})
	}

	signed, err := laravel.SignedPath("/auth/verify/"+fmt.Sprint(user.UID),
		map[string]string{"user": fmt.Sprint(user.UID)}, nil)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	url := "http://" + ctx.Request().Host() + signed
	siteName := options.Get("site_name")
	body := "<p>请点击下面的链接验证您的邮箱地址：</p>" +
		`<p><a href="` + url + `">` + url + `</a></p>` +
		"<p>如果您并没有注册 " + siteName + "，请忽略这封邮件。</p>"
	if err := sendMail(user.Email, "验证您在 "+siteName+" 上的邮箱", body); err != nil {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 2, "message": t(ctx, "user.verification.failed", "msg", err.Error()),
		})
	}
	if session != nil {
		session.Put("last_mail_time", int(time.Now().Unix()))
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "user.verification.success"),
	})
}

// validSignedRequest 校验 Laravel 相对签名 URL（hasValidSignature(false)）。
func validSignedRequest(ctx goravelhttp.Context) bool {
	path := "/" + strings.TrimPrefix(ctx.Request().Path(), "/")
	raw := ""
	if origin := ctx.Request().Origin(); origin != nil && origin.URL != nil {
		raw = origin.URL.RawQuery
	}
	return laravel.ValidSignature(path, raw, time.Now())
}

// consumeCaptcha 校验并消费 session 中的验证码。
func consumeCaptcha(ctx goravelhttp.Context, input string) bool {
	expected := ""
	if session := ctx.Request().Session(); session != nil {
		if value := session.Pull("captcha"); value != nil {
			expected = fmt.Sprint(value)
		}
	}
	return expected != "" && strings.EqualFold(expected, input)
}
