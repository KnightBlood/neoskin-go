package repo

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"neoskin/internal/db"
	"neoskin/internal/php"
)

type OAuthClient struct {
	ID                   int
	UserID               *int
	Name                 string
	Secret               string
	Provider             string
	Redirect             string
	PersonalAccessClient bool
	PasswordClient       bool
	Revoked              bool
	CreatedAt            string
	UpdatedAt            string
}

type OAuthAuthCode struct {
	ID        string
	UserID    int
	ClientID  int
	Scopes    []string
	Revoked   bool
	ExpiresAt string
}

type OAuthAccessToken struct {
	ID        string
	UserID    *int
	ClientID  int
	Name      string
	Scopes    []string
	Revoked   bool
	CreatedAt string
	UpdatedAt string
	ExpiresAt string
}

type OAuthRefreshToken struct {
	ID            string
	AccessTokenID string
	Revoked       bool
	ExpiresAt     string
}

type OAuthRepository struct {
	DB      *sql.DB
	Dialect *db.Dialect
}

func NewOAuthRepository(conn *sql.DB, dialect *db.Dialect) *OAuthRepository {
	return &OAuthRepository{DB: conn, Dialect: dialect}
}

func (r *OAuthRepository) ClientsForUser(uid int) ([]OAuthClient, error) {
	query := `SELECT id, user_id, name, secret, provider, redirect, personal_access_client,
		password_client, revoked, created_at, updated_at FROM oauth_clients
		WHERE user_id = ? ORDER BY id`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []OAuthClient
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *client)
	}
	return result, rows.Err()
}

func (r *OAuthRepository) FindClient(id int) (*OAuthClient, error) {
	query := `SELECT id, user_id, name, secret, provider, redirect, personal_access_client,
		password_client, revoked, created_at, updated_at FROM oauth_clients WHERE id = ?`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanClient(rows)
}

func scanClient(rows *sql.Rows) (*OAuthClient, error) {
	var client OAuthClient
	var userID, secret, provider, createdAt, updatedAt any
	var personal, password, revoked any
	if err := rows.Scan(&client.ID, &userID, &client.Name, &secret, &provider,
		&client.Redirect, &personal, &password, &revoked, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if userID != nil {
		value := intValue(userID)
		client.UserID = &value
	}
	client.Secret = stringValue(secret)
	client.Provider = stringValue(provider)
	client.PersonalAccessClient = boolValue(personal)
	client.PasswordClient = boolValue(password)
	client.Revoked = boolValue(revoked)
	client.CreatedAt = db.TimeString(createdAt)
	client.UpdatedAt = db.TimeString(updatedAt)
	return &client, nil
}

func (r *OAuthRepository) CreateClient(uid int, name, secret, redirect string) (int, error) {
	now := php.Now()
	query := `INSERT INTO oauth_clients
		(user_id, name, secret, redirect, personal_access_client, password_client, revoked,
		 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	var id int64
	switch r.Dialect.Name {
	case "pgsql":
		query += " RETURNING id"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), uid, name, secret, redirect,
			false, false, false, now, now).Scan(&id); err != nil {
			return 0, err
		}
	case "sqlsrv":
		query += "; SELECT SCOPE_IDENTITY()"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), uid, name, secret, redirect,
			false, false, false, now, now).Scan(&id); err != nil {
			return 0, err
		}
	default:
		result, err := r.DB.Exec(db.Rebind(r.Dialect, query), uid, name, secret, redirect,
			false, false, false, now, now)
		if err != nil {
			return 0, err
		}
		id, err = result.LastInsertId()
		if err != nil {
			return 0, err
		}
	}
	return int(id), nil
}

func (r *OAuthRepository) UpdateClient(id int, name, redirect string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE oauth_clients SET name = ?, redirect = ?, updated_at = ? WHERE id = ?`),
		name, redirect, php.Now(), id)
	return err
}

func (r *OAuthRepository) DeleteClient(id int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `DELETE FROM oauth_clients WHERE id = ?`), id)
	return err
}

func (r *OAuthRepository) InsertAuthCode(code OAuthAuthCode) error {
	scopes, _ := json.Marshal(code.Scopes)
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`INSERT INTO oauth_auth_codes (id, user_id, client_id, scopes, revoked, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`),
		code.ID, code.UserID, code.ClientID, string(scopes), false, code.ExpiresAt)
	return err
}

