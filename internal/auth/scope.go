package auth

import (
	"strings"

	"github.com/goravel/framework/contracts/http"

	"neoskin/internal/oauth"
	"neoskin/internal/php"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
)

// tokenScopesKey 保存 Bearer token 授予的 scope；session 登录时不写入。
type tokenScopesKey struct{}

// BearerToken 解析并校验 Authorization: Bearer <JWT>。
// 返回 nil 表示请求没有携带 Bearer token 或 token 无效。
func BearerToken(ctx http.Context, users *repo.UserRepository) (*repo.OAuthAccessToken, error) {
	header := strings.TrimSpace(ctx.Request().Header("Authorization"))
	if header == "" {
		return nil, nil
	}
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return nil, nil
	}
	raw := strings.TrimSpace(header[len(prefix):])

	keys, err := oauth.RuntimeKeys()
	if err != nil {
		return nil, nil
	}
	claims := oauth.VerifyRS256(raw, keys)
	if claims == nil {
		return nil, nil
	}
	jti, _ := claims["jti"].(string)
	if jti == "" {
		return nil, nil
	}

	repository := repo.NewOAuthRepository(users.DB, users.Dialect)
	token, err := repository.FindAccessToken(jti)
	if err != nil {
		return nil, err
	}
	if token == nil || token.Revoked || token.ExpiresAt <= php.Now() {
		return nil, nil
	}
	return token, nil
}

// Authenticate 解析 Bearer token 并把 scope 写入上下文，供 OAuthScope 中间件判断。
// 使用 session 登录时不写入 scope（等价于 Passport 的 transient token，拥有全部权限）。
func Authenticate(ctx http.Context) error {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return err
	}
	users := repo.NewUserRepository(conn, dialect)
	token, err := BearerToken(ctx, users)
	if err != nil {
		return err
	}
	if token != nil {
		rememberScopes(ctx, token.Scopes)
	}
	return nil
}

// HasAnyScope 判断当前请求是否满足 scope。
// Passport 的 session 登录（transient token）拥有全部权限，因此无 Bearer token 时直接通过。
func HasAnyScope(ctx http.Context, required ...string) bool {
	if len(required) == 0 {
		return true
	}
	granted := CurrentTokenScopes(ctx)
	if granted == nil {
		return true
	}
	for _, scope := range granted {
		if scope == "*" {
			return true
		}
	}
	for _, want := range required {
		for _, scope := range granted {
			if scope == want {
				return true
			}
		}
	}
	return false
}

// CurrentTokenScopes 返回 Bearer token 的 scope；session 请求返回 nil。
func CurrentTokenScopes(ctx http.Context) []string {
	value := ctx.Value(tokenScopesKey{})
	if value == nil {
		return nil
	}
	scopes, _ := value.([]string)
	return scopes
}

func rememberScopes(ctx http.Context, scopes []string) {
	if scopes == nil {
		scopes = []string{}
	}
	ctx.WithValue(tokenScopesKey{}, scopes)
}
