package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	apihttp "github.com/ante-rg/ante-app/api/internal/http"
	"github.com/ante-rg/ante-app/api/internal/testdb"
)

func newPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func scorer(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s.URL
}

func okScorer(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func get(t *testing.T, s *apihttp.Server, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	apihttp.NewAPIHandler(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not json: %q", rec.Body.String())
	}
	return rec, body
}

func server(t *testing.T, scorerURL string, timeout time.Duration) *apihttp.Server {
	return &apihttp.Server{
		DB:        newPool(t, testdb.Start(t)),
		ScorerURL: scorerURL,
		Client:    &http.Client{Timeout: timeout},
		Version:   "test-sha",
	}
}

func assertHealth(t *testing.T, rec *httptest.ResponseRecorder, body map[string]any, code int, status, db, sc string) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("code = %d, want %d", rec.Code, code)
	}
	want := map[string]any{"status": status, "database": db, "scorer": sc, "version": "test-sha"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %v, want %v", k, body[k], v)
		}
	}
}

func TestHealth_OK(t *testing.T) {
	rec, body := get(t, server(t, scorer(t, okScorer), time.Second), "/healthz")
	assertHealth(t, rec, body, 200, "ok", "ok", "ok")
}

func TestHealth_ScorerDown(t *testing.T) {
	s := httptest.NewServer(nil)
	url := s.URL
	s.Close()
	rec, body := get(t, server(t, url, time.Second), "/healthz")
	assertHealth(t, rec, body, 200, "degraded", "ok", "down")
}

func TestHealth_ScorerSlow(t *testing.T) {
	url := scorer(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	srv := server(t, url, 100*time.Millisecond)
	start := time.Now()
	rec, body := get(t, srv, "/healthz")
	if d := time.Since(start); d >= time.Second {
		t.Errorf("took %v", d)
	}
	assertHealth(t, rec, body, 200, "degraded", "ok", "down")
}

func TestHealth_ScorerNon200(t *testing.T) {
	url := scorer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	rec, body := get(t, server(t, url, time.Second), "/healthz")
	assertHealth(t, rec, body, 200, "degraded", "ok", "down")
}

func TestHealth_DatabaseDown(t *testing.T) {
	srv := &apihttp.Server{
		DB:        newPool(t, "postgres://x:x@127.0.0.1:1/x"),
		ScorerURL: scorer(t, okScorer),
		Client:    &http.Client{Timeout: time.Second},
		Version:   "test-sha",
	}
	rec, body := get(t, srv, "/healthz")
	if rec.Code != 503 || body["status"] != "down" || body["database"] != "down" {
		t.Fatalf("got %d %v", rec.Code, body)
	}
}

func TestUnimplemented_Returns501(t *testing.T) {
	srv := &apihttp.Server{DB: newPool(t, "postgres://x:x@127.0.0.1:1/x"), Client: http.DefaultClient}
	rec, body := get(t, srv, "/api/v1/operators")
	if rec.Code != 501 || body["code"] != "not_implemented" || body["message"] != "Not implemented yet" {
		t.Fatalf("got %d %v", rec.Code, body)
	}
}
