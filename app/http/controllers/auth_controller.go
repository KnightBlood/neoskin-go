package controllers

import (
	"fmt"
	nethttp "net/http"
	"regexp"
	"strings"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/auth"
	"neoskin/internal/captcha"
	"neoskin/internal/model"
	"neoskin/internal/php"
	"neoskin/internal/plugin"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
	"neoskin/internal/service"
)

type AuthController struct{}

func NewAuthController() *AuthController {
	return &AuthController{}
}

// LoginPage 渲染登录页（auth.login）。
func (r *AuthController) LoginPage(ctx goravelhttp.Context) goravelhttp.Response {
	if user, _, _, err := currentUser(ctx); err == nil && user != nil {
		return ctx.Response().Redirect(nethttp.StatusFound, "/user")
	}
	options := pageOptions()
	tooManyFails := captcha.LoginFails().Count(ctx.Request().Ip()) > captchaRequiredFails
	return renderAuthPage(ctx, "auth_login", "auth.login.title", map[string]any{
		"tooManyFails": tooManyFails,
		"recaptcha":    options.Get("recaptcha_sitekey"),
		"invisible":    options.Bool("recaptcha_invisible", false),
	}, pullFlash(ctx, "msg"), "", "")
}

// Login 处理 POST /auth/login（JSON / form / query）。
func (r *AuthController) Login(ctx goravelhttp.Context) goravelhttp.Response {
	identification := strings.TrimSpace(ctx.Request().Input("identification"))
	password := ctx.Request().Input("password")

	errs := map[string][]string{}
	if identification == "" {
		errs["identification"] = []string{vRequired(ctx, "identification")}
	}
	if password == "" {
		errs["password"] = []string{vRequired(ctx, "password")}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	users := repo.NewUserRepository(conn, dialect)
	players := repo.NewPlayerRepository(conn, dialect)

	// PHP：同一 IP 登录失败超过 3 次后要求验证码（本地验证码或 reCAPTCHA）。
	loginFails := captcha.LoginFails().Count(ctx.Request().Ip())
	if loginFails > captchaRequiredFails {
		options := service.NewOption(conn, dialect)
		if !verifyCaptcha(ctx, options, ctx.Request().Input("captcha")) {
			return validationFailed(ctx, map[string][]string{
				"captcha": {vCaptcha(ctx, options)},
			})
		}
	}

	var user *model.User
	if strings.Contains(identification, "@") {
		user, err = users.FindByEmail(identification)
	} else {
		player, findErr := players.FindDetailByName(identification)
		err = findErr
		if player != nil {
			if player.Permission == model.PermissionBanned {
				return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
					"code": 1, "message": t(ctx, "auth.check.banned"),
					"data": goravelhttp.Json{"login_fails": loginFails},
				})
			}
			user, err = users.FindByID(player.UID)
		}
	}
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	if user == nil {
		// PHP：账号不存在返回 code 2 + auth.validation.user，且不计入失败次数。
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 2, "message": t(ctx, "auth.validation.user"),
		})
	}
	if !php.VerifyPassword(user.Password, password) {
		fails := captcha.LoginFails().Increment(ctx.Request().Ip())
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "auth.validation.password"),
			"data": goravelhttp.Json{"login_fails": fails},
		})
	}
	if user.Permission == model.PermissionBanned {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "auth.check.banned"),
		})
	}

	if err := auth.Login(ctx, users, user.UID, truthy(ctx.Request().Input("keep"))); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	// PHP：登录成功后清空失败计数（Session::forget + Cache::forget）。
	captcha.LoginFails().Reset(ctx.Request().Ip())
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserLoggedIn, Payload: user.UID})
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserAuthenticated, Payload: user.UID})

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "auth.login.success"),
		"data": goravelhttp.Json{"uid": user.UID, "redirectTo": "/user"},
	})
}

// Logout 处理 POST /auth/logout。
func (r *AuthController) Logout(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	users := repo.NewUserRepository(conn, dialect)
	if err := auth.Logout(ctx, users); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "auth.logout.success"),
	})
}

type userJSON struct {
	UID          int    `json:"uid"`
	Email        string `json:"email"`
	Nickname     string `json:"nickname"`
	Locale       string `json:"locale"`
	Score        int    `json:"score"`
	Avatar       int    `json:"avatar"`
	IP           string `json:"ip"`
	DarkMode     bool   `json:"is_dark_mode"`
	Permission   int    `json:"permission"`
	LastSignAt   string `json:"last_sign_at"`
	RegisterAt   string `json:"register_at"`
	Verified     bool   `json:"verified"`
	Verification string `json:"verification_token"`
}

