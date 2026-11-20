package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	apihttp "github.com/ante-rg/ante-app/api/internal/http"
	"github.com/ante-rg/ante-app/api/internal/ingestkey"
)

type p3Lookup struct{ hash string }

func (l p3Lookup) OperatorByKeyHash(_ context.Context, h pgtype.Text) (string, error) {
	if h.String == l.hash {
		return "P3", nil
	}
	return "", context.Canceled
}

func TestIngest_DBErrorLoggedWithoutValues(t *testing.T) {
	buf := captureLog(t)
	const key = "ante_SECRET-KEY-77"
	h := apihttp.NewRouter(apihttp.NewAPIHandler(&apihttp.Server{DB: failDB{}}), t.TempDir(),
		apihttp.Auth{DevUser: "dev@localhost"}, noAudit{}, p3Lookup{hash: ingestkey.Hash(key)})
	body := `{"player_id":"X01","events":[{"event_order":1,"event_type":"DEPOSIT","seconds_from_first_event":0,"game_code":"SECRET-GAME-42"}]}`
	req := httptest.NewRequest(http.MethodPost, "/ingest/v1/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500: %s", rec.Code, rec.Body)
	}
	log := buf.String()
	if !strings.Contains(log, "handler failed") {
		t.Fatalf("error not logged: %s", log)
	}
	for _, secret := range []string{"SECRET-GAME-42", key, ingestkey.Hash(key)} {
		if strings.Contains(log, secret) {
			t.Fatalf("secret leaked into log: %s", log)
		}
	}
}

func TestIngest_InvalidBeforeDB(t *testing.T) {
	const key = "ante_K"
	h := apihttp.NewRouter(apihttp.NewAPIHandler(&apihttp.Server{DB: failDB{}}), t.TempDir(),
		apihttp.Auth{DevUser: "dev@localhost"}, noAudit{}, p3Lookup{hash: ingestkey.Hash(key)})
	for name, ev := range map[string]string{
		"no seconds": `{"event_order":1,"event_type":"DEPOSIT"}`,
		"product":    `{"event_order":1,"event_type":"DEPOSIT","seconds_from_first_event":0,"product":"POKER"}`,
		"nil text":   `{"event_order":1,"event_type":"DEPOSIT","seconds_from_first_event":0,"product":"<nil>"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/ingest/v1/events", strings.NewReader(`{"player_id":"X01","events":[`+ev+`]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: code = %d, want 422: %s", name, rec.Code, rec.Body)
		}
	}
}
