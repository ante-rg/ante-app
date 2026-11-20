package http_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ante-rg/ante-app/api/internal/auth"
	apihttp "github.com/ante-rg/ante-app/api/internal/http"
	"github.com/ante-rg/ante-app/api/internal/store"
)

type noAudit struct{}

func (noAudit) InsertAuditLog(context.Context, store.InsertAuditLogParams) error { return nil }

type failDB struct{}

func (failDB) Ping(context.Context) error { return nil }
func (failDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("db down")
}
func (failDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("db down")
}
func (failDB) Begin(context.Context) (pgx.Tx, error)            { return nil, errors.New("db down") }
func (failDB) QueryRow(context.Context, string, ...any) pgx.Row { return failRow{} }

type failRow struct{}

func (failRow) Scan(...any) error { return errors.New("db down") }

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestCreateReview_DBErrorLoggedWithoutNote(t *testing.T) {
	buf := captureLog(t)
	body := `{"as_of_day":1,"outcome":"no_concern","note":"SECRET-NOTE-123","mode":"normal"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/players/S01/reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithEmail(req.Context(), "rg@example.com"))
	rec := httptest.NewRecorder()
	apihttp.NewAPIHandler(&apihttp.Server{DB: failDB{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(buf.String(), "handler failed") {
		t.Fatalf("error not logged: %s", buf.String())
	}
	if strings.Contains(buf.String(), "SECRET-NOTE-123") {
		t.Fatalf("note leaked into log: %s", buf.String())
	}
}

func TestCreateReview_MissingDayNeedsNoDB(t *testing.T) {
	for _, body := range []string{
		`{"outcome":"no_concern","mode":"normal"}`,
		`{"as_of_day":null,"outcome":"no_concern","mode":"normal"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/players/S01/reviews", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		apihttp.NewAPIHandler(&apihttp.Server{}).ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "as_of_day is required") {
			t.Fatalf("%s: code = %d: %s", body, rec.Code, rec.Body)
		}
	}
}

func TestCreateReview_NoteNotLogged(t *testing.T) {
	buf := captureLog(t)

	h := apihttp.NewRouter(apihttp.NewAPIHandler(&apihttp.Server{}), t.TempDir(), apihttp.Auth{DevUser: "dev@localhost"}, noAudit{}, p3Lookup{})
	body := `{"as_of_day":1,"outcome":"bad","note":"SECRET-NOTE-123","mode":"normal"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/players/S01/reviews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422: %s", rec.Code, rec.Body)
	}

	if strings.Contains(buf.String(), "SECRET-NOTE-123") {
		t.Fatalf("note leaked into log: %s", buf.String())
	}
}
