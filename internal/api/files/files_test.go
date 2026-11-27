package files

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"local-file-indexer/internal/indexer"
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

func indexSample(test *testing.T, db *store.DB) string {
	test.Helper()
	root := test.TempDir()
	notes := filepath.Join(root, "notes")
	if err := os.MkdirAll(notes, 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notes, "Report.txt"), []byte("hello"), 0o644); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "100%_done.md"), []byte("x"), 0o644); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "1000.txt"), []byte("y"), 0o644); err != nil {
		test.Fatal(err)
	}
	if _, err := indexer.Index(context.Background(), db, root); err != nil {
		test.Fatal(err)
	}
	return root
}

func getFiles(test *testing.T, mux *http.ServeMux, rawQuery string) response {
	test.Helper()
	rec := httptest.NewRecorder()
	target := "/files"
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		test.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got response
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		test.Fatal(err)
	}
	return got
}

func TestFiles(test *testing.T) {
	db := openDB(test)
	root := indexSample(test, db)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /files", Handler(db))

	test.Run("lists indexed files", func(test *testing.T) {
		got := getFiles(test, mux, "")
		if got.Count != 5 || len(got.Files) != 5 {
			test.Fatalf("count=%d files=%d", got.Count, len(got.Files))
		}
		if got.Files[0].Path != root {
			test.Fatalf("first path=%s", got.Files[0].Path)
		}
	})

	test.Run("filters by keyword in name", func(test *testing.T) {
		got := getFiles(test, mux, "keyword=report")
		if got.Count != 1 || got.Files[0].Name != "Report.txt" || got.Files[0].Extension != ".txt" {
			test.Fatalf("result=%+v", got)
		}
	})

	test.Run("filters by keyword in path", func(test *testing.T) {
		got := getFiles(test, mux, "keyword=notes")
		if got.Count != 2 {
			test.Fatalf("count=%d", got.Count)
		}
	})

	test.Run("treats like wildcards as plain text", func(test *testing.T) {
		got := getFiles(test, mux, url.Values{"keyword": {"100%"}}.Encode())
		if got.Count != 1 || got.Files[0].Name != "100%_done.md" {
			test.Fatalf("result=%+v", got)
		}
	})

	test.Run("returns an empty list when nothing matches", func(test *testing.T) {
		got := getFiles(test, mux, "keyword=missing")
		if got.Count != 0 || len(got.Files) != 0 {
			test.Fatalf("result=%+v", got)
		}
	})

	test.Run("rejects a bad limit", func(test *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/files?limit=0", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			test.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
