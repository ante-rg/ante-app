package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ante-rg/ante-app/api/internal/db"
	apihttp "github.com/ante-rg/ante-app/api/internal/http"
	"github.com/ante-rg/ante-app/api/internal/store"
	"github.com/ante-rg/ante-app/api/internal/testdb"
)

func TestAudit_WritesRow(t *testing.T) {
	ctx := context.Background()
	pool, err := db.Open(ctx, testdb.Start(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	stub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := apihttp.NewRouter(stub, os.TempDir(), apihttp.Auth{DevUser: "dev@localhost"}, store.New(pool))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/queue", nil))

	var (
		actor, method, path string
		status              int32
		hasAt               bool
	)
	row := pool.QueryRow(ctx, "SELECT actor, method, path, status, at IS NOT NULL FROM audit_log")
	if err := row.Scan(&actor, &method, &path, &status, &hasAt); err != nil {
		t.Fatal(err)
	}
	if actor != "dev@localhost" || method != "GET" || path != "/api/v1/queue" || status != 200 || !hasAt {
		t.Fatalf("got %s %s %s %d %v", actor, method, path, status, hasAt)
	}
}