func (r *OAuthRepository) FindAuthCode(id string) (*OAuthAuthCode, error) {
	rows, err := r.DB.Query(db.Rebind(r.Dialect,
		`SELECT id, user_id, client_id, scopes, revoked, expires_at
		 FROM oauth_auth_codes WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var code OAuthAuthCode
	var scopes, expiresAt, revoked any
	if err := rows.Scan(&code.ID, &code.UserID, &code.ClientID, &scopes, &revoked,
		&expiresAt); err != nil {
		return nil, err
	}
	code.Scopes = parseStringArray(stringValue(scopes))
	code.Revoked = boolValue(revoked)
	code.ExpiresAt = db.TimeString(expiresAt)
	return &code, nil
}

func (r *OAuthRepository) RevokeAuthCode(id string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE oauth_auth_codes SET revoked = ? WHERE id = ?`), true, id)
	return err
}

func (r *OAuthRepository) InsertAccessToken(token OAuthAccessToken) error {
	scopes, _ := json.Marshal(token.Scopes)
	var userID any
	if token.UserID != nil {
		userID = *token.UserID
	}
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`INSERT INTO oauth_access_tokens
		 (id, user_id, client_id, name, scopes, revoked, created_at, updated_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		token.ID, userID, token.ClientID, nullString(token.Name), string(scopes), false,
		token.CreatedAt, token.UpdatedAt, token.ExpiresAt)
	return err
}

func (r *OAuthRepository) TokensForUser(uid int) ([]OAuthAccessToken, error) {
	query := `SELECT id, user_id, client_id, name, scopes, revoked, created_at, updated_at,
		expires_at FROM oauth_access_tokens WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTokens(rows)
}

// ActiveTokensForUser 对应 Passport 的 forUser()：未撤销、未过期且客户端有效。
func (r *OAuthRepository) ActiveTokensForUser(uid int) ([]OAuthAccessToken, error) {
	query := `SELECT t.id, t.user_id, t.client_id, t.name, t.scopes, t.revoked,
		t.created_at, t.updated_at, t.expires_at
		FROM oauth_access_tokens t
		JOIN oauth_clients c ON c.id = t.client_id
		WHERE t.user_id = ? AND t.revoked = ? AND c.revoked = ?
		ORDER BY t.created_at DESC`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), uid, false, false)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens, err := scanTokens(rows)
	if err != nil {
		return nil, err
	}
	now := php.Now()
	active := tokens[:0]
	for _, token := range tokens {
		if token.ExpiresAt > now {
			active = append(active, token)
		}
	}
	return active, nil
}

// FindTokenForUser 对应 Passport 的 findForUser()：只返回有效 token。
func (r *OAuthRepository) FindTokenForUser(id string, uid int) (*OAuthAccessToken, error) {
	query := `SELECT id, user_id, client_id, name, scopes, revoked, created_at, updated_at,
		expires_at FROM oauth_access_tokens WHERE id = ? AND user_id = ? AND revoked = ?`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), id, uid, false)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens, err := scanTokens(rows)
	if err != nil || len(tokens) == 0 {
		return nil, err
	}
	if tokens[0].ExpiresAt <= php.Now() {
		return nil, nil
	}
	return &tokens[0], nil
}

func scanTokens(rows *sql.Rows) ([]OAuthAccessToken, error) {
	var result []OAuthAccessToken
	for rows.Next() {
		var token OAuthAccessToken
		var userID, name, scopes, createdAt, updatedAt, expiresAt, revoked any
		if err := rows.Scan(&token.ID, &userID, &token.ClientID, &name, &scopes, &revoked,
			&createdAt, &updatedAt, &expiresAt); err != nil {
			return nil, err
		}
		if userID != nil {
			value := intValue(userID)
			token.UserID = &value
		}
		token.Name = stringValue(name)
		token.Scopes = parseStringArray(stringValue(scopes))
		token.Revoked = boolValue(revoked)
		token.CreatedAt = db.TimeString(createdAt)
		token.UpdatedAt = db.TimeString(updatedAt)
		token.ExpiresAt = db.TimeString(expiresAt)
		result = append(result, token)
	}
	return result, rows.Err()
}

