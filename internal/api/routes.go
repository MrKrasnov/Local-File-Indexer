package api

import "net/http"

func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /test", test)
}
