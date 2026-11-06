package controllers

import (
	"encoding/base64"
	"fmt"
	"html"
	nethttp "net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/app/facades"
	"neoskin/internal/auth"
	"neoskin/internal/laravel"
	"neoskin/internal/model"
	"neoskin/internal/oauth"
	"neoskin/internal/php"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
)

type OAuthController struct{}

func NewOAuthController() *OAuthController {
	return &OAuthController{}
}

func (r *OAuthController) repository() (*repo.OAuthRepository, *oauth.Keys, goravelhttp.Response) {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return nil, nil, nil
	}
	keys, err := oauth.RuntimeKeys()
	if err != nil {
		return nil, nil, nil
	}
	return repo.NewOAuthRepository(conn, dialect), keys, nil
}

// Scopes 处理 GET /oauth/scopes。
func (r *OAuthController) Scopes(ctx goravelhttp.Context) goravelhttp.Response {
	repository, _, _ := r.repository()
	scopes := map[string]string{}
	for name, description := range oauth.DefaultScopes {
		scopes[name] = description
	}
	for name, description := range repository.Scopes() {
		scopes[name] = description
	}
	data := make([]goravelhttp.Json, 0, len(scopes))
	for name, description := range scopes {
		data = append(data, goravelhttp.Json{"id": name, "description": description})
	}
	return ctx.Response().Json(nethttp.StatusOK, data)
}

// Clients 处理 GET /oauth/clients。
func (r *OAuthController) Clients(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	clients, err := repository.ClientsForUser(user.UID)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	data := make([]goravelhttp.Json, 0, len(clients))
	for _, client := range clients {
		data = append(data, clientJSON(client))
	}
	return ctx.Response().Json(nethttp.StatusOK, data)
}

// StoreClient 处理 POST /oauth/clients。
func (r *OAuthController) StoreClient(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	name := strings.TrimSpace(ctx.Request().Input("name"))
	redirect := strings.TrimSpace(ctx.Request().Input("redirect"))
	errs := map[string][]string{}
	if name == "" {
		errs["name"] = []string{vRequired(ctx, "name")}
	}
	if redirect == "" {
		errs["redirect"] = []string{vRequired(ctx, "redirect")}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}
	secret := repo.RandomHex(40)
	hashed, err := php.HashPassword(secret, 0)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	id, err := repository.CreateClient(user.UID, name, hashed, redirect)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	client, _ := repository.FindClient(id)
	output := clientJSON(*client)
	output["plain_secret"] = secret
	return ctx.Response().Json(nethttp.StatusOK, output)
}

