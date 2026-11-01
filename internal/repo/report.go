package repo

import (
	"database/sql"

	"neoskin/internal/db"
	"neoskin/internal/php"
)

const (
	ReportPending  = 0
	ReportResolved = 1
	ReportRejected = 2
)

type ReportRow struct {
	ID       int
	TID      int
	Uploader int
	Reporter int
	Reason   string
	Status   int
	ReportAt string
}

type ReportRepository struct {
	DB      *sql.DB
	Dialect *db.Dialect
}

func NewReportRepository(conn *sql.DB, dialect *db.Dialect) *ReportRepository {
	return &ReportRepository{DB: conn, Dialect: dialect}
}

func (r *ReportRepository) Insert(tid, uploader, reporter int, reason string) (int, error) {
	query := `INSERT INTO reports (tid, uploader, reporter, reason, status, report_at)
		VALUES (?, ?, ?, ?, ?, ?)`
	var id int64
	switch r.Dialect.Name {
	case "pgsql":
		query += " RETURNING id"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), tid, uploader, reporter,
			reason, ReportPending, php.Now()).Scan(&id); err != nil {
			return 0, err
		}
	case "sqlsrv":
		query += "; SELECT SCOPE_IDENTITY()"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), tid, uploader, reporter,
			reason, ReportPending, php.Now()).Scan(&id); err != nil {
			return 0, err
		}
	default:
		result, err := r.DB.Exec(db.Rebind(r.Dialect, query), tid, uploader, reporter,
			reason, ReportPending, php.Now())
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

func (r *ReportRepository) Exists(reporter, tid int) (bool, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect,
		`SELECT COUNT(*) FROM reports WHERE reporter = ? AND tid = ?`), reporter, tid).Scan(&count)
	return count > 0, err
}

func (r *ReportRepository) Find(id int) (*ReportRow, error) {
	rows, err := r.DB.Query(db.Rebind(r.Dialect,
		`SELECT id, tid, uploader, reporter, reason, status, report_at
		 FROM reports WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var row ReportRow
	var reportAt any
	if err := rows.Scan(&row.ID, &row.TID, &row.Uploader, &row.Reporter, &row.Reason,
		&row.Status, &reportAt); err != nil {
		return nil, err
	}
	row.ReportAt = db.TimeString(reportAt)
	return &row, nil
}

func (r *ReportRepository) UpdateStatus(id, status int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE reports SET status = ? WHERE id = ?`), status, id)
	return err
}

func (r *ReportRepository) CountByReporter(uid int) (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect,
		`SELECT COUNT(*) FROM reports WHERE reporter = ?`), uid).Scan(&count)
	return count, err
}

func (r *ReportRepository) PageByReporter(uid, limit, offset int) ([]ReportRow, error) {
	return r.page(`WHERE reporter = ?`, []any{uid}, limit, offset)
}

func (r *ReportRepository) CountAll() (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, `SELECT COUNT(*) FROM reports`)).Scan(&count)
	return count, err
}

func (r *ReportRepository) PageAll(limit, offset int) ([]ReportRow, error) {
	return r.page("", nil, limit, offset)
}

func (r *ReportRepository) page(where string, args []any, limit, offset int) ([]ReportRow, error) {
	query := `SELECT id, tid, uploader, reporter, reason, status, report_at
		FROM reports ` + where + ` ORDER BY report_at DESC ` + paging(r.Dialect.Name, limit, offset)
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ReportRow
	for rows.Next() {
		var row ReportRow
		var reportAt any
		if err := rows.Scan(&row.ID, &row.TID, &row.Uploader, &row.Reporter, &row.Reason,
			&row.Status, &reportAt); err != nil {
			return nil, err
		}
		row.ReportAt = db.TimeString(reportAt)
		result = append(result, row)
	}
	return result, rows.Err()
}
