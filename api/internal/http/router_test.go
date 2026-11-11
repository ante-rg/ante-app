package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("api:" + r.URL.Path)) })
	return routerWith(t, api)
}

func routerWith(t *testing.T, api http.Handler) http.Handler {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewRouter(api, dir)
}

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestRouter_RootServesIndex(t *testing.T) {
	rec := do(testRouter(t), "GET", "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestRouter_SPAFallback(t *testing.T) {
	rec := do(testRouter(t), "GET", "/players/S001?day=3")
	if rec.Code != 200 || rec.Body.String() != "<html>app</html>" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestRouter_StaticAsset(t *testing.T) {
	rec := do(testRouter(t), "GET", "/assets/app.js")
	if rec.Code != 200 || rec.Body.String() != "console.log(1)" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestRouter_IndexIsNoCache(t *testing.T) {
	h := testRouter(t)
	for _, p := range []string{"/", "/players/S001"} {
		if cc := do(h, "GET", p).Header().Get("Cache-Control"); cc != "no-cache" {
			t.Fatalf("%s: Cache-Control = %q, want no-cache", p, cc)
		}
	}
	if cc := do(h, "GET", "/assets/app.js").Header().Get("Cache-Control"); cc != "" {
		t.Fatalf("asset Cache-Control = %q, want empty", cc)
	}
}

func TestRouter_UnknownAPIPathIs404JSON(t *testing.T) {
	h := routerWith(t, NewAPIHandler(&Server{}))
	for _, c := range [][2]string{{"GET", "/api/v1/nope"}, {"POST", "/ingest/v1/nope"}} {
		rec := do(h, c[0], c[1])
		if rec.Code != 404 || rec.Header().Get("Content-Type") != "application/json" || strings.Contains(rec.Body.String(), "<html>") {
			t.Fatalf("%v: got %d %q %q", c, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
		}
	}
}

func TestRouter_HealthzGoesToAPI(t *testing.T) {
	rec := do(testRouter(t), "GET", "/healthz")
	if rec.Body.String() != "api:/healthz" {
		t.Fatalf("got %q", rec.Body.String())
	}
}

func TestAPI_BadQueryParamIs400JSON(t *testing.T) {
	rec := do(NewAPIHandler(&Server{}), "GET", "/api/v1/queue?as_of_day=abc")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"code":"bad_request"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
