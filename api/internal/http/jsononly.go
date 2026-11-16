package http

import (
	"mime"
	"net/http"

	"github.com/ante-rg/ante-app/api/internal/gen"
)

func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, gen.Error{Code: "unsupported_media_type", Message: "Send the body as application/json"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
