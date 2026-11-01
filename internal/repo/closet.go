package repo

import (
	"database/sql"
	"fmt"

	"neoskin/internal/db"
	"neoskin/internal/model"
)

type ClosetRepository struct {
	DB      *sql.DB
	Dialect *db.Dialect
}

func NewClosetRepository(conn *sql.DB, dialect *db.Dialect) *ClosetRepository {
	return &ClosetRepository{DB: conn, Dialect: dialect}
}

// InCloset 判断用户衣柜里是否有该纹理。
func (r *ClosetRepository) InCloset(uid, tid int) (bool, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect,
		`SELECT COUNT(*) FROM user_closet WHERE user_uid = ? AND texture_tid = ?`),
		uid, tid).Scan(&count)
	return count > 0, err
}

func (r *ClosetRepository) Add(uid, tid int, itemName string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`INSERT INTO user_closet (user_uid, texture_tid, item_name) VALUES (?, ?, ?)`),
		uid, tid, itemName)
	return err
}

func (r *ClosetRepository) Remove(uid, tid int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`DELETE FROM user_closet WHERE user_uid = ? AND texture_tid = ?`), uid, tid)
	return err
}

// Item 是 user_closet 关联行 + 纹理。
type Item struct {
	UserUID    int
	TextureTID int
	ItemName   string
	Texture    model.Texture
}

func typeCondition(category string) string {
	if category == "cape" {
		return "t.type = 'cape'"
	}
	return "t.type IN ('steve','alex')"
}

func (r *ClosetRepository) Count(uid int, category, search string) (int, error) {
	query := `SELECT COUNT(*) FROM user_closet c JOIN textures t ON t.tid = c.texture_tid
		WHERE c.user_uid = ? AND ` + typeCondition(category)
	args := []any{uid}
	if search != "" {
		query += ` AND c.item_name LIKE ?`
		args = append(args, "%"+search+"%")
	}
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, query), args...).Scan(&count)
	return count, err
}

func (r *ClosetRepository) Page(uid int, category, search string, limit, offset int) ([]Item, error) {
	query := `SELECT t.tid, t.name, t.type, t.hash, t.` + r.Dialect.Quote("size") +
		`, t.uploader, t.` + r.Dialect.Quote("public") + `, t.upload_at, t.likes,
		c.user_uid, c.texture_tid, c.item_name
		FROM textures t JOIN user_closet c ON t.tid = c.texture_tid
		WHERE c.user_uid = ? AND ` + typeCondition(category)
	args := []any{uid}
	if search != "" {
		query += ` AND c.item_name LIKE ?`
		args = append(args, "%"+search+"%")
	}
	query += ` ORDER BY t.tid DESC ` + paging(r.Dialect.Name, limit, offset)

	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		var public, uploadAt, itemName any
		if err := rows.Scan(&item.Texture.Tid, &item.Texture.Name, &item.Texture.Type,
			&item.Texture.Hash, &item.Texture.Size, &item.Texture.Uploader, &public,
			&uploadAt, &item.Texture.Likes, &item.UserUID, &item.TextureTID,
			&itemName); err != nil {
			return nil, err
		}
		item.Texture.Public = boolValue(public)
		item.Texture.UploadAt = db.TimeString(uploadAt)
		item.ItemName = stringValue(itemName)
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *ClosetRepository) AllIDs(uid int) ([]int, error) {
	rows, err := r.DB.Query(db.Rebind(r.Dialect,
		`SELECT texture_tid FROM user_closet WHERE user_uid = ?`), uid)
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

func (r *ClosetRepository) FindItem(uid, tid int) (*Item, error) {
	query := `SELECT t.tid, t.name, t.type, t.hash, t.` + r.Dialect.Quote("size") +
		`, t.uploader, t.` + r.Dialect.Quote("public") + `, t.upload_at, t.likes,
		c.user_uid, c.texture_tid, c.item_name
		FROM textures t JOIN user_closet c ON t.tid = c.texture_tid
		WHERE c.user_uid = ? AND c.texture_tid = ?`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), uid, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}

	var item Item
	var public, uploadAt, itemName any
	if err := rows.Scan(&item.Texture.Tid, &item.Texture.Name, &item.Texture.Type,
		&item.Texture.Hash, &item.Texture.Size, &item.Texture.Uploader, &public,
		&uploadAt, &item.Texture.Likes, &item.UserUID, &item.TextureTID,
		&itemName); err != nil {
		return nil, err
	}
	item.Texture.Public = boolValue(public)
	item.Texture.UploadAt = db.TimeString(uploadAt)
	item.ItemName = stringValue(itemName)
	return &item, nil
}
func (r *ClosetRepository) RenameItem(uid, tid int, name string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE user_closet SET item_name = ? WHERE user_uid = ? AND texture_tid = ?`),
		name, uid, tid)
	return err
}

func paging(dialect string, limit, offset int) string {
	if dialect == "sqlsrv" {
		return fmt.Sprintf("OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
	}
	return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
}

// AllForUser 返回用户的全部衣柜条目（含皮肤与披风），对应 $user->closet。
func (r *ClosetRepository) AllForUser(uid int) ([]Item, error) {
	query := `SELECT t.tid, t.name, t.type, t.hash, t.` + r.Dialect.Quote("size") +
		`, t.uploader, t.` + r.Dialect.Quote("public") + `, t.upload_at, t.likes,
		c.user_uid, c.texture_tid, c.item_name
		FROM textures t JOIN user_closet c ON t.tid = c.texture_tid
		WHERE c.user_uid = ? ORDER BY t.tid DESC`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var item Item
		var public, uploadAt, itemName any
		if err := rows.Scan(&item.Texture.Tid, &item.Texture.Name, &item.Texture.Type,
			&item.Texture.Hash, &item.Texture.Size, &item.Texture.Uploader, &public,
			&uploadAt, &item.Texture.Likes, &item.UserUID, &item.TextureTID,
			&itemName); err != nil {
			return nil, err
		}
		item.Texture.Public = boolValue(public)
		item.Texture.UploadAt = db.TimeString(uploadAt)
		item.ItemName = stringValue(itemName)
		items = append(items, item)
	}
	return items, rows.Err()
}
