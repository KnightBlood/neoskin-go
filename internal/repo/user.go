package repo

import (
	"database/sql"
	"fmt"
	"strings"

	"neoskin/internal/db"
	"neoskin/internal/model"
	"neoskin/internal/php"
)

// UserRepository 是 users 表的访问层（原生 SQL，方言由 db.Dialect 决定）。
type UserRepository struct {
	DB      *sql.DB
	Dialect *db.Dialect
}

func NewUserRepository(conn *sql.DB, dialect *db.Dialect) *UserRepository {
	return &UserRepository{DB: conn, Dialect: dialect}
}

const userColumns = `uid, email, nickname, password, permission, avatar, score,
	verified, is_dark_mode, locale, ip, register_at, last_sign_at, verification_token`

func (r *UserRepository) FindByEmail(email string) (*model.User, error) {
	return r.findOne(`SELECT `+userColumns+` FROM users WHERE email = ?`, email)
}

func (r *UserRepository) FindByID(uid int) (*model.User, error) {
	return r.findOne(`SELECT `+userColumns+` FROM users WHERE uid = ?`, uid)
}

func (r *UserRepository) findOne(query string, args ...any) (*model.User, error) {
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return scanUser(rows)
}

func scanUser(rows *sql.Rows) (*model.User, error) {
	var user model.User
	var verified, darkMode, locale, registerAt, lastSignAt, verification any
	if err := rows.Scan(&user.UID, &user.Email, &user.Nickname, &user.Password,
		&user.Permission, &user.Avatar, &user.Score, &verified, &darkMode,
		&locale, &user.IP, &registerAt, &lastSignAt, &verification); err != nil {
		return nil, err
	}

	user.Locale = stringValue(locale)
	user.Verified = boolValue(verified)
	user.DarkMode = boolValue(darkMode)
	user.RegisterAt = db.TimeString(registerAt)
	user.LastSignAt = db.TimeString(lastSignAt)
	user.Verification = stringValue(verification)

	return &user, nil
}

func (r *UserRepository) CountByEmail(email string) (int, error) {
	return r.count(`SELECT COUNT(*) FROM users WHERE email = ?`, email)
}

func (r *UserRepository) CountByIP(ip string) (int, error) {
	return r.count(`SELECT COUNT(*) FROM users WHERE ip = ?`, ip)
}

func (r *UserRepository) count(query string, args ...any) (int, error) {
	var value int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, query), args...).Scan(&value)
	return value, err
}

// Insert 新建用户（注册/测试/安装用），返回 uid。
func (r *UserRepository) Insert(email, nickname, password, ip string, score int) (int, error) {
	now := php.Now()
	query := `INSERT INTO users
		(email, nickname, score, avatar, password, ip, permission,
		 last_sign_at, register_at, verified, verification_token, remember_token,
		 locale, is_dark_mode)
		VALUES (?,?,?,0,?,?,0,?,?,?, '', NULL, NULL, ?)`

	var insertedID int64
	switch r.Dialect.Name {
	case "pgsql":
		query += " RETURNING uid"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query),
			email, nickname, score, password, ip, now, now, false, false).
			Scan(&insertedID); err != nil {
			return 0, err
		}
	case "sqlsrv":
		query += "; SELECT SCOPE_IDENTITY()"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query),
			email, nickname, score, password, ip, now, now, false, false).
			Scan(&insertedID); err != nil {
			return 0, err
		}
	default:
		result, err := r.DB.Exec(db.Rebind(r.Dialect, query),
			email, nickname, score, password, ip, now, now, false, false)
		if err != nil {
			return 0, err
		}

		insertedID, err = result.LastInsertId()
		if err != nil {
			return 0, err
		}
	}

	return int(insertedID), nil
}

