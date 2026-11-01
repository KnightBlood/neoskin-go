package repo

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"neoskin/internal/db"
	"neoskin/internal/php"
)

const (
	CommunicationAnnouncement = "announcement"
	CommunicationMessage      = "message"
	CommunicationBoth         = "both"
	CommunicationDraft        = "draft"
	CommunicationPublished    = "published"
	CommunicationWithdrawn    = "withdrawn"
)

type CommunicationRow struct {
	ID            int
	AuthorID      int
	LegacyKey     string
	DeliveryToken string
	Title         string
	Kind          string
	Status        string
	Locale        string
	Editor        string
	Source        string
	Blocks        string
	Audience      string
	PublishedAt   string
	CreatedAt     string
	UpdatedAt     string
}

type CommunicationRepository struct {
	DB      *sql.DB
	Dialect *db.Dialect
}

func NewCommunicationRepository(conn *sql.DB, dialect *db.Dialect) *CommunicationRepository {
	return &CommunicationRepository{DB: conn, Dialect: dialect}
}

func (r *CommunicationRepository) Insert(row CommunicationRow) (int, error) {
	query := `INSERT INTO communications
		(author_id, legacy_key, delivery_token, title, kind, status, locale, editor,
		 source, blocks, audience, published_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	var id int64
	args := []any{row.AuthorID, nullString(row.LegacyKey), nullString(row.DeliveryToken),
		row.Title, row.Kind, row.Status, row.Locale, row.Editor, nullString(row.Source),
		nullString(row.Blocks), nullString(row.Audience), nullString(row.PublishedAt),
		nullString(row.CreatedAt), nullString(row.UpdatedAt)}
	switch r.Dialect.Name {
	case "pgsql":
		query += " RETURNING id"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), args...).Scan(&id); err != nil {
			return 0, err
		}
	case "sqlsrv":
		query += "; SELECT SCOPE_IDENTITY()"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), args...).Scan(&id); err != nil {
			return 0, err
		}
	default:
		result, err := r.DB.Exec(db.Rebind(r.Dialect, query), args...)
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

func (r *CommunicationRepository) Update(row CommunicationRow) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `UPDATE communications SET
		title = ?, kind = ?, status = ?, locale = ?, editor = ?, source = ?, blocks = ?,
		audience = ?, published_at = ?, updated_at = ? WHERE id = ?`),
		row.Title, row.Kind, row.Status, row.Locale, row.Editor, nullString(row.Source),
		nullString(row.Blocks), nullString(row.Audience), nullString(row.PublishedAt),
		nullString(row.UpdatedAt), row.ID)
	return err
}

func (r *CommunicationRepository) Withdraw(id int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE communications SET status = ?, updated_at = ? WHERE id = ?`),
		CommunicationWithdrawn, php.Now(), id)
	return err
}

func (r *CommunicationRepository) Find(id int) (*CommunicationRow, error) {
	return r.findOne(`WHERE id = ?`, id)
}

func (r *CommunicationRepository) FindLatestAnnouncement(locale string) (*CommunicationRow, error) {
	return r.findOne(`WHERE kind = ? AND locale = ? ORDER BY id DESC`, CommunicationAnnouncement, locale)
}

func (r *CommunicationRepository) findOne(where string, args ...any) (*CommunicationRow, error) {
	query := `SELECT id, author_id, legacy_key, delivery_token, title, kind, status, locale,
		editor, source, blocks, audience, published_at, created_at, updated_at
		FROM communications ` + where + ` LIMIT 1`
	// SQL Server 不支持 LIMIT；用 TOP 1（这里简单改写）。
	if r.Dialect.Name == "sqlsrv" {
		query = `SELECT TOP 1 id, author_id, legacy_key, delivery_token, title, kind, status,
			locale, editor, source, blocks, audience, published_at, created_at, updated_at
			FROM communications ` + where
	}
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanCommunication(rows)
}

func (r *CommunicationRepository) CountMessages() (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect,
		`SELECT COUNT(*) FROM communications WHERE kind IN (?, ?)`),
		CommunicationMessage, CommunicationBoth).Scan(&count)
	return count, err
}

func (r *CommunicationRepository) PageMessages(limit, offset int) ([]CommunicationRow, error) {
	query := `SELECT id, author_id, legacy_key, delivery_token, title, kind, status, locale,
		editor, source, blocks, audience, published_at, created_at, updated_at
		FROM communications WHERE kind IN (?, ?) ORDER BY id DESC ` +
		paging(r.Dialect.Name, limit, offset)
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), CommunicationMessage, CommunicationBoth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []CommunicationRow
	for rows.Next() {
		row, err := scanCommunication(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *row)
	}
	return result, rows.Err()
}

func scanCommunication(rows *sql.Rows) (*CommunicationRow, error) {
	var row CommunicationRow
	var legacyKey, deliveryToken, source, blocks, audience, publishedAt, createdAt, updatedAt any
	if err := rows.Scan(&row.ID, &row.AuthorID, &legacyKey, &deliveryToken, &row.Title,
		&row.Kind, &row.Status, &row.Locale, &row.Editor, &source, &blocks, &audience,
		&publishedAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	row.LegacyKey = stringValue(legacyKey)
	row.DeliveryToken = stringValue(deliveryToken)
	row.Source = stringValue(source)
	row.Blocks = stringValue(blocks)
	row.Audience = stringValue(audience)
	row.PublishedAt = db.TimeString(publishedAt)
	row.CreatedAt = db.TimeString(createdAt)
	row.UpdatedAt = db.TimeString(updatedAt)
	return &row, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func RandomHex(length int) string {
	buffer := make([]byte, (length+1)/2)
	_, _ = rand.Read(buffer)
	return hex.EncodeToString(buffer)[:length]
}

// RandomUUID 生成带连字符的 UUID v4（notifications.id 契约）。
func RandomUUID() string {
	buffer := make([]byte, 16)
	_, _ = rand.Read(buffer)
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(buffer)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:32]
}

// DeliveryCount 统计某条通信投递的站内信数量（notifications.data.communication_id）。
func (r *CommunicationRepository) DeliveryCount(id int) int {
	query := `SELECT COUNT(*) FROM notifications WHERE data LIKE ?`
	var count int
	_ = r.DB.QueryRow(db.Rebind(r.Dialect, query), "%\"communication_id\":"+itoa(id)+"%").Scan(&count)
	return count
}

// ReadCount 统计某条通信已读的站内信数量。
func (r *CommunicationRepository) ReadCount(id int) int {
	query := `SELECT COUNT(*) FROM notifications WHERE data LIKE ? AND read_at IS NOT NULL`
	var count int
	_ = r.DB.QueryRow(db.Rebind(r.Dialect, query), "%\"communication_id\":"+itoa(id)+"%").Scan(&count)
	return count
}

func itoa(value int) string {
	return fmt.Sprint(value)
}
