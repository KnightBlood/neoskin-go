package repo

import (
	"database/sql"
	"strings"

	"neoskin/internal/db"
	"neoskin/internal/model"
)

type TextureRepository struct {
	DB      *sql.DB
	Dialect *db.Dialect
}

func NewTextureRepository(conn *sql.DB, dialect *db.Dialect) *TextureRepository {
	return &TextureRepository{DB: conn, Dialect: dialect}
}

func (r *TextureRepository) columns() string {
	return "tid, name, type, hash, size, uploader, " + r.Dialect.Quote("public") + ", upload_at, likes"
}

func (r *TextureRepository) FindByTid(tid int) (*model.Texture, error) {
	return r.findOne(`SELECT `+r.columns()+` FROM textures WHERE tid = ?`, tid)
}

func (r *TextureRepository) FindByHash(hash string) (*model.Texture, error) {
	return r.findOne(`SELECT `+r.columns()+` FROM textures WHERE hash = ?`, hash)
}

func (r *TextureRepository) DeleteByTid(tid int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `DELETE FROM textures WHERE tid = ?`), tid)
	return err
}

// Insert 新增纹理（上传/测试用），返回 tid。
func (r *TextureRepository) Insert(name, textureType, hash string, size, uploader int,
	isPublic bool, uploadAt string) (int, error) {
	query := `INSERT INTO textures
		(name, type, hash, size, uploader, ` + r.Dialect.Quote("public") + `, upload_at, likes)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)`
	publicValue := 0
	if isPublic {
		publicValue = 1
	}

	var id int64
	switch r.Dialect.Name {
	case "pgsql":
		query += " RETURNING tid"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), name, textureType, hash,
			size, uploader, publicValue, uploadAt).Scan(&id); err != nil {
			return 0, err
		}
	case "sqlsrv":
		query += "; SELECT SCOPE_IDENTITY()"
		if err := r.DB.QueryRow(db.Rebind(r.Dialect, query), name, textureType, hash,
			size, uploader, publicValue, uploadAt).Scan(&id); err != nil {
			return 0, err
		}
	default:
		result, err := r.DB.Exec(db.Rebind(r.Dialect, query), name, textureType, hash,
			size, uploader, publicValue, uploadAt)
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

