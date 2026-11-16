package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireJSON(t *testing.T) {
	cases := []struct {
		name, method, ct string
		want             int
	}{
		{"text/plain", "POST", "text/plain", 415},
		{"form", "POST", "application/x-www-form-urlencoded", 415},
		{"no content type", "POST", "", 415},
		{"json", "POST", "application/json", 200},
		{"json charset", "POST", "application/json; charset=utf-8", 200},
		{"get without content type", "GET", "", 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			called := false
			h := requireJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				_, _ = w.Write([]byte("ok"))
			}))
			req := httptest.NewRequest(c.method, "/x", strings.NewReader("{}"))
			if c.ct != "" {
				req.Header.Set("Content-Type", c.ct)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d", rec.Code, c.want)
			}
			if c.want == 415 {
				if called {
					t.Fatal("next was called")
				}
				if !strings.Contains(rec.Body.String(), `"code":"unsupported_media_type"`) {
					t.Fatalf("body = %s", rec.Body.String())
				}
			}
		})
	}
}

func TestRouter_NonJSONWriteIs415AndAudited(t *testing.T) {
	fa := &fakeAudit{}
	req := httptest.NewRequest("POST", "/api/v1/players/S01/reviews", strings.NewReader("x"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	routerWithAuth(t, stubAPI, Auth{DevUser: "dev@localhost"}, fa).ServeHTTP(rec, req)
	if rec.Code != 415 {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
	wantRow(t, fa, "dev@localhost", "POST", "/api/v1/players/S01/reviews", 415)
}
