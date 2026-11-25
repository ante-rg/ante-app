package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ante-rg/ante-app/api/internal/gen"
	"github.com/ante-rg/ante-app/api/internal/ingestkey"
)

const maxIngestBody = 16 << 20

type KeyLookup interface {
	OperatorByKeyHash(ctx context.Context, hash pgtype.Text) (string, error)
}

type operatorCtxKey struct{}

func withOperator(ctx context.Context, code string) context.Context {
	return context.WithValue(ctx, operatorCtxKey{}, code)
}

func operatorFrom(ctx context.Context) (string, bool) {
	code, ok := ctx.Value(operatorCtxKey{}).(string)
	return code, ok
}

func requireOperatorKey(kl KeyLookup) func(http.Handler) http.Handler {
	if kl == nil {
		panic("http: requireOperatorKey needs a KeyLookup")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				unknownKey(w)
				return
			}
			code, err := kl.OperatorByKeyHash(r.Context(), pgtype.Text{String: ingestkey.Hash(key), Valid: true})
			if errors.Is(err, pgx.ErrNoRows) {
				unknownKey(w)
				return
			}
			if err != nil {
				slog.Error("key lookup failed", "err", err, "request_id", middleware.GetReqID(r.Context()))
				writeError(w, http.StatusInternalServerError, gen.Error{Code: "internal", Message: "Internal error"})
				return
			}
			setActor(r.Context(), code)
			r.Body = http.MaxBytesReader(w, r.Body, maxIngestBody)
			next.ServeHTTP(w, r.WithContext(withOperator(r.Context(), code)))
		})
	}
}

func unknownKey(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, gen.Error{Code: "unauthorized", Message: "Missing or unknown API key"})
}
