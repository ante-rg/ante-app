package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ante-rg/ante-app/api/internal/ingestkey"
)

const testKey = "ante_valid-test-key"

type fakeKeys struct {
	byHash map[string]string
	err    error
}

func (f fakeKeys) OperatorByKeyHash(_ context.Context, h pgtype.Text) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if c, ok := f.byHash[h.String]; ok {
		return c, nil
	}
	return "", pgx.ErrNoRows
}

var p3Keys = fakeKeys{byHash: map[string]string{ingestkey.Hash(testKey): "P3"}}

var ingestStub = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/big") {
		_, err := io.ReadAll(r.Body)
		var mbe *http.MaxBytesError
		if !errors.As(err, &mbe) {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}
	code, _ := operatorFrom(r.Context())
	_, _ = w.Write([]byte(code))
})

func ingest(t *testing.T, kl KeyLookup, fa *fakeAudit, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	routerWithKeys(t, ingestStub, Auth{DevUser: "dev@localhost"}, fa, kl).ServeHTTP(rec, req)
	return rec
}

const keyBody = `{"code":"unauthorized","message":"Missing or unknown API key"}`

func TestOperatorKey_Missing(t *testing.T) {
	fa := &fakeAudit{}
	rec := ingest(t, p3Keys, fa, httptest.NewRequest("POST", "/ingest/v1/events", nil))
	if rec.Code != 401 || strings.TrimSpace(rec.Body.String()) != keyBody {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	wantRow(t, fa, "anonymous", "POST", "/ingest/v1/events", 401)
}

func TestOperatorKey_Unknown(t *testing.T) {
	req := httptest.NewRequest("POST", "/ingest/v1/events", nil)
	req.Header.Set("X-API-Key", "ante_nope")
	if rec := ingest(t, p3Keys, &fakeAudit{}, req); rec.Code != 401 {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestOperatorKey_Valid(t *testing.T) {
	fa := &fakeAudit{}
	req := httptest.NewRequest("POST", "/ingest/v1/events", nil)
	req.Header.Set("X-API-Key", testKey)
	rec := ingest(t, p3Keys, fa, req)
	if rec.Code != 200 || rec.Body.String() != "P3" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	wantRow(t, fa, "P3", "POST", "/ingest/v1/events", 200)
}

func TestOperatorKey_HeaderCase(t *testing.T) {
	req := httptest.NewRequest("POST", "/ingest/v1/events", nil)
	req.Header.Set("x-api-key", testKey)
	if rec := ingest(t, p3Keys, &fakeAudit{}, req); rec.Code != 200 {
		t.Fatalf("lowercase header: got %d", rec.Code)
	}
}

func TestOperatorKey_LookupError(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	req := httptest.NewRequest("POST", "/ingest/v1/events", nil)
	req.Header.Set("X-API-Key", testKey)
	rec := ingest(t, fakeKeys{err: errors.New("db down")}, &fakeAudit{}, req)
	if rec.Code != 500 {
		t.Fatalf("got %d", rec.Code)
	}
	if out := buf.String(); strings.Contains(out, testKey) || strings.Contains(out, ingestkey.Hash(testKey)) {
		t.Fatalf("log leaks key: %s", out)
	}
}

func TestOperatorKey_APIUnaffected(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/replay", nil)
	req.Header.Set("X-API-Key", "ante_nope")
	req.Header.Set("Accept", "application/json")
	rec := ingest(t, p3Keys, &fakeAudit{}, req)
	if rec.Code != 200 {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestOperatorKey_BodyLimit(t *testing.T) {
	req := httptest.NewRequest("POST", "/ingest/v1/big", bytes.NewReader(make([]byte, 16<<20+1)))
	req.Header.Set("X-API-Key", testKey)
	if rec := ingest(t, p3Keys, &fakeAudit{}, req); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d", rec.Code)
	}
}
