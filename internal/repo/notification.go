package repo

import (
	"database/sql"
	"encoding/json"

	"neoskin/internal/db"
	"neoskin/internal/php"
)

const (
	SiteMessageType = "App\\Notifications\\SiteMessage"
	NotifiableType  = "App\\Models\\User"
)

type NotificationRow struct {
	ID        string
	Type      string
	Data      map[string]any
	ReadAt    string
	CreatedAt string
	UpdatedAt string
}

type NotificationRepository struct {
	DB      *sql.DB
	Dialect *db.Dialect
}

func NewNotificationRepository(conn *sql.DB, dialect *db.Dialect) *NotificationRepository {
	return &NotificationRepository{DB: conn, Dialect: dialect}
}

func (r *NotificationRepository) Insert(notifiableID int, data map[string]any, createdAt string) error {
	encoded, _ := json.Marshal(data)
	query := `INSERT INTO notifications
		(id, type, notifiable_type, notifiable_id, data, read_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NULL, ?, ?)`
	_, err := r.DB.Exec(db.Rebind(r.Dialect, query), RandomUUID(), SiteMessageType,
		NotifiableType, notifiableID, string(encoded), createdAt, createdAt)
	return err
}

func (r *NotificationRepository) CountByUser(uid int) (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect,
		`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`),
		NotifiableType, uid).Scan(&count)
	return count, err
}

func (r *NotificationRepository) PageByUser(uid, limit, offset int) ([]NotificationRow, error) {
	query := `SELECT id, type, data, read_at, created_at, updated_at FROM notifications
		WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC ` +
		paging(r.Dialect.Name, limit, offset)
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), NotifiableType, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []NotificationRow
	for rows.Next() {
		var row NotificationRow
		var data, readAt, createdAt, updatedAt any
		if err := rows.Scan(&row.ID, &row.Type, &data, &readAt, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		row.Data = parseJSONMap(stringValue(data))
		row.ReadAt = db.TimeString(readAt)
		row.CreatedAt = db.TimeString(createdAt)
		row.UpdatedAt = db.TimeString(updatedAt)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *NotificationRepository) FindByUser(uid int, id string) (*NotificationRow, error) {
	query := `SELECT id, type, data, read_at, created_at, updated_at FROM notifications
		WHERE notifiable_type = ? AND notifiable_id = ? AND id = ?`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), NotifiableType, uid, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var row NotificationRow
	var data, readAt, createdAt, updatedAt any
	if err := rows.Scan(&row.ID, &row.Type, &data, &readAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	row.Data = parseJSONMap(stringValue(data))
	row.ReadAt = db.TimeString(readAt)
	row.CreatedAt = db.TimeString(createdAt)
	row.UpdatedAt = db.TimeString(updatedAt)
	return &row, nil
}

func (r *NotificationRepository) MarkRead(uid int, id string) error {
	now := php.Now()
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE notifications SET read_at = ?, updated_at = ?
		 WHERE notifiable_type = ? AND notifiable_id = ? AND id = ?`),
		now, now, NotifiableType, uid, id)
	return err
}

func parseJSONMap(raw string) map[string]any {
	result := map[string]any{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &result)
	}
	return result
}
