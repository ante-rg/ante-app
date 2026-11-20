package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ante-rg/ante-app/api/internal/auth"
	"github.com/ante-rg/ante-app/api/internal/store"
)

type fakeVerifier struct{ tokens map[string]string }

func (f fakeVerifier) Email(_ context.Context, tok string) (string, error) {
	if e, ok := f.tokens[tok]; ok {
		return e, nil
	}
	return "", errors.New("bad token")
}

type fakeAudit struct {
	rows   []store.InsertAuditLogParams
	err    error
	ctxErr []error
}

func (f *fakeAudit) InsertAuditLog(ctx context.Context, arg store.InsertAuditLogParams) error {
	f.rows = append(f.rows, arg)
	f.ctxErr = append(f.ctxErr, ctx.Err())
	return f.err
}

var stubAPI = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v1/panic":
		panic("boom")
	case "/api/v1/nocontent":
		w.WriteHeader(http.StatusNoContent)
		return
	case "/api/v1/implicit":
		_, _ = w.Write([]byte("x"))
		return
	}
	email, _ := auth.Email(r.Context())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(email))
})

var cfAuth = Auth{Verifier: fakeVerifier{tokens: map[string]string{"good": "rg@example.com"}}}

func serve(t *testing.T, a Auth, fa *fakeAudit, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	routerWithAuth(t, stubAPI, a, fa).ServeHTTP(rec, req)
	return rec
}

func withToken(method, path, tok string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set("Cf-Access-Jwt-Assertion", tok)
	return r
}

func wantRow(t *testing.T, fa *fakeAudit, actor, method, path string, status int32) {
	t.Helper()
	want := store.InsertAuditLogParams{Actor: actor, Method: method, Path: path, Status: status}
	if len(fa.rows) != 1 || fa.rows[0] != want {
		t.Fatalf("rows = %+v, want [%+v]", fa.rows, want)
	}
}