func (r *OAuthRepository) RevokeToken(id string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE oauth_access_tokens SET revoked = ? WHERE id = ?`), true, id)
	return err
}

func (r *OAuthRepository) InsertRefreshToken(token OAuthRefreshToken) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`INSERT INTO oauth_refresh_tokens (id, access_token_id, revoked, expires_at)
		 VALUES (?, ?, ?, ?)`), token.ID, token.AccessTokenID, false, token.ExpiresAt)
	return err
}

func (r *OAuthRepository) FindRefreshToken(id string) (*OAuthRefreshToken, error) {
	rows, err := r.DB.Query(db.Rebind(r.Dialect,
		`SELECT id, access_token_id, revoked, expires_at FROM oauth_refresh_tokens WHERE id = ?`),
		id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var token OAuthRefreshToken
	var revoked, expiresAt any
	if err := rows.Scan(&token.ID, &token.AccessTokenID, &revoked, &expiresAt); err != nil {
		return nil, err
	}
	token.Revoked = boolValue(revoked)
	token.ExpiresAt = db.TimeString(expiresAt)
	return &token, nil
}

func (r *OAuthRepository) RevokeRefreshToken(id string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE oauth_refresh_tokens SET revoked = ? WHERE id = ?`), true, id)
	return err
}

func (r *OAuthRepository) Scopes() map[string]string {
	result := map[string]string{}
	rows, err := r.DB.Query(db.Rebind(r.Dialect, `SELECT name, description FROM scopes`))
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var name, description string
		if rows.Scan(&name, &description) == nil {
			result[name] = description
		}
	}
	return result
}

func parseStringArray(raw string) []string {
	var result []string
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &result)
	}
	return result
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case int:
		return typed
	case []byte:
		var parsed int
		_, _ = fmt.Sscan(string(typed), &parsed)
		return parsed
	case string:
		var parsed int
		_, _ = fmt.Sscan(typed, &parsed)
		return parsed
	}
	return 0
}
func (r *OAuthRepository) FindAccessToken(id string) (*OAuthAccessToken, error) {
	rows, err := r.DB.Query(db.Rebind(r.Dialect,
		`SELECT id, user_id, client_id, name, scopes, revoked, created_at, updated_at,
		 expires_at FROM oauth_access_tokens WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var token OAuthAccessToken
	var userID, name, scopes, createdAt, updatedAt, expiresAt, revoked any
	if err := rows.Scan(&token.ID, &userID, &token.ClientID, &name, &scopes, &revoked,
		&createdAt, &updatedAt, &expiresAt); err != nil {
		return nil, err
	}
	if userID != nil {
		value := intValue(userID)
		token.UserID = &value
	}
	token.Name = stringValue(name)
	token.Scopes = parseStringArray(stringValue(scopes))
	token.Revoked = boolValue(revoked)
	token.CreatedAt = db.TimeString(createdAt)
	token.UpdatedAt = db.TimeString(updatedAt)
	token.ExpiresAt = db.TimeString(expiresAt)
	return &token, nil
}
func (r *OAuthRepository) FindPersonalClient() (*OAuthClient, error) {
	query := `SELECT id, user_id, name, secret, provider, redirect, personal_access_client,
		password_client, revoked, created_at, updated_at FROM oauth_clients
		WHERE personal_access_client = ? AND revoked = ? ORDER BY id LIMIT 1`
	if r.Dialect.Name == "sqlsrv" {
		query = `SELECT TOP 1 id, user_id, name, secret, provider, redirect,
			personal_access_client, password_client, revoked, created_at, updated_at
			FROM oauth_clients WHERE personal_access_client = ? AND revoked = ? ORDER BY id`
	}
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), true, false)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanClient(rows)
}

// RevokeRefreshTokensForAccessToken 撤销某个 access token 关联的 refresh token。
func (r *OAuthRepository) RevokeRefreshTokensForAccessToken(accessTokenID string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE oauth_refresh_tokens SET revoked = ? WHERE access_token_id = ?`),
		true, accessTokenID)
	return err
}
