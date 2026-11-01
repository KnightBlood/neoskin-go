package repo

import (
	"database/sql"

	"neoskin/internal/db"
	"neoskin/internal/model"
	"neoskin/internal/php"
)

type PlayerRepository struct {
	DB      *sql.DB
	Dialect *db.Dialect
}

func NewPlayerRepository(conn *sql.DB, dialect *db.Dialect) *PlayerRepository {
	return &PlayerRepository{DB: conn, Dialect: dialect}
}

// FindDetailByName 返回玩家 + 所属用户权限（对应 Player::where('name')->firstOrFail）。
func (r *PlayerRepository) FindDetailByName(name string) (*model.PlayerDetail, error) {
	query := `SELECT p.pid, p.uid, p.name, p.tid_skin, p.tid_cape, p.last_modified, u.permission
		FROM players p JOIN users u ON u.uid = p.uid
		WHERE p.name = ?`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	var detail model.PlayerDetail
	var lastModified any
	if err := rows.Scan(&detail.PID, &detail.UID, &detail.Name, &detail.TidSkin,
		&detail.TidCape, &lastModified, &detail.Permission); err != nil {
		return nil, err
	}
	detail.LastModified = db.TimeString(lastModified)

	return &detail, nil
}

func (r *PlayerRepository) ListByUID(uid int) ([]model.Player, error) {
	query := `SELECT pid, uid, name, tid_skin, tid_cape, last_modified
		FROM players WHERE uid = ? ORDER BY pid`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []model.Player
	for rows.Next() {
		var player model.Player
		var lastModified any
		if err := rows.Scan(&player.PID, &player.UID, &player.Name, &player.TidSkin,
			&player.TidCape, &lastModified); err != nil {
			return nil, err
		}
		player.LastModified = db.TimeString(lastModified)
		players = append(players, player)
	}

	return players, rows.Err()
}

// Insert 新建玩家（默认 tid_skin=-1；注册流程用 InsertWithSkin(…, 0)）。
func (r *PlayerRepository) Insert(uid int, name string) (int, error) {
	return r.InsertWithSkin(uid, name, -1)
}

// InsertWithSkin 新建玩家并指定 tid_skin。
func (r *PlayerRepository) InsertWithSkin(uid int, name string, tidSkin int) (int, error) {
	now := php.Now()
	query := `INSERT INTO players (uid, name, tid_skin, tid_cape, last_modified)
		VALUES (?, ?, ?, 0, ?)`

	var id int64
	switch r.Dialect.Name {
	case "pgsql":
		query += " RETURNING pid"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), uid, name, tidSkin, now).
			Scan(&id); err != nil {
			return 0, err
		}
	case "sqlsrv":
		query += "; SELECT SCOPE_IDENTITY()"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), uid, name, tidSkin, now).
			Scan(&id); err != nil {
			return 0, err
		}
	default:
		result, err := r.DB.Exec(db.Rebind(r.Dialect, query), uid, name, tidSkin, now)
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

func (r *PlayerRepository) UpdateTextures(pid, tidSkin, tidCape int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE players SET tid_skin = ?, tid_cape = ?, last_modified = ? WHERE pid = ?`),
		tidSkin, tidCape, php.Now(), pid)
	return err
}

func (r *PlayerRepository) Delete(pid int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `DELETE FROM players WHERE pid = ?`), pid)
	return err
}
func (r *PlayerRepository) CountByUID(uid int) (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect,
		`SELECT COUNT(*) FROM players WHERE uid = ?`), uid).Scan(&count)
	return count, err
}
func (r *PlayerRepository) FindByID(pid int) (*model.Player, error) {
	query := `SELECT pid, uid, name, tid_skin, tid_cape, last_modified FROM players WHERE pid = ?`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var player model.Player
	var lastModified any
	if err := rows.Scan(&player.PID, &player.UID, &player.Name, &player.TidSkin,
		&player.TidCape, &lastModified); err != nil {
		return nil, err
	}
	player.LastModified = db.TimeString(lastModified)
	return &player, nil
}

func (r *PlayerRepository) UpdateName(pid int, name string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE players SET name = ?, last_modified = ? WHERE pid = ?`), name, php.Now(), pid)
	return err
}
func (r *PlayerRepository) FindDetailByID(pid int) (*model.PlayerDetail, error) {
	query := `SELECT p.pid, p.uid, p.name, p.tid_skin, p.tid_cape, p.last_modified, u.permission
		FROM players p JOIN users u ON u.uid = p.uid WHERE p.pid = ?`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var detail model.PlayerDetail
	var lastModified any
	if err := rows.Scan(&detail.PID, &detail.UID, &detail.Name, &detail.TidSkin,
		&detail.TidCape, &lastModified, &detail.Permission); err != nil {
		return nil, err
	}
	detail.LastModified = db.TimeString(lastModified)
	return &detail, nil
}

func (r *PlayerRepository) AdminCount(q string) (int, error) {
	query := `SELECT COUNT(*) FROM players`
	args := []any{}
	if q != "" {
		query += ` WHERE name LIKE ?`
		args = append(args, "%"+q+"%")
	}
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, query), args...).Scan(&count)
	return count, err
}

func (r *PlayerRepository) AdminPage(q string, limit, offset int) ([]model.Player, error) {
	query := `SELECT pid, uid, name, tid_skin, tid_cape, last_modified FROM players`
	args := []any{}
	if q != "" {
		query += ` WHERE name LIKE ?`
		args = append(args, "%"+q+"%")
	}
	query += ` ORDER BY pid DESC ` + paging(r.Dialect.Name, limit, offset)
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.Player
	for rows.Next() {
		var player model.Player
		var lastModified any
		if err := rows.Scan(&player.PID, &player.UID, &player.Name, &player.TidSkin,
			&player.TidCape, &lastModified); err != nil {
			return nil, err
		}
		player.LastModified = db.TimeString(lastModified)
		result = append(result, player)
	}
	return result, rows.Err()
}

func (r *PlayerRepository) UpdateOwner(pid, uid int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE players SET uid = ?, last_modified = ? WHERE pid = ?`), uid, php.Now(), pid)
	return err
}
func (r *PlayerRepository) CountAll() (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, `SELECT COUNT(*) FROM players`)).Scan(&count)
	return count, err
}
