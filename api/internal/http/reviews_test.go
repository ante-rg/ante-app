package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ante-rg/ante-app/api/internal/auth"
	apihttp "github.com/ante-rg/ante-app/api/internal/http"
)

func reviewFixture() *fixture {
	f := queueFixture()
	sx := fxPlayer{ID: "SX", Operator: "P1", LastDay: 2, Reasons: map[int32]string{}}
	for d := 0; d <= 2; d++ {
		sx.Scores = append(sx.Scores, sc(0.1))
	}
	f.Players = append(f.Players, sx)
	return f
}

func send(t *testing.T, s *apihttp.Server, method, path, body, email string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if email != "" {
		req = req.WithContext(auth.WithEmail(req.Context(), email))
	}
	rec := httptest.NewRecorder()
	apihttp.NewAPIHandler(s).ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func reviewBody(day int, outcome, note, mode string) string {
	b, _ := json.Marshal(map[string]any{"as_of_day": day, "outcome": outcome, "note": note, "mode": mode})
	return string(b)
}

func post(t *testing.T, s *apihttp.Server, player, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return send(t, s, http.MethodPost, "/api/v1/players/"+player+"/reviews", body, "rg@example.com")
}

func TestCreateReview_Saves(t *testing.T) {
	s := serve(t, reviewFixture())
	rec, b := post(t, s, "S01", reviewBody(4, "no_concern", "looks fine", "normal"))
	if rec.Code != 201 {
		t.Fatalf("code = %d %v", rec.Code, b)
	}
	if b["player_id"] != "S01" || b["as_of_day"] != float64(4) || b["outcome"] != "no_concern" ||
		b["note"] != "looks fine" || b["reviewer"] != "rg@example.com" || b["blind"] != false ||
		b["model_version"] != "test-v0" {
		t.Fatalf("body = %v", b)
	}
	if id, _ := b["id"].(float64); id <= 0 {
		t.Fatalf("id = %v", b["id"])
	}
	if c, _ := b["created_at"].(string); c == "" {
		t.Fatalf("created_at = %v", b["created_at"])
	}
}

func TestCreateReview_ClampsToLastDay(t *testing.T) {
	s := serve(t, reviewFixture())
	rec, b := post(t, s, "SX", reviewBody(5, "no_concern", "", "normal"))
	if rec.Code != 201 || b["as_of_day"] != float64(2) {
		t.Fatalf("code = %d %v", rec.Code, b)
	}
}

func TestCreateReview_Blind(t *testing.T) {
	s := serve(t, reviewFixture())
	rec, b := post(t, s, "S01", reviewBody(4, "no_concern", "", "blind"))
	if rec.Code != 201 || b["blind"] != true {
		t.Fatalf("code = %d %v", rec.Code, b)
	}
}

func TestCreateReview_BlankNoteIsNull(t *testing.T) {
	s := serve(t, reviewFixture())
	rec, b := post(t, s, "S01", reviewBody(4, "no_concern", "   ", "normal"))
	if v, ok := b["note"]; rec.Code != 201 || (ok && v != nil) {
		t.Fatalf("code = %d %v", rec.Code, b)
	}
}

func TestCreateReview_Invalid(t *testing.T) {
	s := serve(t, reviewFixture())
	for name, body := range map[string]string{
		"outcome":   reviewBody(4, "bad", "", "normal"),
		"mode":      reviewBody(4, "no_concern", "", "x"),
		"day":       reviewBody(6, "no_concern", "", "normal"),
		"long note": reviewBody(4, "no_concern", strings.Repeat("é", 2001), "normal"),
		"no day":    `{"outcome":"no_concern","mode":"normal"}`,
		"null day":  `{"as_of_day":null,"outcome":"no_concern","mode":"normal"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec, b := post(t, s, "S01", body)
			if rec.Code != 422 || b["code"] != "invalid" {
				t.Fatalf("code = %d %v", rec.Code, b)
			}
		})
	}
}

func TestCreateReview_NoteAt2000(t *testing.T) {
	s := serve(t, reviewFixture())
	rec, b := post(t, s, "S01", reviewBody(4, "no_concern", strings.Repeat("é", 2000), "normal"))
	if rec.Code != 201 {
		t.Fatalf("code = %d %v", rec.Code, b)
	}
}

func TestCreateReview_UnknownPlayer(t *testing.T) {
	s := serve(t, reviewFixture())
	rec, b := post(t, s, "NOPE", reviewBody(4, "no_concern", "", "normal"))
	if rec.Code != 404 || b["code"] != "not_found" {
		t.Fatalf("code = %d %v", rec.Code, b)
	}
}

func TestCreateReview_NoActiveModel(t *testing.T) {
	s := serve(t, nil)
	rec, b := post(t, s, "S01", reviewBody(4, "no_concern", "", "normal"))
	if rec.Code != 404 || b["code"] != "no_active_model" {
		t.Fatalf("code = %d %v", rec.Code, b)
	}
}

func TestListReviews_DayCutAndOrder(t *testing.T) {
	s := serve(t, reviewFixture())
	for _, r := range []struct {
		day     int
		outcome string
	}{{2, "no_concern"}, {4, "intervention_needed"}, {3, "not_enough_data"}} {
		if rec, b := post(t, s, "S01", reviewBody(r.day, r.outcome, "", "normal")); rec.Code != 201 {
			t.Fatalf("post: %d %v", rec.Code, b)
		}
	}
	outcomes := func(day int) []any {
		rec, b := send(t, s, http.MethodGet, "/api/v1/players/S01/reviews?as_of_day="+strconv.Itoa(day), "", "rg@example.com")
		if rec.Code != 200 {
			t.Fatalf("code = %d %v", rec.Code, b)
		}
		items, ok := b["items"].([]any)
		if !ok {
			t.Fatalf("items = %v", b["items"])
		}
		var got []any
		for _, it := range items {
			got = append(got, it.(map[string]any)["outcome"])
		}
		return got
	}
	if got := outcomes(3); len(got) != 2 || got[0] != "not_enough_data" || got[1] != "no_concern" {
		t.Fatalf("day 3 = %v", got)
	}
	if got := outcomes(1); len(got) != 0 {
		t.Fatalf("day 1 = %v", got)
	}
}

func TestListReviews_Errors(t *testing.T) {
	s := serve(t, reviewFixture())
	rec, b := send(t, s, http.MethodGet, "/api/v1/players/NOPE/reviews?as_of_day=3", "", "rg@example.com")
	if rec.Code != 404 {
		t.Fatalf("unknown: %d %v", rec.Code, b)
	}
	rec, b = send(t, s, http.MethodGet, "/api/v1/players/S01/reviews?as_of_day=99", "", "rg@example.com")
	if rec.Code != 400 || b["code"] != "bad_request" {
		t.Fatalf("day: %d %v", rec.Code, b)
	}
}
