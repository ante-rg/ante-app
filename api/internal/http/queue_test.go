package http_test

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	apihttp "github.com/ante-rg/ante-app/api/internal/http"
)

func queueFixture() *fixture {
	var ps []fxPlayer
	for i := 1; i <= 20; i++ {
		p := fxPlayer{ID: fmt.Sprintf("S%02d", i), Operator: "P1", LastDay: 5, Reasons: map[int32]string{}}
		if i > 10 {
			p.Operator = "P2"
		}
		for d := 0; d <= 5; d++ {
			var s *float64
			if i < 20 {
				s = sc(0.21 - 0.01*float64(i))
			}
			switch {
			case d == 4 && i == 1:
				s = sc(0.999)
			case d == 4 && i == 3:
				s = sc(0.205)
			}
			p.Scores = append(p.Scores, s)
		}
		ps = append(ps, p)
	}
	ps[0].Reasons[2] = `[{"code":"c","text":"DAY2","weight":0.5}]`
	ps[0].Reasons[4] = `[{"code":"x","text":"AFTER-CUTOFF","weight":0.9}]`
	ps[0].Reasons[5] = `[{"code":"a","text":"minor","weight":0.2},{"code":"b","text":"TOP","weight":0.7}]`
	ps[2].Flags = []int32{2, 5}
	ps[4].Flags = []int32{5}
	return &fixture{Players: ps}
}

func queue(t *testing.T, s *apihttp.Server, query string) map[string]any {
	t.Helper()
	rec, body := get(t, s, "/api/v1/queue?"+query)
	if rec.Code != 200 {
		t.Fatalf("code = %d %v", rec.Code, body)
	}
	return body
}

func itemsOf(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	raw, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("items not an array: %v", body["items"])
	}
	out := make([]map[string]any, len(raw))
	for i, r := range raw {
		out[i] = r.(map[string]any)
	}
	return out
}

func ids(items []map[string]any) string {
	var s []string
	for _, it := range items {
		s = append(s, it["player_id"].(string))
	}
	return strings.Join(s, ",")
}

