package main

import (
	"log"
	"net/http"
	"os"

	"local-file-indexer/internal/api"
	"local-file-indexer/internal/store"
)

func main() {
	dbPath := os.Getenv("INDEX_DB")
	if dbPath == "" {
		dbPath = "data/index.db"
	}

	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	api.New(db).Register(mux)

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
