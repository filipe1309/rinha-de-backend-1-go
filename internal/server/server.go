package server

import (
	"net/http"
	"time"

	"github.com/filipe1309/rinha-de-backend-1-2023/internal/person"
)

// NewServer creates an HTTP server with all routes registered.
func NewServer(addr string, handler *person.Handler) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /pessoas", handler.Create)

	mux.HandleFunc("GET /pessoas/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		handler.GetByID(w, r, id)
	})

	mux.HandleFunc("GET /pessoas", handler.Search)

	mux.HandleFunc("GET /contagem-pessoas", handler.Count)

	return &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}
