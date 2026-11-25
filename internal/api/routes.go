package api

import (
	"net/http"

	"local-file-indexer/internal/api/scan"
	"local-file-indexer/internal/store"
)

type Server struct {
	db *store.DB
}

func New(db *store.DB) *Server {
	return &Server{db: db}
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /test", test)
	mux.HandleFunc("POST /scan", scan.Handler(s.db))
}
