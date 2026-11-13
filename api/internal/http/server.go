// Package http holds the API handlers. Handlers implement the generated strict interface.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ante-rg/ante-app/api/internal/gen"
	"github.com/ante-rg/ante-app/api/internal/store"
)

var errNotImplemented = errors.New("not implemented")

type Pinger interface {
	Ping(ctx context.Context) error
}

type DB interface {
	Pinger
	store.DBTX
}

type Server struct {
	DB        DB
	ScorerURL string
	Client    *http.Client
	Version   string
}

var _ gen.StrictServerInterface = (*Server)(nil)

func NewAPIHandler(s *Server) http.Handler {
	h := gen.NewStrictHandlerWithOptions(s, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequest,
		ResponseErrorHandlerFunc: responseError,
	})
	mux := chi.NewRouter()
	mux.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, gen.Error{Code: "not_found", Message: "Not found"})
	})
	mux.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, gen.Error{Code: "method_not_allowed", Message: "Method not allowed"})
	})
	return gen.HandlerWithOptions(h, gen.ChiServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: badRequest,
	})
}

func badRequest(w http.ResponseWriter, _ *http.Request, err error) {
	writeError(w, http.StatusBadRequest, gen.Error{Code: "bad_request", Message: err.Error()})
}

func writeError(w http.ResponseWriter, status int, body gen.Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func responseError(w http.ResponseWriter, _ *http.Request, err error) {
	status, body := http.StatusInternalServerError, gen.Error{Code: "internal", Message: "Internal error"}
	if errors.Is(err, errNotImplemented) {
		status, body = http.StatusNotImplemented, gen.Error{Code: "not_implemented", Message: "Not implemented yet"}
	} else {
		slog.Error("handler failed", "err", err)
	}
	writeError(w, status, body)
}

func (s *Server) ListPlayerSessions(context.Context, gen.ListPlayerSessionsRequestObject) (gen.ListPlayerSessionsResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) ListPlayerReviews(context.Context, gen.ListPlayerReviewsRequestObject) (gen.ListPlayerReviewsResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) CreateReview(context.Context, gen.CreateReviewRequestObject) (gen.CreateReviewResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) ListOperators(context.Context, gen.ListOperatorsRequestObject) (gen.ListOperatorsResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) GetModelCard(context.Context, gen.GetModelCardRequestObject) (gen.GetModelCardResponseObject, error) {
	return nil, errNotImplemented
}

func (s *Server) IngestEvents(context.Context, gen.IngestEventsRequestObject) (gen.IngestEventsResponseObject, error) {
	return nil, errNotImplemented
}