// DeleteWithPlayers 删除用户及其玩家（测试/删号用）。
func (r *UserRepository) DeleteWithPlayers(uid int) error {
	if _, err := r.DB.Exec(db.Rebind(r.Dialect,
		`DELETE FROM players WHERE uid = ?`), uid); err != nil {
		return err
	}

	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`DELETE FROM users WHERE uid = ?`), uid)
	return err
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case int64:
		return typed != 0
	case int32:
		return typed != 0
	case []byte:
		return string(typed) != "0" && !strings.EqualFold(string(typed), "false")
	case string:
		return typed != "0" && !strings.EqualFold(typed, "false")
	}

	return true
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(typed)
	case string:
		return typed
	}

	return fmt.Sprint(value)
}
func (r *UserRepository) UpdateNickname(uid int, nickname string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `UPDATE users SET nickname = ? WHERE uid = ?`),
		nickname, uid)
	return err
}

func (r *UserRepository) UpdatePassword(uid int, password string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `UPDATE users SET password = ? WHERE uid = ?`),
		password, uid)
	return err
}

func (r *UserRepository) UpdateEmail(uid int, email string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE users SET email = ?, verified = ? WHERE uid = ?`), email, false, uid)
	return err
}

func (r *UserRepository) UpdateAvatar(uid, tid int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `UPDATE users SET avatar = ? WHERE uid = ?`),
		tid, uid)
	return err
}

func (r *UserRepository) UpdateDarkMode(uid int, dark bool) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE users SET is_dark_mode = ? WHERE uid = ?`), dark, uid)
	return err
}

func (r *UserRepository) UpdateScoreAndSign(uid, score int, lastSignAt string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE users SET score = ?, last_sign_at = ? WHERE uid = ?`),
		score, lastSignAt, uid)
	return err
}

// Delete 删除账号及其玩家/衣柜记录。
func (r *UserRepository) Delete(uid int) error {
	for _, query := range []string{
		`DELETE FROM user_closet WHERE user_uid = ?`,
		`DELETE FROM players WHERE uid = ?`,
		`DELETE FROM users WHERE uid = ?`,
	} {
		if _, err := r.DB.Exec(db.Rebind(r.Dialect, query), uid); err != nil {
			return err
		}
	}

	return nil
}
func (r *UserRepository) UpdateScore(uid, score int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `UPDATE users SET score = ? WHERE uid = ?`),
		score, uid)
	return err
}
func (r *UserRepository) UpdatePermission(uid, permission int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE users SET permission = ? WHERE uid = ?`), permission, uid)
	return err
}
func (r *UserRepository) UpdateVerified(uid int, verified bool) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE users SET verified = ? WHERE uid = ?`), verified, uid)
	return err
}

func (r *UserRepository) AdminCount(q string) (int, error) {
	query := `SELECT COUNT(*) FROM users`
	args := []any{}
	if q != "" {
		query += ` WHERE email LIKE ? OR nickname LIKE ?`
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, query), args...).Scan(&count)
	return count, err
}

func (r *UserRepository) AdminPage(q string, limit, offset int) ([]*model.User, error) {
	query := `SELECT ` + userColumns + ` FROM users`
	args := []any{}
	if q != "" {
		query += ` WHERE email LIKE ? OR nickname LIKE ?`
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	query += ` ORDER BY uid DESC ` + paging(r.Dialect.Name, limit, offset)

	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*model.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}
func (r *UserRepository) IdsForAudience(receiver string, uid int, email string) ([]int, error) {
	var query string
	var args []any
	switch receiver {
	case "normal":
		query = `SELECT uid FROM users WHERE permission = 0 ORDER BY uid`
	case "uid":
		query = `SELECT uid FROM users WHERE uid = ?`
		args = []any{uid}
	case "email":
		query = `SELECT uid FROM users WHERE email = ?`
		args = []any{email}
	default:
		query = `SELECT uid FROM users ORDER BY uid`
	}
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (r *UserRepository) CountAll() (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, `SELECT COUNT(*) FROM users`)).Scan(&count)
	return count, err
}

func (r *UserRepository) RegisterTimesSince(since string) ([]string, error) {
	rows, err := r.DB.Query(db.Rebind(r.Dialect,
		`SELECT register_at FROM users WHERE register_at >= ?`), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var value any
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, db.TimeString(value))
	}
	return result, rows.Err()
}
