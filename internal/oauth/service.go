package oauth

import (
	"fmt"
	"time"

	"neoskin/internal/php"
	"neoskin/internal/repo"
	"neoskin/internal/runtime"
)

const (
	TokenTTLSeconds = 31536000
	CodeTTLSeconds  = 600
)

var DefaultScopes = map[string]string{
	"User.Read":                   "auth.oauth.scope.user.read",
	"Notification.Read":           "auth.oauth.scope.notification.read",
	"Notification.ReadWrite":      "auth.oauth.scope.notification.readwrite",
	"Player.Read":                 "auth.oauth.scope.player.read",
	"Player.ReadWrite":            "auth.oauth.scope.player.readwrite",
	"Closet.Read":                 "auth.oauth.scope.closet.read",
	"Closet.ReadWrtie":            "auth.oauth.scope.closet.readwrite",
	"UsersManagement.Read":        "auth.oauth.scope.users-management.read",
	"UsersManagement.ReadWrite":   "auth.oauth.scope.users-management.readwrite",
	"PlayersManagement.Read":      "auth.oauth.scope.players-management.read",
	"PlayersManagement.ReadWrite": "auth.oauth.scope.players-management.readwrite",
	"ClosetManagement.Read":       "auth.oauth.scope.closet-management.read",
	"ClosetManagement.ReadWrite":  "auth.oauth.scope.closet-management.readwrite",
	"ReportsManagement.Read":      "auth.oauth.scope.reports-management.read",
	"ReportsManagement.ReadWrite": "auth.oauth.scope.reports-management.readwrite",
}

func RuntimeKeys() (*Keys, error) {
	return LoadKeys(runtime.StorageRoot())
}

type Issued struct {
	AccessToken  string
	RefreshToken string
	TokenID      string
	ExpiresIn    int
}

// Issue 签发 access token（可选 refresh token）。
func Issue(repository *repo.OAuthRepository, keys *Keys, userID *int, clientID int,
	name string, scopes []string, withRefresh bool) (*Issued, error) {
	tokenID := repo.RandomHex(40)
	now := time.Now()
	exp := now.Add(TokenTTLSeconds * time.Second)

	sub := ""
	if userID != nil {
		sub = itoa(*userID)
	} else {
		sub = itoa(clientID)
	}

	token := repo.OAuthAccessToken{
		ID: tokenID, UserID: userID, ClientID: clientID, Name: name, Scopes: scopes,
		CreatedAt: php.Now(), UpdatedAt: php.Now(), ExpiresAt: exp.Format("2006-01-02 15:04:05"),
	}
	if err := repository.InsertAccessToken(token); err != nil {
		return nil, err
	}

	claims := map[string]any{
		"aud":    []string{itoa(clientID)},
		"jti":    tokenID,
		"iat":    now.Unix(),
		"nbf":    now.Unix(),
		"exp":    exp.Unix(),
		"sub":    sub,
		"scopes": scopes,
	}
	accessToken, err := SignRS256(claims, keys)
	if err != nil {
		return nil, err
	}

	issued := &Issued{AccessToken: accessToken, TokenID: tokenID, ExpiresIn: TokenTTLSeconds}
	if withRefresh {
		refresh := repo.RandomHex(40)
		if err := repository.InsertRefreshToken(repo.OAuthRefreshToken{
			ID: refresh, AccessTokenID: tokenID,
			ExpiresAt: exp.Format("2006-01-02 15:04:05"),
		}); err != nil {
			return nil, err
		}
		issued.RefreshToken = refresh
	}
	return issued, nil
}

func itoa(value int) string {
	return fmt.Sprint(value)
}