// User 处理 GET /api/user。
func (r *AuthController) User(ctx goravelhttp.Context) goravelhttp.Response {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	users := repo.NewUserRepository(conn, dialect)
	user, err := auth.CurrentUser(ctx, users)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if user == nil {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"message": "Unauthenticated.",
		})
	}

	return ctx.Response().Json(nethttp.StatusOK, userToJSON(user))
}

// Captcha 生成验证码图片并写入 session（PHP 源为 JPEG）。
func (r *AuthController) Captcha(ctx goravelhttp.Context) goravelhttp.Response {
	phrase, image, err := captcha.Generate()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	if session := ctx.Request().Session(); session != nil {
		session.Put("captcha", phrase)
	}

	return ctx.Response().Data(nethttp.StatusOK, "image/jpeg", image)
}

// RegisterPage 最小注册页（CSRF meta + 验证码）。
// RegisterPage 渲染注册页（auth.register）。
func (r *AuthController) RegisterPage(ctx goravelhttp.Context) goravelhttp.Response {
	if user, _, _, err := currentUser(ctx); err == nil && user != nil {
		return ctx.Response().Redirect(nethttp.StatusFound, "/user")
	}
	options := pageOptions()
	return renderAuthPage(ctx, "auth_register", "auth.register.title", map[string]any{
		"player":    options.Bool("register_with_player_name", true),
		"recaptcha": options.Get("recaptcha_sitekey"),
		"invisible": options.Bool("recaptcha_invisible", false),
	}, pullFlash(ctx, "msg"), "", "")
}

// Register 处理 POST /auth/register。
func (r *AuthController) Register(ctx goravelhttp.Context) goravelhttp.Response {
	email := strings.TrimSpace(ctx.Request().Input("email"))
	password := ctx.Request().Input("password")
	captchaInput := strings.TrimSpace(ctx.Request().Input("captcha"))
	playerName := strings.TrimSpace(ctx.Request().Input("player_name"))
	nickname := strings.TrimSpace(ctx.Request().Input("nickname"))

	conn, dialect, err := runtime.Connection()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	options := service.NewOption(conn, dialect)
	users := repo.NewUserRepository(conn, dialect)
	players := repo.NewPlayerRepository(conn, dialect)

	withPlayer := options.Bool("register_with_player_name", true)
	errs := map[string][]string{}
	// PHP：'email' => required|email|unique:users（唯一性在下方单独检查）。
	if email == "" {
		errs["email"] = []string{vRequired(ctx, "email")}
	} else if !regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`).MatchString(email) {
		errs["email"] = []string{vEmail(ctx, "email")}
	}
	// PHP：'password' => required|min:8|max:32。
	if message := vPasswordMessage(ctx, "password", password, 8, 32); message != "" {
		errs["password"] = []string{message}
	}

	if withPlayer {
		if message := validatePlayerName(ctx, options, "player_name", playerName); message != "" {
			errs["player_name"] = []string{message}
		}
	} else if nickname == "" {
		errs["nickname"] = []string{vRequired(ctx, "nickname")}
	} else if len([]rune(nickname)) > 255 {
		// PHP：'nickname' => required|max:255。
		errs["nickname"] = []string{vMaxString(ctx, "nickname", "255")}
	}

	if !verifyCaptcha(ctx, options, captchaInput) {
		errs["captcha"] = []string{vCaptcha(ctx, options)}
	}

	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	if count, _ := users.CountByEmail(email); count > 0 {
		return validationFailed(ctx, map[string][]string{
			"email": {vUnique(ctx, "email")},
		})
	}
	if withPlayer {
		if existing, _ := players.FindDetailByName(playerName); existing != nil {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": t(ctx, "user.player.add.repeated"),
			})
		}
	}

	regsPerIP := options.Int("regs_per_ip", 3)
	if count, _ := users.CountByIP(ctx.Request().Ip()); count >= regsPerIP {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": t(ctx, "auth.register.max", "regs", fmt.Sprint(regsPerIP)),
		})
	}

	hashed, err := php.HashPassword(password, 0)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	displayName := nickname
	if withPlayer {
		displayName = playerName
	}
	uid, err := users.Insert(email, displayName, hashed, ctx.Request().Ip(),
		options.Int("user_initial_score", 1000))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	if withPlayer {
		if _, err := players.InsertWithSkin(uid, playerName, 0); err != nil {
			return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
		}
	}

	if err := auth.Login(ctx, users, uid, false); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserRegistered, Payload: uid})
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserLoggedIn, Payload: uid})

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": t(ctx, "auth.register.success"),
		"data": goravelhttp.Json{"uid": uid},
	})
}
