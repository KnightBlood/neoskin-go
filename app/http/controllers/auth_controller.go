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
	return renderAuthPage(ctx, "auth_login", "auth.login.title", map[string]any{
		"tooManyFails": false,
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
		errs["identification"] = []string{"The identification field is required."}
	}
	if password == "" {
		errs["password"] = []string{"The password field is required."}
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

	var user *model.User
	if strings.Contains(identification, "@") {
		user, err = users.FindByEmail(identification)
	} else {
		player, findErr := players.FindDetailByName(identification)
		err = findErr
		if player != nil {
			if player.Permission == model.PermissionBanned {
				return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
					"code": 1, "message": "This user has been banned.",
				})
			}
			user, err = users.FindByID(player.UID)
		}
	}
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	if user == nil || !php.VerifyPassword(user.Password, password) {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": "Wrong password.",
			"data": goravelhttp.Json{"login_fails": 1},
		})
	}
	if user.Permission == model.PermissionBanned {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": "This user has been banned.",
		})
	}

	if err := auth.Login(ctx, user.UID); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserLoggedIn, Payload: user.UID})
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserAuthenticated, Payload: user.UID})

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "Logged in.", "data": goravelhttp.Json{"uid": user.UID},
	})
}

// Logout 处理 POST /auth/logout。
func (r *AuthController) Logout(ctx goravelhttp.Context) goravelhttp.Response {
	if err := auth.Logout(ctx); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "You are now logged out.",
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
	if email == "" || !regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`).MatchString(email) {
		errs["email"] = []string{"The email field must be a valid email address."}
	}
	if len(password) < 8 || len(password) > 32 {
		errs["password"] = []string{"The password must be between 8 and 32 characters."}
	}

	if withPlayer {
		minLength := options.Int("player_name_length_min", 3)
		maxLength := options.Int("player_name_length_max", 16)
		if playerName == "" || len(playerName) < minLength || len(playerName) > maxLength ||
			!regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(playerName) {
			errs["player_name"] = []string{"The player name is invalid."}
		}
	} else if nickname == "" {
		errs["nickname"] = []string{"The nickname field is required."}
	}

	expected := ""
	if session := ctx.Request().Session(); session != nil {
		if value := session.Pull("captcha"); value != nil {
			expected = fmt.Sprint(value)
		}
	}
	if expected == "" || !strings.EqualFold(expected, captchaInput) {
		errs["captcha"] = []string{"The captcha is incorrect."}
	}

	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	if count, _ := users.CountByEmail(email); count > 0 {
		return validationFailed(ctx, map[string][]string{
			"email": {"The email has already been taken."},
		})
	}
	if withPlayer {
		if existing, _ := players.FindDetailByName(playerName); existing != nil {
			return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
				"code": 1, "message": "This player name is already taken.",
			})
		}
	}

	regsPerIP := options.Int("regs_per_ip", 3)
	if count, _ := users.CountByIP(ctx.Request().Ip()); count >= regsPerIP {
		return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
			"code": 1, "message": fmt.Sprintf("You can only register %d accounts per IP.", regsPerIP),
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

	if err := auth.Login(ctx, uid); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserRegistered, Payload: uid})
	plugin.DefaultDispatcher.Dispatch(plugin.Event{Name: plugin.EventUserLoggedIn, Payload: uid})

	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"code": 0, "message": "Your account was registered. Redirecting...",
		"data": goravelhttp.Json{"uid": uid},
	})
}

func validationFailed(ctx goravelhttp.Context, errors map[string][]string) goravelhttp.Response {
	return ctx.Response().Json(nethttp.StatusUnprocessableEntity, goravelhttp.Json{
		"message": "The given data was invalid.",
		"errors":  errors,
	})
}
