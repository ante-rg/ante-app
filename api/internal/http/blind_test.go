package http_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	apihttp "github.com/ante-rg/ante-app/api/internal/http"
)

const (
	reviewerA = "a@example.com"
	reviewerB = "b@example.com"
)

func blindFixture() *fixture {
	f := queueFixture()
	sy := fxPlayer{ID: "SY", Operator: "P1", LastDay: 2, Reasons: map[int32]string{}}
	for d := 0; d <= 2; d++ {
		sy.Scores = append(sy.Scores, sc(0.1))
	}
	f.Players = append(f.Players, sy)
	f.Sample = []fxSample{{"S07", "model", 0}, {"S03", "rules", 1}, {"S15", "both", 2}, {"SY", "model", 3}}
	return f
}

func blindGet(t *testing.T, s *apihttp.Server, path, email string) map[string]any {
	t.Helper()
	rec, body := send(t, s, http.MethodGet, path, "", email)
	if rec.Code != 200 {
		t.Fatalf("%s: code = %d %s", path, rec.Code, rec.Body)
	}
	return body
}

func statuses(items []map[string]any) string {
	var out []string
	for _, it := range items {
		o, _ := it["last_outcome"].(string)
		out = append(out, it["player_id"].(string)+":"+it["review_status"].(string)+":"+o)
	}
	return strings.Join(out, ",")
}

func seedBlindReviews(t *testing.T, s *apihttp.Server) {
	t.Helper()
	for _, c := range []struct{ email, player, mode string }{
		{reviewerA, "S07", "blind"}, {reviewerA, "S03", "normal"}, {reviewerB, "S15", "blind"},
	} {
		path := "/api/v1/players/" + c.player + "/reviews"
		if rec, b := send(t, s, http.MethodPost, path, reviewBody(4, "no_concern", "", c.mode), c.email); rec.Code != 201 {
			t.Fatalf("post %s: %d %v", c.player, rec.Code, b)
		}
	}
}

func TestBlindQueue_OrderAndFields(t *testing.T) {
	s := serve(t, blindFixture())
	rec, _ := send(t, s, http.MethodGet, "/api/v1/queue?as_of_day=5&mode=blind", "", reviewerA)
	var body struct {
		Mode   string                       `json:"mode"`
		Counts json.RawMessage              `json:"counts"`
		Items  []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	eq(t, "mode", body.Mode, "blind")
	eq(t, "counts", string(body.Counts), "{}")
	var got []string
	for _, it := range body.Items {
		got = append(got, strings.Trim(string(it["player_id"]), `"`))
		wantKeys(t, it, false, "rank", "score", "band", "score_trend_30d", "main_reason", "flagged_by_rules", "source", "position")
	}
	eq(t, "ids", strings.Join(got, ","), "S07,S03,S15,SY")
	sy := body.Items[3]
	eq(t, "effective_day", string(sy["effective_day"]), "2")
	eq(t, "history_ended_on_day", string(sy["history_ended_on_day"]), "2")
}

func TestBlindQueue_IgnoresBandAndCapacity(t *testing.T) {
	s := serve(t, blindFixture())
	items := itemsOf(t, blindGet(t, s, "/api/v1/queue?as_of_day=5&mode=blind&band=high&capacity=1", reviewerA))
	eq(t, "ids", ids(items), "S07,S03,S15,SY")
}

func TestBlindQueue_OperatorFilter(t *testing.T) {
	s := serve(t, blindFixture())
	eq(t, "P2", ids(itemsOf(t, blindGet(t, s, "/api/v1/queue?as_of_day=5&mode=blind&operator=P2", reviewerA))), "S15")
	body := blindGet(t, s, "/api/v1/queue?as_of_day=5&mode=blind&operator=P9", reviewerA)
	if items, ok := body["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("items = %v", body["items"])
	}
}

func TestBlindQueue_OwnBlindReviewsOnly(t *testing.T) {
	s := serve(t, blindFixture())
	seedBlindReviews(t, s)
	base := "/api/v1/queue?as_of_day=5&mode=blind"
	eq(t, "a", statuses(itemsOf(t, blindGet(t, s, base, reviewerA))),
		"S07:reviewed:no_concern,S03:pending:,S15:pending:,SY:pending:")
	eq(t, "reviewed", ids(itemsOf(t, blindGet(t, s, base+"&review_status=reviewed", reviewerA))), "S07")
	eq(t, "pending", ids(itemsOf(t, blindGet(t, s, base+"&review_status=pending", reviewerA))), "S03,S15,SY")
	d3 := itemsOf(t, blindGet(t, s, "/api/v1/queue?as_of_day=3&mode=blind", reviewerA))
	eq(t, "day3", d3[0]["review_status"], "pending")
	normal := itemsOf(t, blindGet(t, s, "/api/v1/queue?as_of_day=5&band=high,elevated,low&capacity=25", reviewerA))
	seen := 0
	for _, it := range normal {
		switch it["player_id"] {
		case "S07", "S03", "S15":
			seen++
			eq(t, it["player_id"].(string), it["review_status"].(string), "reviewed")
		}
	}
	if seen != 3 {
		t.Fatalf("normal queue showed %d of S07, S03, S15, want 3", seen)
	}
}

func TestBlindReviews_OwnOnly(t *testing.T) {
	s := serve(t, blindFixture())
	seedBlindReviews(t, s)
	n := func(path, email string) int {
		return len(blindGet(t, s, path, email)["items"].([]any))
	}
	const day = "?as_of_day=5"
	for _, c := range []struct {
		name, path, email string
		want              int
	}{
		{"a blind S07", "S07/reviews" + day + "&mode=blind", reviewerA, 1},
		{"b blind S07", "S07/reviews" + day + "&mode=blind", reviewerB, 0},
		{"a blind S03", "S03/reviews" + day + "&mode=blind", reviewerA, 0},
		{"a normal S03", "S03/reviews" + day, reviewerA, 1},
	} {
		if got := n("/api/v1/players/"+c.path, c.email); got != c.want {
			t.Errorf("%s: %d items, want %d", c.name, got, c.want)
		}
	}
	if got := n("/api/v1/players/S07/reviews?as_of_day=3&mode=blind", reviewerA); got != 0 {
		t.Errorf("as_of_day=3: %d items, want 0", got)
	}
	if got := n("/api/v1/players/S07/reviews?as_of_day=4&mode=blind", reviewerA); got != 1 {
		t.Errorf("as_of_day=4: %d items, want 1", got)
	}
	if rec, _ := send(t, s, http.MethodGet, "/api/v1/players/S07/reviews"+day+"&mode=x", "", reviewerA); rec.Code != 400 {
		t.Errorf("bad mode code = %d", rec.Code)
	}
}

func TestBlindQueue_NoSample(t *testing.T) {
	s := serve(t, queueFixture())
	body := blindGet(t, s, "/api/v1/queue?as_of_day=5&mode=blind", reviewerA)
	if items, ok := body["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("items = %v", body["items"])
	}
}
