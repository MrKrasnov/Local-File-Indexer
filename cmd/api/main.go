package main

import (
	"log"
	"net/http"

	"local-file-indexer/internal/api"
)

func main() {
	mux := http.NewServeMux()
	api.Register(mux)

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
