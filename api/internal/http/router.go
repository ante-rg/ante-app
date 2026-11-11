package http

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(api http.Handler, webDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, requestLogger)
	r.Handle("/api/*", api)
	r.Handle("/ingest/*", api)
	r.Handle("/healthz", api)
	r.NotFound(spa(webDir))
	return r
}

func spa(webDir string) http.HandlerFunc {
	files := http.FileServer(http.Dir(webDir))
	return func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(webDir, filepath.Clean("/"+r.URL.Path))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			if filepath.Base(p) != "index.html" {
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	}
}
