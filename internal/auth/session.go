package auth

import (
	"errors"
	"fmt"

	"github.com/goravel/framework/contracts/http"

	"neoskin/internal/model"
	"neoskin/internal/repo"
)

// SessionKey 与 Laravel SessionGuard 一致：login_web_ + sha1("Illuminate\Auth\SessionGuard")。
const SessionKey = "login_web_59ba36addc2b2f9401580f014c7f58ea4e30989d"

const LoginFailsKey = "login_fails"

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
		return nil, nil
	}

	return users.FindByID(uid)
}

func Login(ctx http.Context, uid int) error {
	session := ctx.Request().Session()
	if session == nil {
		return ErrNoSession
	}

	session.Put(SessionKey, uid)
	// TODO: Laravel 会 regenerate session id；Goravel 的 session cookie 在
	// middleware Next 之前写出，regenerate 后需要额外写 cookie，待实现后恢复。
	return nil
}

func Logout(ctx http.Context) error {
	session := ctx.Request().Session()
	if session == nil {
		return ErrNoSession
	}

	session.Forget(SessionKey)
	return nil
}
