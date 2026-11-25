package scan

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"local-file-indexer/internal/api/respond"
	"local-file-indexer/internal/indexer"
	"local-file-indexer/internal/store"
)

func Handler(db *store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(r.URL.Query().Get("path"))
		if path == "" {
			respond.JSON(w, http.StatusBadRequest, map[string]string{
				"error": "path query parameter is required",
			})
			return
		}

		result, err := indexer.Index(r.Context(), db, path)
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, indexer.ErrNotAbsolute):
				status = http.StatusBadRequest
			case errors.Is(err, os.ErrNotExist):
				status = http.StatusNotFound
			}
			respond.JSON(w, status, map[string]string{"error": err.Error()})
			return
		}

		log.Printf("scan path=%s saved=%d removed=%d errors=%d", result.Path, result.Saved, result.Removed, result.ErrorCount)
		respond.JSON(w, http.StatusOK, result)
	}
}