func TestQueue(t *testing.T) {
	s := serve(t, queueFixture())
	t.Run("TestQueue_DefaultFilterAndCapacity", func(t *testing.T) {
		body := queue(t, s, "as_of_day=5")
		items := itemsOf(t, body)
		if got := ids(items); got != "S01,S02,S03,S04" {
			t.Fatalf("ids = %s", got)
		}
		for i, band := range []string{"high", "high", "elevated", "elevated"} {
			if items[i]["band"] != band || items[i]["rank"] != float64(i+1) {
				t.Fatalf("item %d = %v", i, items[i])
			}
			if items[i]["review_status"] != "pending" || items[i]["last_outcome"] != nil {
				t.Fatalf("item %d status = %v", i, items[i])
			}
		}
		if body["meta"].(map[string]any)["capacity"] != 10.0 {
			t.Fatalf("meta = %v", body["meta"])
		}
	})
	t.Run("TestQueue_CapacityCut", func(t *testing.T) {
		items := itemsOf(t, queue(t, s, "as_of_day=5&band=high,elevated,low&capacity=3"))
		if got := ids(items); got != "S01,S02,S03" {
			t.Fatalf("ids = %s", got)
		}
	})
	t.Run("TestQueue_CapacityLargerThanPlayers", func(t *testing.T) {
		body := queue(t, s, "as_of_day=5&band=high,elevated,low,insufficient_data&capacity=1000")
		items := itemsOf(t, body)
		if len(items) != 20 {
			t.Fatalf("len = %d", len(items))
		}
		last := items[19]
		if last["player_id"] != "S20" || last["rank"] != nil || last["band"] != "insufficient_data" {
			t.Fatalf("last = %v", last)
		}
		if body["meta"].(map[string]any)["capacity"] != 1000.0 {
			t.Fatalf("meta = %v", body["meta"])
		}
	})
	t.Run("TestQueue_OperatorFilterKeepsGlobalRank", func(t *testing.T) {
		items := itemsOf(t, queue(t, s, "as_of_day=5&operator=P2&band=high,elevated,low"))
		if len(items) != 9 || items[0]["player_id"] != "S11" || items[0]["rank"] != 11.0 {
			t.Fatalf("items = %v", ids(items))
		}
		for _, it := range items {
			if it["operator_code"] != "P2" {
				t.Fatalf("operator = %v", it)
			}
		}
	})
	t.Run("TestQueue_Counts", func(t *testing.T) {
		counts := queue(t, s, "as_of_day=5&operator=P2")["counts"].(map[string]any)
		want := map[string]any{
			"high": 2.0, "elevated": 2.0, "low": 15.0, "insufficient_data": 1.0,
			"pending_review": 4.0, "new_high_since_previous_day": 1.0,
		}
		for k, v := range want {
			if counts[k] != v {
				t.Errorf("counts[%s] = %v, want %v", k, counts[k], v)
			}
		}
	})
	t.Run("TestQueue_CountsNewHighZeroOnDayZero", func(t *testing.T) {
		counts := queue(t, s, "as_of_day=0")["counts"].(map[string]any)
		if counts["new_high_since_previous_day"] != 0.0 {
			t.Fatalf("counts = %v", counts)
		}
	})
	t.Run("TestQueue_TrendAndReason", func(t *testing.T) {
		items := itemsOf(t, queue(t, s, "as_of_day=5&band=high,elevated,low"))
		s01, s03 := items[0], items[2]
		trend := s01["score_trend_30d"].([]any)
		if len(trend) != 30 {
			t.Fatalf("len = %d", len(trend))
		}
		for i := 0; i < 24; i++ {
			if trend[i] != nil {
				t.Fatalf("trend[%d] = %v", i, trend[i])
			}
		}
		want := []float64{0.20, 0.20, 0.20, 0.20, 0.999, 0.20}
		for i, w := range want {
			if got, _ := trend[24+i].(float64); math.Abs(got-w) > 1e-6 {
				t.Fatalf("trend[%d] = %v, want %v", 24+i, trend[24+i], w)
			}
		}
		if s01["main_reason"] != "TOP" {
			t.Fatalf("main_reason = %v", s01["main_reason"])
		}
		if s01["flagged_by_rules"] != false || s03["flagged_by_rules"] != true {
			t.Fatalf("flags = %v %v", s01["flagged_by_rules"], s03["flagged_by_rules"])
		}
	})
	t.Run("TestQueue_CutOff", func(t *testing.T) {
		rec, body := get(t, s, "/api/v1/queue?as_of_day=2&band=high,elevated,low&capacity=20")
		if rec.Code != 200 {
			t.Fatalf("code = %d", rec.Code)
		}
		raw, _ := json.Marshal(body)
		for _, bad := range []string{"0.999", "AFTER-CUTOFF"} {
			if strings.Contains(string(raw), bad) {
				t.Fatalf("response leaks %s", bad)
			}
		}
		byID := map[string]map[string]any{}
		for _, it := range itemsOf(t, body) {
			byID[it["player_id"].(string)] = it
		}
		if byID["S01"]["main_reason"] != "DAY2" {
			t.Fatalf("main_reason = %v", byID["S01"]["main_reason"])
		}
		if byID["S03"]["flagged_by_rules"] != true || byID["S05"]["flagged_by_rules"] != false {
			t.Fatalf("flags S03=%v S05=%v", byID["S03"]["flagged_by_rules"], byID["S05"]["flagged_by_rules"])
		}
		trend := byID["S01"]["score_trend_30d"].([]any)
		if len(trend) != 30 || trend[29] == nil || trend[26] != nil {
			t.Fatalf("trend = %v", trend)
		}
	})
	t.Run("TestQueue_ReviewedEmpty", func(t *testing.T) {
		items := itemsOf(t, queue(t, s, "as_of_day=5&review_status=reviewed"))
		if len(items) != 0 {
			t.Fatalf("items = %v", items)
		}
	})
	t.Run("TestQueue_Errors", func(t *testing.T) {
		for query, want := range map[string]int{
			"as_of_day=-1": 400, "as_of_day=6": 400, "band=foo": 400, "band=high,foo": 400,
			"capacity=0": 400, "capacity=1001": 400, "mode=blind": 501,
			"mode=blnd": 400, "review_status=foo": 400,
		} {
			if rec, body := get(t, s, "/api/v1/queue?"+query); rec.Code != want {
				t.Errorf("%s: %d %v, want %d", query, rec.Code, body, want)
			}
		}
	})
}

