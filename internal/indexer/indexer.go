package indexer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"local-file-indexer/internal/store"
)

const maxReportedErrors = 100

var ErrNotAbsolute = errors.New("path must be absolute")

type Result struct {
	Path       string   `json:"path"`
	Saved      int      `json:"saved"`
	Removed    int      `json:"removed"`
	ErrorCount int      `json:"error_count"`
	Errors     []string `json:"errors"`
}

// Index walks root and stores filesystem metadata for every entry.
// Nested symlinks are recorded and not followed. A symlink passed as root is followed.
// Previously indexed entries under root are removed only when the walk finishes without errors.
func Index(ctx context.Context, db *store.DB, root string) (Result, error) {
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return Result{}, ErrNotAbsolute
	}

	info, err := os.Stat(root)
	if err != nil {
		return Result{}, err
	}

	tx, err := db.BeginScan(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()

	result := Result{Path: root, Errors: []string{}}
	s := &scanner{tx: tx, result: &result}
	if err := s.walk(ctx, root, info); err != nil {
		return Result{}, err
	}

	if result.ErrorCount == 0 {
		removed, err := tx.DeleteStale(ctx, root)
		if err != nil {
			return Result{}, err
		}
		result.Removed = int(removed)
	}

	if err := tx.Commit(); err != nil {
		return Result{}, err
	}
	return result, nil
}

type scanner struct {
	tx     *store.Tx
	result *Result
}

func (s *scanner) walk(ctx context.Context, path string, info fs.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	ext := ""
	if !info.IsDir() {
		ext = filepath.Ext(info.Name())
	}
	if err := s.tx.Upsert(ctx, store.Meta{
		Path:      path,
		Name:      info.Name(),
		Size:      info.Size(),
		Mode:      info.Mode().String(),
		ModTime:   info.ModTime(),
		IsDir:     info.IsDir(),
		Extension: ext,
	}); err != nil {
		return err
	}
	s.result.Saved++

	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		s.addErr(fmt.Sprintf("%s: %s", path, err))
		return nil
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		child := filepath.Join(path, entry.Name())
		childInfo, err := entry.Info()
		if err != nil {
			s.addErr(fmt.Sprintf("%s: %s", child, err))
			continue
		}
		if err := s.walk(ctx, child, childInfo); err != nil {
			return err
		}
	}
	return nil
}

func (s *scanner) addErr(msg string) {
	s.result.ErrorCount++
	if len(s.result.Errors) < maxReportedErrors {
		s.result.Errors = append(s.result.Errors, msg)
	}
}
