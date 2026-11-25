package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/ante-rg/ante-app/api/internal/store"
)

const auditTimeout = 2 * time.Second

type AuditWriter interface {
	InsertAuditLog(ctx context.Context, arg store.InsertAuditLogParams) error
}

func audit(aw AuditWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slot := "anonymous"
			ctx := context.WithValue(r.Context(), actorKey{}, &slot)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}
				rec := recover()
				if rec != nil {
					status = http.StatusInternalServerError
				}
				wctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), auditTimeout)
				defer cancel()
				err := aw.InsertAuditLog(wctx, store.InsertAuditLogParams{Actor: slot, Method: r.Method, Path: r.URL.Path, Status: int32(status)})
				if err != nil {
					slog.Error("audit write failed", "err", err, "request_id", middleware.GetReqID(r.Context()))
				}
				if rec != nil {
					panic(rec)
				}
			}()
			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}