// UpdateClient 处理 PUT /oauth/clients/{client_id}。
func (r *OAuthController) UpdateClient(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	id := ctx.Request().RouteInt("client_id")
	client, err := repository.FindClient(id)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if client == nil || client.UserID == nil || *client.UserID != user.UID {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	name := ctx.Request().Input("name", client.Name)
	redirect := ctx.Request().Input("redirect", client.Redirect)
	if err := repository.UpdateClient(id, name, redirect); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	updated, _ := repository.FindClient(id)
	return ctx.Response().Json(nethttp.StatusOK, clientJSON(*updated))
}

// DeleteClient 处理 DELETE /oauth/clients/{client_id}。
func (r *OAuthController) DeleteClient(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	id := ctx.Request().RouteInt("client_id")
	client, err := repository.FindClient(id)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if client == nil || client.UserID == nil || *client.UserID != user.UID {
		return ctx.Response().String(nethttp.StatusNotFound, "Not Found")
	}
	if err := repository.DeleteClient(id); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().String(nethttp.StatusNoContent, "")
}

// Authorize 处理 GET /oauth/authorize。
func (r *OAuthController) Authorize(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	clientID := ctx.Request().QueryInt("client_id")
	redirectURI := ctx.Request().Query("redirect_uri")
	responseType := ctx.Request().Query("response_type", "code")
	state := ctx.Request().Query("state")
	scopes := splitScopes(ctx.Request().Query("scope"))

	client, err := repository.FindClient(clientID)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if client == nil || client.Revoked {
		return ctx.Response().String(nethttp.StatusBadRequest, "invalid_client")
	}
	if !redirectAllowed(client.Redirect, redirectURI) {
		return ctx.Response().String(nethttp.StatusBadRequest, "invalid_redirect_uri")
	}
	if responseType != "code" {
		return ctx.Response().String(nethttp.StatusBadRequest, "unsupported_response_type")
	}
	if len(scopes) == 0 {
		scopes = []string{"User.Read"}
	}
	allScopes := oauth.DefaultScopes
	for _, scope := range scopes {
		if _, ok := allScopes[scope]; !ok {
			if _, dbScope := repository.Scopes()[scope]; !dbScope {
				return ctx.Response().String(nethttp.StatusBadRequest, "invalid_scope")
			}
		}
	}

	session := ctx.Request().Session()
	authToken := repo.RandomHex(40)
	session.Put("oauth_auth_token", authToken)
	session.Put("oauth_client_id", clientID)
	session.Put("oauth_redirect_uri", redirectURI)
	session.Put("oauth_state", state)
	session.Put("oauth_scopes", scopes)

	var scopesHTML strings.Builder
	for _, scope := range scopes {
		description := allScopes[scope]
		if description == "" {
			description = repository.Scopes()[scope]
		}
		scopesHTML.WriteString("<li><code>" + html.EscapeString(scope) + "</code> — " +
			html.EscapeString(description) + "</li>")
	}
	page := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8">
<meta name="csrf-token" content="%s"><title>Authorize</title></head><body>
<h1>Authorize application</h1><p>“%s” requests access to your account (%s).</p>
<ul>%s</ul><form method="post" action="/oauth/authorize">
<input type="hidden" name="auth_token" value="%s">
<button type="submit">Authorize</button>
<button type="button" onclick="fetch('/oauth/authorize',{method:'DELETE'}).then(()=>location.reload())">Deny</button>
</form></body></html>`, session.Token(), html.EscapeString(client.Name),
		html.EscapeString(user.Nickname), scopesHTML.String(), authToken)

	return ctx.Response().String(nethttp.StatusOK, page)
}

// Approve 处理 POST /oauth/authorize。
func (r *OAuthController) Approve(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	session := ctx.Request().Session()
	posted := ctx.Request().Input("auth_token")
	stored := fmt.Sprint(session.Get("oauth_auth_token"))
	if stored == "" || stored != posted {
		return ctx.Response().String(nethttp.StatusForbidden, "Forbidden")
	}
	clientID := toInt(session.Get("oauth_client_id"))
	redirectURI := fmt.Sprint(session.Get("oauth_redirect_uri"))
	state := fmt.Sprint(session.Get("oauth_state"))
	scopes := toStrings(session.Get("oauth_scopes"))

	code := repo.RandomHex(80)
	expires := time.Now().Add(oauth.CodeTTLSeconds * time.Second).
		Format("2006-01-02 15:04:05")
	if err := repository.InsertAuthCode(repo.OAuthAuthCode{
		ID: code, UserID: user.UID, ClientID: clientID, Scopes: scopes, ExpiresAt: expires,
	}); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	for _, key := range []string{"oauth_auth_token", "oauth_client_id", "oauth_redirect_uri",
		"oauth_state", "oauth_scopes"} {
		session.Forget(key)
	}

	separator := "?"
	if strings.Contains(redirectURI, "?") {
		separator = "&"
	}
	target := redirectURI + separator + "code=" + url.QueryEscape(code)
	if state != "" {
		target += "&state=" + url.QueryEscape(state)
	}
	ctx.Response().Header("Location", target)
	return ctx.Response().String(nethttp.StatusFound, "")
}

// Deny 处理 DELETE /oauth/authorize。
func (r *OAuthController) Deny(ctx goravelhttp.Context) goravelhttp.Response {
	session := ctx.Request().Session()
	redirectURI := fmt.Sprint(session.Get("oauth_redirect_uri"))
	state := fmt.Sprint(session.Get("oauth_state"))
	for _, key := range []string{"oauth_auth_token", "oauth_client_id", "oauth_redirect_uri",
		"oauth_state", "oauth_scopes"} {
		session.Forget(key)
	}
	if redirectURI == "" {
		return ctx.Response().String(nethttp.StatusNoContent, "")
	}
	separator := "?"
	if strings.Contains(redirectURI, "?") {
		separator = "&"
	}
	target := redirectURI + separator + "error=access_denied"
	if state != "" {
		target += "&state=" + url.QueryEscape(state)
	}
	ctx.Response().Header("Location", target)
	return ctx.Response().String(nethttp.StatusFound, "")
}

// Token 处理 POST /oauth/token。
func (r *OAuthController) Token(ctx goravelhttp.Context) goravelhttp.Response {
	repository, keys, _ := r.repository()
	grantType := ctx.Request().Input("grant_type")
	switch grantType {
	case "authorization_code":
		return r.tokenFromCode(ctx, repository, keys)
	case "refresh_token":
		return r.tokenFromRefresh(ctx, repository, keys)
	case "client_credentials":
		return r.tokenFromClientCredentials(ctx, repository, keys)
	}
	return ctx.Response().Json(nethttp.StatusBadRequest, goravelhttp.Json{
		"error": "unsupported_grant_type",
	})
}

func (r *OAuthController) tokenFromCode(ctx goravelhttp.Context, repository *repo.OAuthRepository,
	keys *oauth.Keys) goravelhttp.Response {
	codeValue := ctx.Request().Input("code")
	clientID := ctx.Request().InputInt("client_id")
	clientSecret := ctx.Request().Input("client_secret")
	if id, secret, ok := basicClient(ctx); ok {
		clientID, clientSecret = id, secret
	}
	redirectURI := ctx.Request().Input("redirect_uri")
	code, err := repository.FindAuthCode(codeValue)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if code == nil || code.Revoked || expired(code.ExpiresAt) {
		return ctx.Response().Json(nethttp.StatusBadRequest, goravelhttp.Json{
			"error": "invalid_grant",
		})
	}
	client, err := repository.FindClient(clientID)
	if err != nil || client == nil {
		return ctx.Response().Json(nethttp.StatusBadRequest, goravelhttp.Json{
			"error": "invalid_client",
		})
	}
	if !php.VerifyPassword(client.Secret, clientSecret) {
		return ctx.Response().Json(nethttp.StatusBadRequest, goravelhttp.Json{
			"error": "invalid_client",
		})
	}
	if redirectURI != "" && !redirectAllowed(client.Redirect, redirectURI) {
		return ctx.Response().Json(nethttp.StatusBadRequest, goravelhttp.Json{
			"error": "invalid_grant",
		})
	}
	if code.ClientID != clientID {
		return ctx.Response().Json(nethttp.StatusBadRequest, goravelhttp.Json{
			"error": "invalid_grant",
		})
	}
	_ = repository.RevokeAuthCode(code.ID)

	userID := code.UserID
	issued, err := oauth.Issue(repository, keys, &userID, clientID, "", code.Scopes, true)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return tokenResponse(ctx, issued)
}

func (r *OAuthController) tokenFromRefresh(ctx goravelhttp.Context,
	repository *repo.OAuthRepository, keys *oauth.Keys) goravelhttp.Response {
	refresh, err := repository.FindRefreshToken(ctx.Request().Input("refresh_token"))
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if refresh == nil || refresh.Revoked || expired(refresh.ExpiresAt) {
		return ctx.Response().Json(nethttp.StatusBadRequest, goravelhttp.Json{
			"error": "invalid_grant",
		})
	}
	accessToken, _ := repository.FindAccessToken(refresh.AccessTokenID)
	if accessToken == nil {
		return ctx.Response().Json(nethttp.StatusBadRequest, goravelhttp.Json{
			"error": "invalid_grant",
		})
	}
	_ = repository.RevokeRefreshToken(refresh.ID)
	_ = repository.RevokeToken(accessToken.ID)
	issued, err := oauth.Issue(repository, keys, accessToken.UserID, accessToken.ClientID,
		accessToken.Name, accessToken.Scopes, true)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return tokenResponse(ctx, issued)
}

// basicClient 解析 `Authorization: Basic base64(client_id:client_secret)`。
//
// RFC 6749 要求客户端凭据先做 form-urlencoded 再做 Base64；这里对 id/secret
// 各做一次 QueryUnescape。
func basicClient(ctx goravelhttp.Context) (int, string, bool) {
	header := ctx.Request().Header("Authorization")
	if header == "" || !strings.HasPrefix(strings.ToLower(header), "basic ") {
		return 0, "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[6:]))
	if err != nil {
		return 0, "", false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return 0, "", false
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	secret, err := url.QueryUnescape(parts[1])
	if err != nil {
		secret = parts[1]
	}

	return id, secret, true
}

// tokenFromClientCredentials 处理 client_credentials grant。
//
// Passport 默认启用该 grant（password grant 默认关闭）；没有用户，也不下发
// refresh token。scope 省略时使用 Passport 的默认 scope `User.Read`。
func (r *OAuthController) tokenFromClientCredentials(ctx goravelhttp.Context,
	repository *repo.OAuthRepository, keys *oauth.Keys) goravelhttp.Response {
	clientID := ctx.Request().InputInt("client_id")
	clientSecret := ctx.Request().Input("client_secret")
	if id, secret, ok := basicClient(ctx); ok {
		clientID, clientSecret = id, secret
	}

	client, err := repository.FindClient(clientID)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if client == nil || client.Revoked {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"error": "invalid_client",
		})
	}
	if client.Secret != "" && !php.VerifyPassword(client.Secret, clientSecret) {
		return ctx.Response().Json(nethttp.StatusUnauthorized, goravelhttp.Json{
			"error": "invalid_client",
		})
	}

	scopes := splitScopes(ctx.Request().Input("scope"))
	if len(scopes) == 0 {
		scopes = []string{"User.Read"}
	}

	issued, err := oauth.Issue(repository, keys, nil, client.ID, "", scopes, false)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}

	return tokenResponse(ctx, issued)
}

// Tokens 处理 GET /oauth/tokens。
func (r *OAuthController) Tokens(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	tokens, err := repository.ActiveTokensForUser(user.UID)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, tokenList(repository, tokens))
}

// DeleteToken 处理 DELETE /oauth/tokens/{token_id}。
func (r *OAuthController) DeleteToken(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	return revokeTokenForUser(ctx, repository, user.UID, ctx.Request().Route("token_id"))
}

func (r *OAuthController) userAndRepo(ctx goravelhttp.Context) (*model.User,
	*repo.OAuthRepository, goravelhttp.Response) {
	conn, dialect, err := runtime.Connection()
	if err != nil {
		return nil, nil, nil
	}
	users := repo.NewUserRepository(conn, dialect)
	user, err := auth.CurrentUser(ctx, users)
	if err != nil || user == nil {
		return nil, nil, ctx.Response().Json(nethttp.StatusUnauthorized,
			goravelhttp.Json{"message": "Unauthenticated."})
	}
	return user, repo.NewOAuthRepository(conn, dialect), nil
}

// PersonalTokens 处理 GET /oauth/personal-access-tokens。
func (r *OAuthController) PersonalTokens(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	tokens, err := repository.ActiveTokensForUser(user.UID)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	filtered := make([]repo.OAuthAccessToken, 0, len(tokens))
	for _, token := range tokens {
		client, err := repository.FindClient(token.ClientID)
		if err != nil {
			continue
		}
		if client == nil || client.Revoked || !client.PersonalAccessClient {
			continue
		}
		filtered = append(filtered, token)
	}
	return ctx.Response().Json(nethttp.StatusOK, tokenList(repository, filtered))
}

// StorePersonalToken 处理 POST /oauth/personal-access-tokens。
func (r *OAuthController) StorePersonalToken(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	name := strings.TrimSpace(ctx.Request().Input("name"))
	errs := map[string][]string{}
	if name == "" {
		errs["name"] = []string{vRequired(ctx, "name")}
	} else if len([]rune(name)) > 255 {
		errs["name"] = []string{vMaxString(ctx, "name", "255")}
	}
	if len(errs) > 0 {
		return validationFailed(ctx, errs)
	}

	scopes := inputScopes(ctx.Request().Input("scopes"))
	for _, scope := range scopes {
		if !knownScope(repository, scope) {
			return validationFailed(ctx, map[string][]string{
				"scopes": {vIn(ctx, "scopes")},
			})
		}
	}

	client, err := repository.FindPersonalClient()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if client == nil {
		return ctx.Response().String(nethttp.StatusInternalServerError,
			"Personal access client not found for 'users' user provider. Please create one.")
	}
	keys, keyErr := oauth.RuntimeKeys()
	if keyErr != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, keyErr.Error())
	}
	userID := user.UID
	issued, err := oauth.Issue(repository, keys, &userID, client.ID, name, scopes, false)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	return ctx.Response().Json(nethttp.StatusOK, goravelhttp.Json{
		"accessTokenId": issued.TokenID, "tokenType": "Bearer",
		"expiresIn": issued.ExpiresIn, "accessToken": issued.AccessToken,
	})
}

// DeletePersonalToken 处理 DELETE /oauth/personal-access-tokens/{token_id}。
func (r *OAuthController) DeletePersonalToken(ctx goravelhttp.Context) goravelhttp.Response {
	user, repository, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	return revokeTokenForUser(ctx, repository, user.UID, ctx.Request().Route("token_id"))
}

// revokeTokenForUser 对应 Passport 的 findForUser + revoke（同时撤销 refresh token）。
func revokeTokenForUser(ctx goravelhttp.Context, repository *repo.OAuthRepository,
	uid int, id string) goravelhttp.Response {
	token, err := repository.FindTokenForUser(id, uid)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	if token == nil {
		return ctx.Response().String(nethttp.StatusNotFound, "")
	}
	if err := repository.RevokeToken(id); err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	_ = repository.RevokeRefreshTokensForAccessToken(id)
	return ctx.Response().String(nethttp.StatusNoContent, "")
}

func tokenList(repository *repo.OAuthRepository, tokens []repo.OAuthAccessToken) []goravelhttp.Json {
	data := make([]goravelhttp.Json, 0, len(tokens))
	for _, token := range tokens {
		entry := goravelhttp.Json{
			"id": token.ID, "user_id": token.UserID, "client_id": token.ClientID,
			"name": token.Name, "scopes": token.Scopes, "revoked": token.Revoked,
			"created_at": token.CreatedAt, "updated_at": token.UpdatedAt,
			"expires_at": token.ExpiresAt,
		}
		if client, err := repository.FindClient(token.ClientID); err == nil && client != nil {
			entry["client"] = clientJSON(*client)
		}
		data = append(data, entry)
	}
	return data
}

func inputScopes(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				result = append(result, text)
			}
		}
		return result
	case string:
		return splitScopes(typed)
	}
	return nil
}

func knownScope(repository *repo.OAuthRepository, scope string) bool {
	if _, ok := oauth.DefaultScopes[scope]; ok {
		return true
	}
	_, ok := repository.Scopes()[scope]
	return ok
}

// RefreshTransientToken 处理 POST /oauth/token/refresh。
// 对应 Passport 的 TransientTokenController@refresh：返回 "Refreshed." 并下发 laravel_token cookie。
func (r *OAuthController) RefreshTransientToken(ctx goravelhttp.Context) goravelhttp.Response {
	user, _, resp := r.userAndRepo(ctx)
	if resp != nil {
		return resp
	}
	key, err := laravel.Key()
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError,
			"NEOSKIN_APP_KEY is not configured.")
	}
	session := ctx.Request().Session()
	minutes := facades.Config().GetInt("session.lifetime", 120)
	expires := time.Now().Add(time.Duration(minutes) * time.Minute)
	token, err := laravel.SignHS256(map[string]any{
		"sub":  fmt.Sprint(user.UID),
		"csrf": session.Token(),
		"exp":  expires.Unix(),
	}, key)
	if err != nil {
		return ctx.Response().String(nethttp.StatusInternalServerError, err.Error())
	}
	response := ctx.Response().Cookie(goravelhttp.Cookie{
		Name:     "laravel_token",
		Value:    token,
		Expires:  expires,
		Path:     "/",
		HttpOnly: true,
		SameSite: "lax",
	})
	return response.String(nethttp.StatusOK, "Refreshed.")
}
