package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"
	goravelsession "github.com/goravel/framework/session"
	"github.com/goravel/framework/support/str"

	"neoskin/internal/laravel"
	"neoskin/internal/model"
	"neoskin/internal/repo"
)

// SessionKey 与 Laravel SessionGuard 一致：login_web_ + sha1("Illuminate\Auth\SessionGuard")。
const SessionKey = "login_web_59ba36addc2b2f9401580f014c7f58ea4e30989d"

// passwordHashKey 对应 Laravel 的 session('password_hash_'.$this->name)。
const passwordHashKey = "password_hash_59ba36addc2b2f9401580f014c7f58ea4e30989d"

// RememberCookieName 与 Laravel SessionGuard::getRecallerName 一致：
// remember_web_ + 同一 sha1 后缀。
const RememberCookieName = "remember_web_59ba36addc2b2f9401580f014c7f58ea4e30989d"

// rememberDuration 对应 Laravel cookie `forever`（config("auth.reminders.expire")
// 默认 2628000 分钟，即 5 年）。
const rememberDuration = 2628000 * time.Minute

// passwordMACFallbackKey 对应 Laravel SessionGuard::hashPasswordForCookie 的
// `$this->hashKey ?? 'base-key-for-password-hash-mac'` 兜底（APP_KEY 缺失时）。
const passwordMACFallbackKey = "base-key-for-password-hash-mac"

var ErrNoSession = errors.New("session is not available")

func CurrentUID(ctx http.Context) int {
	session := ctx.Request().Session()
	if session == nil {
		return 0
	}

	value := session.Get(SessionKey)
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var uid int
		_, _ = fmt.Sscan(typed, &uid)
		return uid
	}

	return 0
}

func CurrentUser(ctx http.Context, users *repo.UserRepository) (*model.User, error) {
	uid := CurrentUID(ctx)
	if uid <= 0 {
		token, err := BearerToken(ctx, users)
		if err != nil {
			return nil, err
		}
		if token != nil && token.UserID != nil {
			uid = *token.UserID
			rememberScopes(ctx, token.Scopes)
		}
	}
	if uid <= 0 {
		// Laravel SessionGuard::user()：session 未命中时回退 remember
		// cookie（Auth::login($user, $keep) 下发的长效 cookie）。
		user := recallUser(ctx, users)
		if user != nil {
			uid = user.UID
			if session := ctx.Request().Session(); session != nil {
				session.Put(SessionKey, uid)
			}
		}
	}
	if uid <= 0 {
		return nil, nil
	}

	return users.FindByID(uid)
}

// recallUser 对应 Laravel SessionGuard::userFromRecaller：解析 remember
// cookie（id|token|password-mac），数据库中 remember_token 与密码 HMAC 同时
// 匹配才通过（密码修改后旧 cookie 自动失效，与 PHP hash 校验等价）。
func recallUser(ctx http.Context, users *repo.UserRepository) *model.User {
	parts := strings.Split(ctx.Request().Cookie(RememberCookieName), "|")
	if len(parts) != 3 {
		return nil
	}

	var uid int
	if _, err := fmt.Sscan(parts[0], &uid); err != nil || uid <= 0 {
		return nil
	}
	token, mac := parts[1], parts[2]
	if token == "" || mac == "" {
		return nil
	}

	user, err := users.FindByID(uid)
	if err != nil || user == nil {
		return nil
	}
	if user.RememberToken == "" || !hmacEqual(user.RememberToken, token) {
		return nil
	}
	if !hmacEqual(passwordMAC(user.Password), mac) {
		return nil
	}

	return user
}

// Login 对应 Laravel SessionGuard::login：写 session、regenerate session ID
// （session fixation 防护），并可选建立 remember cookie。
func Login(ctx http.Context, users *repo.UserRepository, uid int, remember bool) error {
	session := ctx.Request().Session()
	if session == nil {
		return ErrNoSession
	}

	session.Put(SessionKey, uid)
	if err := session.Regenerate(true); err != nil {
		return err
	}

	// session 中间件在 Next() 之前已写出旧 ID 的 Set-Cookie；Goravel
	// session.WriteCookie 的注释明确要求轮换 ID 后重新调用，同名 cookie
	// 会覆盖（gin 驱动内部 removeSetCookie）。
	goravelsession.WriteCookie(ctx, session)

	if !remember {
		return nil
	}

	user, err := users.FindByID(uid)
	if err != nil {
		return err
	}
	if user == nil {
		return nil
	}

	token := user.RememberToken
	// Laravel ensureRememberTokenIsSet：token 为空时先轮换一个新值。
	if token == "" {
		token = str.Random(60)
		if err := users.UpdateRememberToken(user.UID, token); err != nil {
			return err
		}
	}
	// Laravel 还会写 session('password_hash_<name>')，保持同键名。
	session.Put(passwordHashKey, passwordMAC(user.Password))
	writeRememberCookie(ctx, rememberCookieValue(uid, token, passwordMAC(user.Password)))

	return nil
}

// Logout 对应 Laravel SessionGuard::logout：清除 session 身份；remember
// cookie 存在时下发过期 cookie；remember_token 非空时轮换（cycleRememberToken），
// 使所有已记住设备失效。
func Logout(ctx http.Context, users *repo.UserRepository) error {
	session := ctx.Request().Session()
	if session == nil {
		return ErrNoSession
	}

	uid := CurrentUID(ctx)
	session.Forget(SessionKey)

	hasCookie := ctx.Request().Cookie(RememberCookieName) != ""
	if hasCookie {
		expireRememberCookie(ctx)
	}

	// session 已过期但 remember cookie 仍有效时（Laravel 的 user() 也会经
	// recaller 找回用户），同样轮换其 token。
	var user *model.User
	if uid > 0 {
		user, _ = users.FindByID(uid)
	} else if hasCookie {
		user = recallUser(ctx, users)
	}
	if user != nil && user.RememberToken != "" {
		if err := users.UpdateRememberToken(user.UID, str.Random(60)); err != nil {
			return err
		}
	}

	return nil
}

// rememberCookieValue 组装 cookie 内容 `uid|token|password-mac`。与 PHP 的
// 偏差：Laravel 会整体加密 cookie 值；Go 版不做加密而是以 NEOSKIN_APP_KEY
// 作为 HMAC key 保证不可伪造，第三段语义与 PHP hash 校验等价。
func rememberCookieValue(uid int, token, mac string) string {
	return fmt.Sprintf("%d|%s|%s", uid, token, mac)
}

func writeRememberCookie(ctx http.Context, value string) {
	ctx.Response().Cookie(http.Cookie{
		Name:     RememberCookieName,
		Value:    value,
		Path:     "/",
		Expires:  time.Now().Add(rememberDuration),
		HttpOnly: true,
		SameSite: "lax",
	})
}

func expireRememberCookie(ctx http.Context) {
	ctx.Response().Cookie(http.Cookie{
		Name:     RememberCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-24 * time.Hour),
		HttpOnly: true,
		SameSite: "lax",
	})
}

// passwordMAC 对应 Laravel SessionGuard::hashPasswordForCookie：
// hash_hmac('sha256', password, key)。key 优先取 NEOSKIN_APP_KEY。
func passwordMAC(passwordHash string) string {
	key, err := laravel.Key()
	if err != nil || len(key) == 0 {
		key = []byte(passwordMACFallbackKey)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(passwordHash))
	return hex.EncodeToString(mac.Sum(nil))
}

func hmacEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}