func TestQueue_NoActiveModel(t *testing.T) {
	if rec, body := get(t, serve(t, nil), "/api/v1/queue"); rec.Code != 404 || body["code"] != "no_active_model" {
		t.Fatalf("no model: %d %v", rec.Code, body)
	}
}

func TestQueue_NoRanks(t *testing.T) {
	s := serve(t, &fixture{SkipRanks: true, Players: players()})
	if rec, body := get(t, s, "/api/v1/queue"); rec.Code != 404 || body["code"] != "no_ranks" {
		t.Fatalf("no ranks: %d %v", rec.Code, body)
	}
}

func TestQueue_CapacityFromCard(t *testing.T) {
	f := queueFixture()
	f.Card = `{"model_version":"test-v0","default_day":2,"default_capacity":3}`
	body := queue(t, serve(t, f), "as_of_day=5&band=high,elevated,low")
	if got := ids(itemsOf(t, body)); got != "S01,S02,S03" {
		t.Fatalf("ids = %s", got)
	}
	if body["meta"].(map[string]any)["capacity"] != 3.0 {
		t.Fatalf("meta = %v", body["meta"])
	}
}

func TestQueue_EffectiveDay(t *testing.T) {
	flat5 := func(v float64) []*float64 { return []*float64{sc(v), sc(v), sc(v), sc(v), sc(v), sc(v)} }
	f := &fixture{Players: []fxPlayer{
		{ID: "LONG", Operator: "P1", LastDay: 5, Scores: flat5(0.5)},
		{
			ID: "SHORT", Operator: "P1", LastDay: 3, Scores: []*float64{sc(0.1), sc(0.2), sc(0.3), sc(0.4)},
			Reasons: map[int32]string{
				1: `[{"code":"o","text":"OLD","weight":0.9}]`,
				3: `[{"code":"n","text":"SHORT3","weight":0.6}]`,
			},
			Flags: []int32{2},
		},
	}}
	var it map[string]any
	for _, c := range itemsOf(t, queue(t, serve(t, f), "as_of_day=5&band=high,elevated,low")) {
		if c["player_id"] == "SHORT" {
			it = c
		}
	}
	if it == nil {
		t.Fatal("SHORT missing")
	}
	if it["effective_day"] != 3.0 || it["history_ended_on_day"] != 3.0 || it["days_observed"] != 4.0 {
		t.Fatalf("days = %v", it)
	}
	if got, _ := it["score"].(float64); math.Abs(got-0.4) > 1e-6 {
		t.Fatalf("score = %v", it["score"])
	}
	trend := it["score_trend_30d"].([]any)
	want := []float64{0.1, 0.2, 0.3, 0.4}
	for i, v := range trend {
		if i < 26 {
			if v != nil {
				t.Fatalf("trend[%d] = %v", i, v)
			}
			continue
		}
		if got, _ := v.(float64); math.Abs(got-want[i-26]) > 1e-6 {
			t.Fatalf("trend[%d] = %v", i, v)
		}
	}
	if len(trend) != 30 || it["main_reason"] != "SHORT3" || it["flagged_by_rules"] != true {
		t.Fatalf("item = %v", it)
	}
}