func want401(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	const body = `{"code":"unauthorized","message":"Sign in through Cloudflare Access"}`
	if rec.Code != 401 || strings.TrimSpace(rec.Body.String()) != body {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	fa := &fakeAudit{}
	want401(t, serve(t, cfAuth, fa, httptest.NewRequest("GET", "/api/v1/queue", nil)))
	wantRow(t, fa, "anonymous", "GET", "/api/v1/queue", 401)
}

func TestAuth_EmptyHeader(t *testing.T) {
	fa := &fakeAudit{}
	want401(t, serve(t, cfAuth, fa, withToken("GET", "/api/v1/queue", "")))
	wantRow(t, fa, "anonymous", "GET", "/api/v1/queue", 401)
}

func TestAuth_InvalidToken(t *testing.T) {
	fa := &fakeAudit{}
	want401(t, serve(t, cfAuth, fa, withToken("GET", "/api/v1/queue", "bad")))
	wantRow(t, fa, "anonymous", "GET", "/api/v1/queue", 401)
}

func TestAuth_EmailHeaderAlone(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/v1/queue", nil)
	r.Header.Set("Cf-Access-Authenticated-User-Email", "rg@example.com")
	want401(t, serve(t, cfAuth, &fakeAudit{}, r))
}

func TestAuth_ValidToken(t *testing.T) {
	fa := &fakeAudit{}
	rec := serve(t, cfAuth, fa, withToken("GET", "/api/v1/players/S001?as_of_day=3", "good"))
	if rec.Code != 200 || rec.Body.String() != "rg@example.com" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	wantRow(t, fa, "rg@example.com", "GET", "/api/v1/players/S001", 200)
}

func TestAuth_StatusFromHandler(t *testing.T) {
	fa := &fakeAudit{}
	serve(t, cfAuth, fa, withToken("GET", "/api/v1/nocontent", "good"))
	wantRow(t, fa, "rg@example.com", "GET", "/api/v1/nocontent", 204)
}

func TestAuth_ImplicitStatus(t *testing.T) {
	fa := &fakeAudit{}
	serve(t, cfAuth, fa, withToken("GET", "/api/v1/implicit", "good"))
	wantRow(t, fa, "rg@example.com", "GET", "/api/v1/implicit", 200)
}

func TestAuth_DevUser(t *testing.T) {
	fa := &fakeAudit{}
	rec := serve(t, Auth{DevUser: "dev@localhost"}, fa, httptest.NewRequest("GET", "/api/v1/queue", nil))
	if rec.Code != 200 || rec.Body.String() != "dev@localhost" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	wantRow(t, fa, "dev@localhost", "GET", "/api/v1/queue", 200)
}

func TestAuth_DevIgnoresHeader(t *testing.T) {
	rec := serve(t, Auth{DevUser: "dev@localhost"}, &fakeAudit{}, withToken("GET", "/api/v1/queue", "bad"))
	if rec.Code != 200 || rec.Body.String() != "dev@localhost" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestAudit_Ingest(t *testing.T) {
	fa := &fakeAudit{}
	rec := serve(t, cfAuth, fa, httptest.NewRequest("POST", "/ingest/v1/events", nil))
	if rec.Code != 401 {
		t.Fatalf("code = %d", rec.Code)
	}
	wantRow(t, fa, "anonymous", "POST", "/ingest/v1/events", 401)
}

func TestAudit_SkipsHealthAndSPA(t *testing.T) {
	fa := &fakeAudit{}
	for _, p := range []string{"/healthz", "/", "/assets/app.js"} {
		if rec := serve(t, cfAuth, fa, httptest.NewRequest("GET", p, nil)); rec.Code != 200 {
			t.Fatalf("%s: code = %d", p, rec.Code)
		}
	}
	if len(fa.rows) != 0 {
		t.Fatalf("rows = %+v, want none", fa.rows)
	}
}

func TestAudit_WriteErrorKeepsResponse(t *testing.T) {
	fa := &fakeAudit{err: errors.New("db down")}
	rec := serve(t, cfAuth, fa, withToken("GET", "/api/v1/queue", "good"))
	if rec.Code != 200 || rec.Body.String() != "rg@example.com" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestAudit_Panic(t *testing.T) {
	fa := &fakeAudit{}
	rec := serve(t, cfAuth, fa, withToken("GET", "/api/v1/panic", "good"))
	if rec.Code != 500 {
		t.Fatalf("code = %d", rec.Code)
	}
	wantRow(t, fa, "rg@example.com", "GET", "/api/v1/panic", 500)
}

func TestAudit_CancelledRequest(t *testing.T) {
	fa := &fakeAudit{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	serve(t, cfAuth, fa, withToken("GET", "/api/v1/queue", "good").WithContext(ctx))
	if len(fa.rows) != 1 || fa.ctxErr[0] != nil {
		t.Fatalf("rows = %+v, ctxErr = %v", fa.rows, fa.ctxErr)
	}
}

func TestAuth_NoBypass(t *testing.T) {
	h := routerWithAuth(t, NewAPIHandler(&Server{}), Auth{Verifier: fakeVerifier{}}, &fakeAudit{})
	for _, path := range []string{
		"/api/v1/queue",
		"/api/v1/queue/",
		"/ingest/../api/v1/queue",
		"/ingest/%2e%2e/api/v1/queue",
		"//api/v1/queue",
		"/API/v1/queue",
		"/healthz/../api/v1/queue",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code == 501 || (rec.Code == 200 && rec.Body.String() != "<html>app</html>") {
			t.Errorf("%s: got %d %q", path, rec.Code, rec.Body.String())
		}
	}
}

func TestNewRouter_PanicsWithoutAuthMode(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("want panic")
		}
	}()
	NewRouter(stubAPI, t.TempDir(), Auth{}, &fakeAudit{}, noKeys{})
}
