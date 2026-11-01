package repo

import (
	"database/sql"
	"encoding/json"

	"neoskin/internal/db"
	"neoskin/internal/php"
)

type LanguageLine struct {
	ID        int
	Group     string
	Key       string
	Text      string
	CreatedAt string
	UpdatedAt string
}

type LanguageLineRepository struct {
	DB      *sql.DB
	Dialect *db.Dialect
}

func NewLanguageLineRepository(conn *sql.DB, dialect *db.Dialect) *LanguageLineRepository {
	return &LanguageLineRepository{DB: conn, Dialect: dialect}
}

func (r *LanguageLineRepository) Count() (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, `SELECT COUNT(*) FROM language_lines`)).Scan(&count)
	return count, err
}

func (r *LanguageLineRepository) Page(limit, offset int) ([]LanguageLine, error) {
	query := `SELECT id, ` + r.Dialect.Quote("group") + `, ` + r.Dialect.Quote("key") +
		`, text, created_at, updated_at FROM language_lines ORDER BY id DESC ` +
		paging(r.Dialect.Name, limit, offset)
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []LanguageLine
	for rows.Next() {
		var line LanguageLine
		var createdAt, updatedAt any
		if err := rows.Scan(&line.ID, &line.Group, &line.Key, &line.Text,
			&createdAt, &updatedAt); err != nil {
			return nil, err
		}
		line.CreatedAt = db.TimeString(createdAt)
		line.UpdatedAt = db.TimeString(updatedAt)
		result = append(result, line)
	}
	return result, rows.Err()
}

func (r *LanguageLineRepository) Create(group, key, locale, text string) error {
	now := php.Now()
	translations := map[string]string{locale: text}
	encoded, _ := json.Marshal(translations)
	query := `INSERT INTO language_lines (` + r.Dialect.Quote("group") + `, ` +
		r.Dialect.Quote("key") + `, text, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := r.DB.Exec(db.Rebind(r.Dialect, query), group, key, string(encoded), now, now)
	return err
}

func (r *LanguageLineRepository) Find(id int) (*LanguageLine, error) {
	query := `SELECT id, ` + r.Dialect.Quote("group") + `, ` + r.Dialect.Quote("key") +
		`, text, created_at, updated_at FROM language_lines WHERE id = ?`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var line LanguageLine
	var createdAt, updatedAt any
	if err := rows.Scan(&line.ID, &line.Group, &line.Key, &line.Text,
		&createdAt, &updatedAt); err != nil {
		return nil, err
	}
	line.CreatedAt = db.TimeString(createdAt)
	line.UpdatedAt = db.TimeString(updatedAt)
	return &line, nil
}

func (r *LanguageLineRepository) UpdateText(id int, locale, text string) error {
	line, err := r.Find(id)
	if err != nil || line == nil {
		return err
	}
	translations := map[string]string{}
	if line.Text != "" {
		_ = json.Unmarshal([]byte(line.Text), &translations)
	}
	translations[locale] = text
	encoded, _ := json.Marshal(translations)
	_, err = r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE language_lines SET text = ?, updated_at = ? WHERE id = ?`),
		string(encoded), php.Now(), id)
	return err
}

func (r *LanguageLineRepository) Delete(id int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `DELETE FROM language_lines WHERE id = ?`), id)
	return err
}

// Overrides 返回某 locale 的全部数据库覆盖（key 为 "group.key"）。
func (r *LanguageLineRepository) Overrides(locale string) map[string]string {
	result := map[string]string{}
	query := `SELECT ` + r.Dialect.Quote("group") + `, ` + r.Dialect.Quote("key") +
		` , text FROM language_lines`
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query))
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var group, key string
		var text any
		if err := rows.Scan(&group, &key, &text); err != nil {
			continue
		}
		raw := stringValue(text)
		if raw == "" {
			continue
		}
		translations := map[string]string{}
		if json.Unmarshal([]byte(raw), &translations) != nil {
			continue
		}
		if value := translations[locale]; value != "" {
			result[group+"."+key] = value
		}
	}
	return result
}
