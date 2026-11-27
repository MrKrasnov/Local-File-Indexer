package scan

import (
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

func TestScan(test *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /scan", Handler(openDB(test)))

	test.Run("missing path", func(test *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/scan", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			test.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	test.Run("relative path", func(test *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/scan?path=docs", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			test.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	test.Run("missing directory", func(test *testing.T) {
		rec := httptest.NewRecorder()
		q := url.Values{}
		q.Set("path", filepath.Join(test.TempDir(), "missing"))
		req := httptest.NewRequest(http.MethodPost, "/scan?"+q.Encode(), nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			test.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	test.Run("indexes directory", func(test *testing.T) {
		root := test.TempDir()
		if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
			test.Fatal(err)
		}
		rec := httptest.NewRecorder()
		q := url.Values{}
		q.Set("path", root)
		req := httptest.NewRequest(http.MethodPost, "/scan?"+q.Encode(), nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			test.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var got indexer.Result
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			test.Fatal(err)
		}
		if got.Saved != 2 || got.Path != root {
			test.Fatalf("result=%+v", got)
		}
	})
}
