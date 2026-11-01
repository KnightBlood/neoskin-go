package controllers

import (
	"fmt"
	nethttp "net/http"
	"strings"
	"time"

	goravelhttp "github.com/goravel/framework/contracts/http"

	"neoskin/internal/oauth"
	"neoskin/internal/repo"
)

func clientJSON(client repo.OAuthClient) goravelhttp.Json {
	return goravelhttp.Json{
		"id": client.ID, "user_id": client.UserID, "name": client.Name,
		"provider": client.Provider, "redirect": client.Redirect,
		"personal_access_client": client.PersonalAccessClient,
		"password_client":        client.PasswordClient,
		"revoked":                client.Revoked, "created_at": client.CreatedAt,
		"updated_at": client.UpdatedAt,
	}
}

func tokenResponse(ctx goravelhttp.Context, issued *oauth.Issued) goravelhttp.Response {
	body := goravelhttp.Json{
		"token_type": "Bearer", "expires_in": issued.ExpiresIn,
		"access_token": issued.AccessToken,
	}
	if issued.RefreshToken != "" {
		body["refresh_token"] = issued.RefreshToken
	}
	return ctx.Response().Json(nethttp.StatusOK, body)
}

func splitScopes(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", " ")
	var result []string
	for _, value := range strings.Fields(raw) {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func redirectAllowed(registered, target string) bool {
	if target == "" {
		return false
	}
	for _, candidate := range strings.Split(registered, ",") {
		if strings.TrimSpace(candidate) == target {
			return true
		}
	}
	return false
}

func expired(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	if err != nil {
		return true
	}
	return time.Now().After(parsed)
}

func toInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var parsed int
		_, _ = fmt.Sscan(typed, &parsed)
		return parsed
	}
	return 0
}

func toStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, fmt.Sprint(item))
		}
		return result
	case string:
		return splitScopes(typed)
	}
	return nil
}
