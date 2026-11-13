package http_test

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	apihttp "github.com/ante-rg/ante-app/api/internal/http"
)

func playerFixture() *fixture {
	var ps []fxPlayer
	for i := 1; i <= 10; i++ {
		p := fxPlayer{ID: fmt.Sprintf("S%02d", i), Operator: "P1", LastDay: 6, Reasons: map[int32]string{}}
		for d := 0; d <= 6; d++ {
			p.Scores = append(p.Scores, sc(0.50-0.01*float64(i)))
		}
		ps = append(ps, p)
	}
	s1 := &ps[0]
	for d, v := range []float64{0.10, 0.10, 0.95, 0.95, 0.95, 0.95, 0.999} {
		s1.Scores[d] = sc(v)
	}
	for _, d := range []int32{1, 3, 5} {
		s1.Reasons[d] = fmt.Sprintf(`[{"code":"c","text":"R%d","weight":0.6}]`, d)
	}
	s1.Reasons[6] = `[{"code":"x","text":"AFTER-CUTOFF","weight":0.9}]`
	bal := 250.0
	s1.Activity = []fxActivity{
		{Day: 1, DepositsN: 1, DepositsEUR: 100, SessionsN: 1, PlayMinutes: 30.5, NetResultEUR: -50, EndBalanceEUR: &bal},
		{Day: 3, DepositsN: 2, DepositsEUR: 200, SessionsN: 1, PlayMinutes: 45, NetResultEUR: -120},
		{Day: 5, DepositsN: 3, DepositsEUR: 300, SessionsN: 2, PlayMinutes: 90.25, NetResultEUR: -300, ChaseSessionsN: 1, NightSessionsN: 1},
		{Day: 6, DepositsN: 9, DepositsEUR: 9999, SessionsN: 1, PlayMinutes: 1, NetResultEUR: -1},
	}
	s1.Sessions = []fxSession{
		{SessionNo: 1, StartDay: 1, DurationS: 100, DepositsN: 1, DepositsEUR: 100},
		{SessionNo: 2, StartDay: 3, DurationS: 100, DepositsN: 2, DepositsEUR: 200},
		{SessionNo: 3, StartDay: 5, DurationS: 100, DepositsN: 3, DepositsEUR: 300},
		{SessionNo: 4, StartDay: 6, DurationS: 100, DepositsN: 4, DepositsEUR: 400},
	}
	s1.Flags = []int32{4}
	s1.OtherFlags = []fxFlag{{Day: 6, RuleID: "R_OTHER", Text: "OTHER-RULE-TEXT"}}
	s99 := fxPlayer{ID: "S99", Operator: "P2", LastDay: 3, Reasons: map[int32]string{}}
	for d := 0; d <= 3; d++ {
		s99.Scores = append(s99.Scores, sc(0.30))
	}
	s99.Activity = []fxActivity{{Day: 3, DepositsN: 1, DepositsEUR: 10, SessionsN: 1}, {Day: 2, DepositsN: 1, DepositsEUR: 10, SessionsN: 1}}
	return &fixture{Players: append(ps, s99)}
}

func num(t *testing.T, v any) float64 {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("want number, got %T %v", v, v)
	}
	return f
}

