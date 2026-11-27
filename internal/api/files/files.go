package files

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"local-file-indexer/internal/api/respond"
	"local-file-indexer/internal/store"
)

const (
	defaultLimit = 100
	maxLimit     = 1000
)

type response struct {
	Keyword string       `json:"keyword"`
	Count   int          `json:"count"`
	Files   []store.Meta `json:"files"`
}

func Handler(db *store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
		limit, err := parseLimit(r.URL.Query().Get("limit"))
		if err != nil {
			respond.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		found, err := db.Search(r.Context(), keyword, limit)
		if err != nil {
			respond.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		log.Printf("files keyword=%q count=%d", keyword, len(found))
		respond.JSON(w, http.StatusOK, response{
			Keyword: keyword,
			Count:   len(found),
			Files:   found,
		})
	}
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > maxLimit {
		return 0, errLimit
	}
	return limit, nil
}

type limitError string

func (e limitError) Error() string { return string(e) }

const errLimit limitError = "limit must be an integer from 1 to 1000"
