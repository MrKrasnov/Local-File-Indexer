package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Meta struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Mode      string    `json:"mode"`
	ModTime   time.Time `json:"mod_time"`
	IsDir     bool      `json:"is_dir"`
	Extension string    `json:"extension"`
}

type Tx struct {
	tx   *sql.Tx
	stmt *sql.Stmt
	at   string
}

func (db *DB) BeginScan(ctx context.Context) (*Tx, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO files (path, name, size, mode, mod_time, is_dir, extension, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name = excluded.name,
			size = excluded.size,
			mode = excluded.mode,
			mod_time = excluded.mod_time,
			is_dir = excluded.is_dir,
			extension = excluded.extension,
			scanned_at = excluded.scanned_at
	`)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return &Tx{
		tx:   tx,
		stmt: stmt,
		at:   time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func (t *Tx) Upsert(ctx context.Context, meta Meta) error {
	isDir := 0
	if meta.IsDir {
		isDir = 1
	}
	_, err := t.stmt.ExecContext(
		ctx,
		meta.Path,
		meta.Name,
		meta.Size,
		meta.Mode,
		meta.ModTime.UTC().Format(time.RFC3339Nano),
		isDir,
		meta.Extension,
		t.at,
	)
	return err
}

// DeleteStale removes rows under root that were not written by this scan.
func (t *Tx) DeleteStale(ctx context.Context, root string) (int64, error) {
	res, err := t.tx.ExecContext(ctx, `
		DELETE FROM files
		WHERE scanned_at <> ?
		  AND (path = ? OR path LIKE ? ESCAPE '\')
	`, t.at, root, childPattern(root))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (t *Tx) Commit() error {
	if err := t.stmt.Close(); err != nil {
		_ = t.tx.Rollback()
		return err
	}
	t.stmt = nil
	return t.tx.Commit()
}

func (t *Tx) Rollback() {
	if t.stmt != nil {
		_ = t.stmt.Close()
		t.stmt = nil
	}
	_ = t.tx.Rollback()
}

func (db *DB) Get(ctx context.Context, path string) (Meta, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT path, name, size, mode, mod_time, is_dir, extension
		FROM files
		WHERE path = ?
	`, path)
	return scanMeta(row)
}

func (db *DB) Search(ctx context.Context, keyword string, limit int) ([]Meta, error) {
	query := `
		SELECT path, name, size, mode, mod_time, is_dir, extension
		FROM files
	`
	args := make([]any, 0, 3)
	if keyword != "" {
		pattern := "%" + escapeLike(keyword) + "%"
		query += ` WHERE name LIKE ? ESCAPE '\' OR path LIKE ? ESCAPE '\'`
		args = append(args, pattern, pattern)
	}
	query += ` ORDER BY path LIMIT ?`
	args = append(args, limit)

	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := []Meta{}
	for rows.Next() {
		meta, err := scanMeta(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, meta)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanMeta(row scanner) (Meta, error) {
	var meta Meta
	var modTime string
	var isDir int
	err := row.Scan(&meta.Path, &meta.Name, &meta.Size, &meta.Mode, &modTime, &isDir, &meta.Extension)
	if err != nil {
		return Meta{}, err
	}
	meta.ModTime, err = time.Parse(time.RFC3339Nano, modTime)
	if err != nil {
		return Meta{}, fmt.Errorf("parse mod_time for %s: %w", meta.Path, err)
	}
	meta.IsDir = isDir != 0
	return meta, nil
}

func childPattern(root string) string {
	return escapeLike(root) + "/%"
}

func escapeLike(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\\', '%', '_':
			b.WriteByte('\\')
		}
		b.WriteByte(value[i])
	}
	return b.String()
}