func (r *TextureRepository) findOne(query string, args ...any) (*model.Texture, error) {
	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	var texture model.Texture
	var public, uploadAt any
	if err := rows.Scan(&texture.Tid, &texture.Name, &texture.Type, &texture.Hash,
		&texture.Size, &texture.Uploader, &public, &uploadAt, &texture.Likes); err != nil {
		return nil, err
	}
	texture.Public = boolValue(public)
	texture.UploadAt = db.TimeString(uploadAt)

	return &texture, nil
}
func (r *TextureRepository) SumSizeByUploader(uid int) (int, error) {
	var size int
	err := r.DB.QueryRow(db.Rebind(r.Dialect,
		`SELECT COALESCE(SUM(`+r.Dialect.Quote("size")+`), 0) FROM textures WHERE uploader = ?`),
		uid).Scan(&size)
	return size, err
}
func (r *TextureRepository) AddLikes(tid, delta int) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE textures SET likes = likes + ? WHERE tid = ?`), delta, tid)
	return err
}

type LibraryRow struct {
	TID      int
	Name     string
	Type     string
	Uploader int
	Public   bool
	Likes    int
	Nickname string
}

func (r *TextureRepository) LibraryCount(textureType, keyword string, uploader, viewerUID int, admin bool) (int, error) {
	where, args := r.libraryWhere(textureType, keyword, uploader, viewerUID, admin)
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, `SELECT COUNT(*) FROM textures t WHERE 1=1`+where),
		args...).Scan(&count)
	return count, err
}

func (r *TextureRepository) LibraryPage(textureType, keyword, sortBy string, uploader, viewerUID int,
	admin bool, limit, offset int) ([]LibraryRow, error) {
	where, args := r.libraryWhere(textureType, keyword, uploader, viewerUID, admin)
	column := "upload_at"
	switch sortBy {
	case "likes":
		column = "likes"
	case "name":
		column = "name"
	case "tid":
		column = "tid"
	}
	query := `SELECT t.tid, t.name, t.type, t.uploader, t.` + r.Dialect.Quote("public") +
		`, t.likes, u.nickname
		FROM textures t LEFT JOIN users u ON u.uid = t.uploader
		WHERE 1=1` + where + ` ORDER BY t.` + r.Dialect.Quote(column) + ` DESC ` +
		paging(r.Dialect.Name, limit, offset)

	rows, err := r.DB.Query(db.Rebind(r.Dialect, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []LibraryRow
	for rows.Next() {
		var row LibraryRow
		var public, nickname any
		if err := rows.Scan(&row.TID, &row.Name, &row.Type, &row.Uploader, &public,
			&row.Likes, &nickname); err != nil {
			return nil, err
		}
		row.Public = boolValue(public)
		row.Nickname = stringValue(nickname)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r *TextureRepository) libraryWhere(textureType, keyword string, uploader, viewerUID int,
	admin bool) (string, []any) {
	var builder strings.Builder
	var args []any
	if textureType == "skin" {
		builder.WriteString(` AND t.type IN ('steve','alex')`)
	} else if textureType != "" {
		builder.WriteString(` AND t.type = ?`)
		args = append(args, textureType)
	}
	if keyword != "" {
		builder.WriteString(` AND t.name LIKE ?`)
		args = append(args, "%"+keyword+"%")
	}
	if uploader > 0 {
		builder.WriteString(` AND t.uploader = ?`)
		args = append(args, uploader)
	}
	if !admin {
		if viewerUID > 0 {
			builder.WriteString(` AND (t.` + r.Dialect.Quote("public") + ` = 1 OR t.uploader = ?)`)
			args = append(args, viewerUID)
		} else {
			builder.WriteString(` AND t.` + r.Dialect.Quote("public") + ` = 1`)
		}
	}
	return builder.String(), args
}

func (r *TextureRepository) UpdateName(tid int, name string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `UPDATE textures SET name = ? WHERE tid = ?`), name, tid)
	return err
}

func (r *TextureRepository) UpdatePublic(tid int, public bool) error {
	value := 0
	if public {
		value = 1
	}
	_, err := r.DB.Exec(db.Rebind(r.Dialect,
		`UPDATE textures SET `+r.Dialect.Quote("public")+` = ? WHERE tid = ?`), value, tid)
	return err
}

func (r *TextureRepository) UpdateType(tid int, textureType string) error {
	_, err := r.DB.Exec(db.Rebind(r.Dialect, `UPDATE textures SET type = ? WHERE tid = ?`),
		textureType, tid)
	return err
}
func (r *TextureRepository) CountByHash(hash string) (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect,
		`SELECT COUNT(*) FROM textures WHERE hash = ?`), hash).Scan(&count)
	return count, err
}
func (r *TextureRepository) FindPublicOrMine(hash string, uid int) (*model.Texture, error) {
	query := `SELECT ` + r.columns() + ` FROM textures
		WHERE hash = ? AND (` + r.Dialect.Quote("public") + ` = 1 OR uploader = ?)`
	return r.findOne(query, hash, uid)
}
func (r *TextureRepository) CountAll() (int, error) {
	var count int
	err := r.DB.QueryRow(db.Rebind(r.Dialect, `SELECT COUNT(*) FROM textures`)).Scan(&count)
	return count, err
}

func (r *TextureRepository) SumSize() (int, error) {
	var size int
	err := r.DB.QueryRow(db.Rebind(r.Dialect,
		`SELECT COALESCE(SUM(`+r.Dialect.Quote("size")+`), 0) FROM textures`)).Scan(&size)
	return size, err
}

func (r *TextureRepository) UploadTimesSince(since string) ([]string, error) {
	rows, err := r.DB.Query(db.Rebind(r.Dialect,
		`SELECT upload_at FROM textures WHERE upload_at >= ?`), since)
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
