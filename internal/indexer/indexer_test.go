package indexer

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"local-file-indexer/internal/store"
)

func openDB(test *testing.T) *store.DB {
	test.Helper()
	db, err := store.Open(filepath.Join(test.TempDir(), "index.db"))
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = db.Close() })
	return db
}

func TestIndexStoresMetadataAndDropsMissing(test *testing.T) {
	dir := test.TempDir()
	root := filepath.Join(dir, "docs")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		test.Fatal(err)
	}
	aPath := filepath.Join(root, "a.txt")
	if err := os.WriteFile(aPath, []byte("hello"), 0o644); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.md"), []byte("x"), 0o644); err != nil {
		test.Fatal(err)
	}

	decoy := filepath.Join(dir, "docs-old")
	if err := os.MkdirAll(decoy, 0o755); err != nil {
		test.Fatal(err)
	}
	decoyFile := filepath.Join(decoy, "c.txt")
	if err := os.WriteFile(decoyFile, []byte("c"), 0o644); err != nil {
		test.Fatal(err)
	}

	underscore := filepath.Join(dir, "a_b")
	if err := os.MkdirAll(underscore, 0o755); err != nil {
		test.Fatal(err)
	}
	underscoreFile := filepath.Join(underscore, "u.txt")
	if err := os.WriteFile(underscoreFile, []byte("u"), 0o644); err != nil {
		test.Fatal(err)
	}
	likeSibling := filepath.Join(dir, "aXb")
	if err := os.MkdirAll(likeSibling, 0o755); err != nil {
		test.Fatal(err)
	}
	likeSiblingFile := filepath.Join(likeSibling, "s.txt")
	if err := os.WriteFile(likeSiblingFile, []byte("s"), 0o644); err != nil {
		test.Fatal(err)
	}

	db := openDB(test)
	ctx := context.Background()

	res, err := Index(ctx, db, root)
	if err != nil {
		test.Fatal(err)
	}
	if res.Saved != 4 {
		test.Fatalf("saved=%d, want 4", res.Saved)
	}
	if res.Removed != 0 {
		test.Fatalf("removed=%d, want 0", res.Removed)
	}

	meta, err := db.Get(ctx, aPath)
	if err != nil {
		test.Fatal(err)
	}
	if meta.Name != "a.txt" || meta.Size != 5 || meta.Extension != ".txt" || meta.IsDir {
		test.Fatalf("metadata=%+v", meta)
	}

	if _, err := Index(ctx, db, decoy); err != nil {
		test.Fatal(err)
	}
	if _, err := Index(ctx, db, underscore); err != nil {
		test.Fatal(err)
	}
	if _, err := Index(ctx, db, likeSibling); err != nil {
		test.Fatal(err)
	}

	if err := os.Remove(aPath); err != nil {
		test.Fatal(err)
	}
	res, err = Index(ctx, db, root)
	if err != nil {
		test.Fatal(err)
	}
	if res.Saved != 3 {
		test.Fatalf("saved=%d, want 3", res.Saved)
	}
	if res.Removed != 1 {
		test.Fatalf("removed=%d, want 1", res.Removed)
	}

	if _, err := db.Get(ctx, aPath); err != sql.ErrNoRows {
		test.Fatalf("deleted file err=%v", err)
	}
	if _, err := db.Get(ctx, decoyFile); err != nil {
		test.Fatal(err)
	}
	if _, err := db.Get(ctx, likeSiblingFile); err != nil {
		test.Fatal(err)
	}
}

func TestIndexFollowsRootSymlink(test *testing.T) {
	dir := test.TempDir()
	root := filepath.Join(dir, "docs")
	if err := os.MkdirAll(root, 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		test.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(root, link); err != nil {
		test.Fatal(err)
	}

	res, err := Index(context.Background(), openDB(test), link)
	if err != nil {
		test.Fatal(err)
	}
	if res.Saved != 2 {
		test.Fatalf("saved=%d, want 2", res.Saved)
	}
}

func TestIndexRejectsRelativePath(test *testing.T) {
	_, err := Index(context.Background(), openDB(test), "docs")
	if err != ErrNotAbsolute {
		test.Fatalf("err=%v", err)
	}
}