func near(t *testing.T, name string, got any, want float64) {
	t.Helper()
	if math.Abs(num(t, got)-want) > 1e-4 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func eq(t *testing.T, name string, got, want any) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func okGet(t *testing.T, s *apihttp.Server, path string) map[string]any {
	t.Helper()
	rec, body := get(t, s, path)
	if rec.Code != 200 {
		t.Fatalf("%s: code = %d %v", path, rec.Code, body)
	}
	return body
}

func TestPlayer(t *testing.T) {
	s := serve(t, playerFixture())
	t.Run("TestPlayer_Case", func(t *testing.T) {
		b := okGet(t, s, "/api/v1/players/S01?as_of_day=5")
		eq(t, "effective_day", b["effective_day"], 5.0)
		eq(t, "days_observed", b["days_observed"], 6.0)
		eq(t, "history_ended_on_day", b["history_ended_on_day"], nil)
		eq(t, "sessions_to_date", b["sessions_to_date"], 3.0)
		eq(t, "deposits_to_date", b["deposits_to_date"], 6.0)
		eq(t, "deposits_eur_to_date", b["deposits_eur_to_date"], 600.0)
		near(t, "score", b["score"], 0.95)
		eq(t, "rank", b["rank"], 1.0)
		eq(t, "band", b["band"], "high")
		eq(t, "rank_of", b["rank_of"], 11.0)
		eq(t, "first_high_day", b["first_high_day"], 2.0)
		eq(t, "first_rule_flag_day", b["first_rule_flag_day"], 4.0)
		reasons, _ := b["reasons"].([]any)
		if len(reasons) != 1 || reasons[0].(map[string]any)["text"] != "R5" {
			t.Errorf("reasons = %v", b["reasons"])
		}
		flags, _ := b["rule_flags"].([]any)
		if len(flags) != 1 {
			t.Fatalf("rule_flags = %v", b["rule_flags"])
		}
		f := flags[0].(map[string]any)
		eq(t, "flag rule_id", f["rule_id"], "R_DEP_2000_30D")
		eq(t, "flag text", f["text"], "Deposits over 2,000 € in 30 days")
		eq(t, "flag first_day", f["first_day"], 4.0)
		if _, ok := b["meta"].(map[string]any); !ok {
			t.Errorf("meta = %v", b["meta"])
		}
	})

	t.Run("TestPlayer_CutOff", func(t *testing.T) {
		b := okGet(t, s, "/api/v1/players/S01?as_of_day=5")
		raw, _ := json.Marshal(b)
		for _, bad := range []string{"0.999", "AFTER-CUTOFF", "9999", "R_OTHER", "OTHER-RULE-TEXT"} {
			if strings.Contains(string(raw), bad) {
				t.Errorf("response leaks %q: %s", bad, raw)
			}
		}
		eq(t, "sessions_to_date", b["sessions_to_date"], 3.0)
	})

	t.Run("TestPlayer_HistoryEnded", func(t *testing.T) {
		b := okGet(t, s, "/api/v1/players/S99?as_of_day=6")
		eq(t, "effective_day", b["effective_day"], 3.0)
		eq(t, "history_ended_on_day", b["history_ended_on_day"], 3.0)
		near(t, "score", b["score"], 0.30)
	})

	t.Run("TestTimeline", func(t *testing.T) {
		b := okGet(t, s, "/api/v1/players/S01/timeline?as_of_day=5")
		days, _ := b["days"].([]any)
		if len(days) != 3 {
			t.Fatalf("days = %v", b["days"])
		}
		want := []struct {
			day, dep, eur float64
			play, score   float64
			band          string
		}{{1, 1, 100, 30.5, 0.10, "low"}, {3, 2, 200, 45, 0.95, "high"}, {5, 3, 300, 90.25, 0.95, "high"}}
		for i, w := range want {
			d := days[i].(map[string]any)
			eq(t, "day", d["day"], w.day)
			eq(t, "deposits_n", d["deposits_n"], w.dep)
			eq(t, "deposits_eur", d["deposits_eur"], w.eur)
			near(t, "play_minutes", d["play_minutes"], w.play)
			near(t, "score", d["score"], w.score)
			eq(t, "band", d["band"], w.band)
		}
		eq(t, "day1 end_balance_eur", days[0].(map[string]any)["end_balance_eur"], 250.0)
		eq(t, "day3 end_balance_eur", days[1].(map[string]any)["end_balance_eur"], nil)
		eq(t, "day5 chase", days[2].(map[string]any)["chase_sessions_n"], 1.0)
		eq(t, "first_high_day", b["first_high_day"], 2.0)
		eq(t, "first_rule_flag_day", b["first_rule_flag_day"], 4.0)
		if _, ok := b["meta"].(map[string]any); !ok {
			t.Errorf("meta = %v", b["meta"])
		}
	})

	t.Run("TestTimeline_CutOffAndHistoryEnded", func(t *testing.T) {
		raw, _ := json.Marshal(okGet(t, s, "/api/v1/players/S01/timeline?as_of_day=5"))
		if strings.Contains(string(raw), "9999") || strings.Contains(string(raw), `"day":6`) {
			t.Errorf("timeline leaks day 6: %s", raw)
		}
		b := okGet(t, s, "/api/v1/players/S99/timeline?as_of_day=6")
		days, _ := b["days"].([]any)
		if len(days) != 2 || days[0].(map[string]any)["day"] != 2.0 || days[1].(map[string]any)["day"] != 3.0 {
			t.Errorf("days = %v, want days 2 and 3 ascending", b["days"])
		}
	})
}

func TestPlayer_FirstHighIgnoresDaysAfterEffective(t *testing.T) {
	f := playerFixture()
	for i := range f.Players[:10] {
		f.Players[i].Scores[6] = sc(0.01)
	}
	b := okGet(t, serve(t, f), "/api/v1/players/S99?as_of_day=6")
	eq(t, "band on day 6", b["band"], "high")
	eq(t, "first_high_day", b["first_high_day"], nil)
}

func TestPlayer_AllNullScores(t *testing.T) {
	f := playerFixture()
	f.Players[1].Scores = make([]*float64, 7)
	b := okGet(t, serve(t, f), "/api/v1/players/S02?as_of_day=5")
	eq(t, "band", b["band"], "insufficient_data")
	eq(t, "score", b["score"], nil)
	eq(t, "rank", b["rank"], nil)
	eq(t, "first_high_day", b["first_high_day"], nil)
}

func TestPlayer_NoRanks(t *testing.T) {
	f := playerFixture()
	f.SkipRanks = true
	s := serve(t, f)
	for _, path := range []string{"/api/v1/players/S01", "/api/v1/players/S01/timeline"} {
		if rec, body := get(t, s, path); rec.Code != 404 || body["code"] != "no_ranks" {
			t.Errorf("%s = %d %v", path, rec.Code, body)
		}
	}
}

func TestPlayer_Errors(t *testing.T) {
	rec, body := get(t, serve(t, nil), "/api/v1/players/S01")
	eq(t, "no model code", rec.Code, 404)
	eq(t, "no model body", body["code"], "no_active_model")
	s := serve(t, playerFixture())
	for _, c := range []struct {
		path string
		code int
		body string
	}{
		{"/api/v1/players/S77", 404, "not_found"},
		{"/api/v1/players/S01?as_of_day=99", 400, "bad_request"},
		{"/api/v1/players/S01?mode=blind", 501, "not_implemented"},
		{"/api/v1/players/S01?mode=blnd", 400, "bad_request"},
		{"/api/v1/players/S77/timeline", 404, "not_found"},
		{"/api/v1/players/S01/timeline?as_of_day=-1", 400, "bad_request"},
	} {
		rec, body := get(t, s, c.path)
		if rec.Code != c.code || body["code"] != c.body {
			t.Errorf("%s = %d %v, want %d %s", c.path, rec.Code, body, c.code, c.body)
		}
	}
}
