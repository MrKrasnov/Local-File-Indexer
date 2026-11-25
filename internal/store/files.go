package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Meta struct {
	Path      string
	Name      string
	Size      int64
	Mode      string
	ModTime   time.Time
	IsDir     bool
	Extension string
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
	var meta Meta
	var modTime string
	var isDir int
	err := db.sql.QueryRowContext(ctx, `
		SELECT path, name, size, mode, mod_time, is_dir, extension
		FROM files
		WHERE path = ?
	`, path).Scan(&meta.Path, &meta.Name, &meta.Size, &meta.Mode, &modTime, &isDir, &meta.Extension)
	if err != nil {
		return Meta{}, err
	}
	meta.ModTime, err = time.Parse(time.RFC3339Nano, modTime)
	if err != nil {
		return Meta{}, fmt.Errorf("parse mod_time for %s: %w", path, err)
	}
	meta.IsDir = isDir != 0
	return meta, nil
}

func childPattern(root string) string {
	var b strings.Builder
	b.Grow(len(root) + 2)
	for i := 0; i < len(root); i++ {
		switch root[i] {
		case '\\', '%', '_':
			b.WriteByte('\\')
		}
		b.WriteByte(root[i])
	}
	b.WriteString("/%")
	return b.String()
}
