package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/ante-rg/ante-app/api/internal/auth"
	"github.com/ante-rg/ante-app/api/internal/gen"
)

type Auth struct {
	DevUser  string
	Verifier auth.Verifier
}

type actorKey struct{}

func setActor(ctx context.Context, email string) {
	if slot, ok := ctx.Value(actorKey{}).(*string); ok {
		*slot = email
	}
}

func requireUser(a Auth) func(http.Handler) http.Handler {
	if a.DevUser == "" && a.Verifier == nil {
		panic("http: Auth needs DevUser or Verifier")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email := a.DevUser
			if email == "" {
				var reason string
				var err error
				if tok := r.Header.Get("Cf-Access-Jwt-Assertion"); tok == "" {
					reason = "missing"
				} else if email, err = a.Verifier.Email(r.Context(), tok); err != nil {
					reason = "invalid"
				}
				if reason != "" {
					slog.Info("auth rejected", "reason", reason, "err", err, "request_id", middleware.GetReqID(r.Context()))
					writeError(w, http.StatusUnauthorized, gen.Error{Code: "unauthorized", Message: "Sign in through Cloudflare Access"})
					return
				}
			}
			setActor(r.Context(), email)
			next.ServeHTTP(w, r.WithContext(auth.WithEmail(r.Context(), email)))
		})
	}
}
