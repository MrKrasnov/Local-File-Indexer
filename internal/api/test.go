package api

import (
	"net/http"

	"local-file-indexer/internal/api/respond"
)

func test(w http.ResponseWriter, r *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{
		"message": "test",
	})
}
