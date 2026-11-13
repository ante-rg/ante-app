package main

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

const testRuleID = "R_DEP_2000_30D"

func lastDays(ds Dataset) map[string]int32 {
	m := map[string]int32{}
	for _, p := range ds.Players {
		m[p.ID] = p.LastDay
	}
	return m
}

func depositsByDay(ds Dataset) map[string][]float64 {
	m := map[string][]float64{}
	for _, p := range ds.Players {
		m[p.ID] = make([]float64, p.LastDay+1)
	}
	for _, a := range ds.Activity {
		m[a.PlayerID][a.Day] = a.DepositsEUR
	}
	return m
}

func sumWindow(dep []float64, day int32, w int32) float64 {
	var s float64
	for d := max(0, day-w+1); d <= day; d++ {
		s += dep[d]
	}
	return s
}

func firstFlagDays(ds Dataset) map[string]int32 {
	m := map[string]int32{}
	for _, f := range ds.RuleFlags {
		if _, ok := m[f.PlayerID]; !ok {
			m[f.PlayerID] = f.Day
		}
	}
	return m
}

func TestGenerate_Deterministic(t *testing.T) {
	if !reflect.DeepEqual(Generate(1), Generate(1)) {
		t.Fatal("same seed gave different datasets")
	}
	if reflect.DeepEqual(Generate(2), Generate(1)) {
		t.Fatal("different seeds gave equal datasets")
	}
}

func TestGenerate_Invariants(t *testing.T) {
	ds := Generate(1)
	checkPlayers(t, ds)
	checkSessions(t, ds)
	checkActivity(t, ds)
	checkScores(t, ds)
	checkWeightsVary(t, ds)
	checkScoreValues(t, ds)
	checkActivityValues(t, ds)
	checkSampleContent(t, ds)
	checkRuleFlags(t, ds)
	checkSample(t, ds)
	if ds.DefaultDay != 120 {
		t.Errorf("DefaultDay = %d, want 120", ds.DefaultDay)
	}
}

func checkPlayers(t *testing.T, ds Dataset) {
	t.Helper()
	if len(ds.Players) != 100 {
		t.Fatalf("players = %d, want 100", len(ds.Players))
	}
	esc := 0
	for i, p := range ds.Players {
		if want := fmt.Sprintf("S%03d", i+1); p.ID != want {
			t.Errorf("player %d ID = %s, want %s", i, p.ID, want)
		}
		if want := fmt.Sprintf("P%d", i/20+1); p.Operator != want {
			t.Errorf("%s operator = %s, want %s", p.ID, p.Operator, want)
		}
		if p.LastDay < 50 || p.LastDay > 700 {
			t.Errorf("%s LastDay = %d out of [50,700]", p.ID, p.LastDay)
		}
		if p.Escalating {
			esc++
		}
	}
	if esc != 15 {
		t.Errorf("escalating = %d, want 15", esc)
	}
}

func checkSessions(t *testing.T, ds Dataset) {
	t.Helper()
	next := map[string]int32{}
	for _, s := range ds.Sessions {
		next[s.PlayerID]++
		if s.No != next[s.PlayerID] {
			t.Fatalf("%s session No = %d, want %d", s.PlayerID, s.No, next[s.PlayerID])
		}
		if want := s.DurationS >= 7200 && s.DepositsN >= 3 && s.NetResultEUR < 0; s.IsChase != want {
			t.Fatalf("%s #%d IsChase = %v, want %v", s.PlayerID, s.No, s.IsChase, want)
		}
	}
	for _, p := range ds.Players {
		if next[p.ID] != p.SessionCount {
			t.Errorf("%s sessions = %d, SessionCount = %d", p.ID, next[p.ID], p.SessionCount)
		}
	}
}

func checkActivity(t *testing.T, ds Dataset) {
	t.Helper()
	last := lastDays(ds)
	sess, dep := map[string]int32{}, map[string]int32{}
	for _, a := range ds.Activity {
		if a.Day < 0 || a.Day > last[a.PlayerID] {
			t.Fatalf("%s activity day %d out of range", a.PlayerID, a.Day)
		}
		sess[a.PlayerID] += a.SessionsN
		dep[a.PlayerID] += a.DepositsN
	}
	for _, p := range ds.Players {
		if sess[p.ID] != p.SessionCount || dep[p.ID] != p.DepositCount {
			t.Errorf("%s activity sums (%d,%d) != players (%d,%d)",
				p.ID, sess[p.ID], dep[p.ID], p.SessionCount, p.DepositCount)
		}
	}
}

func checkScores(t *testing.T, ds Dataset) {
	t.Helper()
	var want int
	for _, p := range ds.Players {
		want += int(p.LastDay) + 1
	}
	if len(ds.Scores) != want {
		t.Fatalf("scores = %d, want %d", len(ds.Scores), want)
	}
	dep := depositsByDay(ds)
	last := lastDays(ds)
	i := 0
	for _, p := range ds.Players {
		for d := int32(0); d <= last[p.ID]; d++ {
			checkScore(t, ds.Scores[i], p.ID, d, sumWindow(dep[p.ID], d, 30))
			i++
		}
	}
}

func checkScore(t *testing.T, s Score, id string, day int32, dep30 float64) {
	t.Helper()
	if s.PlayerID != id || s.Day != day {
		t.Fatalf("score order: got (%s,%d), want (%s,%d)", s.PlayerID, s.Day, id, day)
	}
	if day < 7 {
		if s.Score != nil || len(s.Reasons) != 0 {
			t.Fatalf("%s day %d: want nil score and no reasons", id, day)
		}
		return
	}
	if s.Score == nil || *s.Score < 0 || *s.Score > 1 {
		t.Fatalf("%s day %d: score %v not in [0,1]", id, day, s.Score)
	}
	if len(s.Reasons) != 3 {
		t.Fatalf("%s day %d: %d reasons", id, day, len(s.Reasons))
	}
	codes := []string{s.Reasons[0].Code, s.Reasons[1].Code, s.Reasons[2].Code}
	slices.Sort(codes)
	if !slices.Equal(codes, []string{"chase_sessions_30d", "deposits_30d", "night_sessions_7d"}) {
		t.Fatalf("%s day %d: codes %v", id, day, codes)
	}
	var sum float64
	for j, r := range s.Reasons {
		sum += r.Weight
		if j > 0 && r.Weight > s.Reasons[j-1].Weight {
			t.Fatalf("%s day %d: reasons not sorted by weight", id, day)
		}
		if r.Code == "deposits_30d" {
			if want := fmt.Sprintf("Deposited %s € in the last 30 days.", thousands(int(math.Round(dep30)))); r.Text != want {
				t.Fatalf("%s day %d: text %q, want %q", id, day, r.Text, want)
			}
		}
	}
	if math.Abs(sum-1) > 0.011 {
		t.Fatalf("%s day %d: weights sum %v", id, day, sum)
	}
}

func checkWeightsVary(t *testing.T, ds Dataset) {
	t.Helper()
	for _, s := range ds.Scores {
		if s.Score != nil && s.Reasons[0].Code != "deposits_30d" {
			return
		}
	}
	t.Error("deposits_30d is always the top reason, weights carry no information")
}

func checkRuleFlags(t *testing.T, ds Dataset) {
	t.Helper()
	dep := depositsByDay(ds)
	flagged := map[string]map[int32]bool{}
	for _, f := range ds.RuleFlags {
		if f.RuleID != testRuleID || f.RuleText != "Deposits over 2,000 € in 30 days" {
			t.Fatalf("bad rule flag %+v", f)
		}
		if sumWindow(dep[f.PlayerID], f.Day, 30) <= 2000 {
			t.Fatalf("flag %s day %d without deposits30 > 2000", f.PlayerID, f.Day)
		}
		if flagged[f.PlayerID] == nil {
			flagged[f.PlayerID] = map[int32]bool{}
		}
		flagged[f.PlayerID][f.Day] = true
	}
	for _, p := range ds.Players {
		for d := int32(0); d <= p.LastDay; d++ {
			if over := sumWindow(dep[p.ID], d, 30) > 2000; over != flagged[p.ID][d] {
				t.Fatalf("%s day %d: over=%v flagged=%v", p.ID, d, over, flagged[p.ID][d])
			}
		}
	}
}

func checkSample(t *testing.T, ds Dataset) {
	t.Helper()
	n := len(ds.Sample)
	if n < 10 || n > 20 {
		t.Fatalf("sample size = %d, want 10..20", n)
	}
	if !slices.IsSortedFunc(ds.Sample, func(a, b SampleRow) int { return strings.Compare(a.PlayerID, b.PlayerID) }) {
		t.Error("sample not sorted by PlayerID")
	}
	seenPos, seenID := map[int32]bool{}, map[string]bool{}
	for _, r := range ds.Sample {
		if r.Position < 1 || int(r.Position) > n || seenPos[r.Position] || seenID[r.PlayerID] {
			t.Fatalf("bad sample row %+v", r)
		}
		seenPos[r.Position], seenID[r.PlayerID] = true, true
		if !slices.Contains([]string{"model", "rules", "both"}, r.Source) {
			t.Fatalf("bad source %q", r.Source)
		}
	}
}

func TestGenerate_Calibration(t *testing.T) {
	ds := Generate(1)
	flagged := firstFlagDays(ds)
	chase := map[string]bool{}
	for _, s := range ds.Sessions {
		if s.IsChase {
			chase[s.PlayerID] = true
		}
	}
	var escFlag, ordFlag, escChase, ordChase int
	for _, p := range ds.Players {
		_, f := flagged[p.ID]
		switch {
		case p.Escalating:
			escFlag += b2i(f)
			escChase += b2i(chase[p.ID])
		default:
			ordFlag += b2i(f)
			ordChase += b2i(chase[p.ID])
		}
	}
	t.Logf("escalating flagged=%d chase=%d; ordinary flagged=%d chase=%d", escFlag, escChase, ordFlag, ordChase)
	if escFlag < 10 || ordFlag > 3 || escChase < 10 || ordChase != 0 {
		t.Fatalf("calibration off: escFlag=%d ordFlag=%d escChase=%d ordChase=%d", escFlag, ordFlag, escChase, ordChase)
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestGenerate_ModelEarlierThanRule(t *testing.T) {
	ds := Generate(1)
	last := lastDays(ds)
	scoreAt := map[string][]*float64{}
	for _, s := range ds.Scores {
		scoreAt[s.PlayerID] = append(scoreAt[s.PlayerID], s.Score)
	}
	flagged := firstFlagDays(ds)
	var hit, esc int
	for _, p := range ds.Players {
		first, ok := flagged[p.ID]
		if !p.Escalating || !ok {
			continue
		}
		esc++
		if inTop10(ds, scoreAt, last, first-7, p.ID) {
			hit++
		}
	}
	t.Logf("escalating flagged=%d, in top 10 at first_rule_day-7: %d", esc, hit)
	if esc < 10 || hit < 10 {
		t.Fatalf("model earlier for %d of %d escalating flagged players, want at least 10 of at least 10", hit, esc)
	}
}

func inTop10(ds Dataset, scoreAt map[string][]*float64, last map[string]int32, n int32, id string) bool {
	type row struct {
		id    string
		score float64
	}
	var rows []row
	for _, p := range ds.Players {
		if sc := scoreAt[p.ID][min(max(n, 0), last[p.ID])]; sc != nil {
			rows = append(rows, row{p.ID, *sc})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].score > rows[j].score })
	for _, r := range rows[:min(10, len(rows))] {
		if r.id == id {
			return true
		}
	}
	return false
}

type series struct {
	dep, night, chase []float64
}

func recompute(ds Dataset) (map[string]series, float64) {
	m := map[string]series{}
	for _, p := range ds.Players {
		n := p.LastDay + 1
		m[p.ID] = series{make([]float64, n), make([]float64, n), make([]float64, n)}
	}
	for _, a := range ds.Activity {
		m[a.PlayerID].dep[a.Day] = a.DepositsEUR
		m[a.PlayerID].night[a.Day] = float64(a.NightSessionsN)
		m[a.PlayerID].chase[a.Day] = float64(a.ChaseSessionsN)
	}
	var maxDep float64
	for _, p := range ds.Players {
		for d := int32(0); d <= p.LastDay; d++ {
			maxDep = math.Max(maxDep, sumWindow(m[p.ID].dep, d, 30))
		}
	}
	return m, maxDep
}

func expectedWeights(dep30, night, chase float64) [3]float64 {
	v := [3]float64{math.Min(dep30/2000, 1), math.Min(night/7, 1), math.Min(chase/5, 1)}
	total := v[0] + v[1] + v[2]
	if total == 0 {
		return [3]float64{1, 0, 0}
	}
	return [3]float64{v[0] / total, v[1] / total, v[2] / total}
}

func checkScoreValues(t *testing.T, ds Dataset) {
	t.Helper()
	ser, maxDep := recompute(ds)
	codes := [3]string{"deposits_30d", "night_sessions_7d", "chase_sessions_30d"}
	var zeroRows int
	for _, s := range ds.Scores {
		if s.Score == nil {
			continue
		}
		x := ser[s.PlayerID]
		dep30 := sumWindow(x.dep, s.Day, 30)
		night, chase := sumWindow(x.night, s.Day, 7), sumWindow(x.chase, s.Day, 30)
		if want := math.Round(dep30/maxDep*1e4) / 1e4; *s.Score != want {
			t.Fatalf("%s day %d: score %v, want %v", s.PlayerID, s.Day, *s.Score, want)
		}
		w := expectedWeights(dep30, night, chase)
		for _, r := range s.Reasons {
			i := slices.Index(codes[:], r.Code)
			if math.Abs(r.Weight-w[i]) > 0.011 {
				t.Fatalf("%s day %d: %s weight %v, want ~%v", s.PlayerID, s.Day, r.Code, r.Weight, w[i])
			}
			switch r.Code {
			case "night_sessions_7d":
				mustText(t, s, r, fmt.Sprintf("%d %s at night in the last 7 days.", int(night), noun(int(night), "session")))
			case "chase_sessions_30d":
				mustText(t, s, r, fmt.Sprintf("%d loss-chasing %s in the last 30 days.", int(chase), noun(int(chase), "session")))
			}
		}
		if math.Round(dep30) == 0 && night == 0 && chase == 0 {
			zeroRows++
			if s.Reasons[0].Code != "deposits_30d" || s.Reasons[0].Weight != 1 || s.Reasons[1].Weight != 0 || s.Reasons[2].Weight != 0 {
				t.Fatalf("%s day %d: all-zero reasons %+v, want 1,0,0", s.PlayerID, s.Day, s.Reasons)
			}
		}
	}
	t.Logf("all-zero rows seen: %d", zeroRows)
	if zeroRows == 0 {
		rs := reasons(0, 0, 0)
		if rs[0].Code != "deposits_30d" || rs[0].Weight != 1 || rs[1].Weight != 0 || rs[2].Weight != 0 {
			t.Fatalf("reasons(0,0,0) = %+v", rs)
		}
	}
}

func mustText(t *testing.T, s Score, r Reason, want string) {
	t.Helper()
	if r.Text != want {
		t.Fatalf("%s day %d: text %q, want %q", s.PlayerID, s.Day, r.Text, want)
	}
}

func checkActivityValues(t *testing.T, ds Dataset) {
	t.Helper()
	type key struct {
		id  string
		day int32
	}
	dep := map[key]float64{}
	for _, s := range ds.Sessions {
		dep[key{s.PlayerID, s.StartDay}] += s.DepositsEUR
	}
	for _, a := range ds.Activity {
		if math.Abs(a.DepositsEUR-dep[key{a.PlayerID, a.Day}]) > 1e-9 {
			t.Fatalf("%s day %d: activity deposits %v != sessions %v", a.PlayerID, a.Day, a.DepositsEUR, dep[key{a.PlayerID, a.Day}])
		}
		if a.EndBalanceEUR < 0 {
			t.Fatalf("%s day %d: negative balance %v", a.PlayerID, a.Day, a.EndBalanceEUR)
		}
	}
}

func checkSampleContent(t *testing.T, ds Dataset) {
	t.Helper()
	ser, maxDep := recompute(ds)
	type row struct {
		id string
		k  float64
	}
	var byScore, byFlags []row
	for _, p := range ds.Players {
		eff := min(120, p.LastDay)
		byScore = append(byScore, row{p.ID, math.Round(sumWindow(ser[p.ID].dep, eff, 30)/maxDep*1e4) / 1e4})
		var n float64
		for d := int32(0); d <= eff; d++ {
			if sumWindow(ser[p.ID].dep, d, 30) > 2000 {
				n++
			}
		}
		if n > 0 {
			byFlags = append(byFlags, row{p.ID, n})
		}
	}
	top := func(rows []row) []string {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].k > rows[j].k })
		var ids []string
		for _, r := range rows[:min(10, len(rows))] {
			ids = append(ids, r.id)
		}
		return ids
	}
	model, rules := top(byScore), top(byFlags)
	want := map[string]string{}
	for _, id := range model {
		want[id] = "model"
	}
	for _, id := range rules {
		if want[id] == "model" {
			want[id] = "both"
		} else {
			want[id] = "rules"
		}
	}
	if len(ds.Sample) != len(want) {
		t.Fatalf("sample has %d rows, want %d", len(ds.Sample), len(want))
	}
	for _, r := range ds.Sample {
		if want[r.PlayerID] != r.Source {
			t.Errorf("%s source %q, want %q", r.PlayerID, r.Source, want[r.PlayerID])
		}
	}
}

func noun(n int, w string) string {
	if n == 1 {
		return w
	}
	return w + "s"
}

func TestGenerate_DayZeroAndLastDayActive(t *testing.T) {
	ds := Generate(1)
	have := map[string]map[int32]bool{}
	for _, a := range ds.Activity {
		if have[a.PlayerID] == nil {
			have[a.PlayerID] = map[int32]bool{}
		}
		have[a.PlayerID][a.Day] = true
	}
	for _, p := range ds.Players {
		if !have[p.ID][0] || !have[p.ID][p.LastDay] {
			t.Errorf("%s: day 0 active=%v, last day %d active=%v", p.ID, have[p.ID][0], p.LastDay, have[p.ID][p.LastDay])
		}
	}
}

func TestGenerate_RuleFalsePositives(t *testing.T) {
	ds := Generate(1)
	var rules int
	for _, r := range ds.Sample {
		if r.Source == "rules" {
			rules++
		}
	}
	if rules < 1 {
		t.Errorf("review_sample has %d rules-only rows, want at least 1", rules)
	}
}

func TestGenerate_ReasonLeadersVary(t *testing.T) {
	var night, chase int
	for _, s := range Generate(1).Scores {
		if s.Score == nil {
			continue
		}
		switch s.Reasons[0].Code {
		case "night_sessions_7d":
			night++
		case "chase_sessions_30d":
			chase++
		}
	}
	t.Logf("top reason: night=%d chase=%d", night, chase)
	if night == 0 && chase == 0 {
		t.Error("neither night nor chase ever leads")
	}
}

func TestReasons_TextForms(t *testing.T) {
	rs := reasons(2345.4, 1, 1)
	want := map[string]string{
		"deposits_30d":       "Deposited 2,345 € in the last 30 days.",
		"night_sessions_7d":  "1 session at night in the last 7 days.",
		"chase_sessions_30d": "1 loss-chasing session in the last 30 days.",
	}
	for _, r := range rs {
		if r.Text != want[r.Code] {
			t.Errorf("%s text %q, want %q", r.Code, r.Text, want[r.Code])
		}
	}
	if !slices.ContainsFunc(reasons(640, 2, 0), func(r Reason) bool { return r.Text == "Deposited 640 € in the last 30 days." }) {
		t.Error("640 € text missing")
	}
}

func TestGenerate_MoneyHasTwoDecimals(t *testing.T) {
	ds := Generate(1)
	check := func(what string, v float64) {
		if v != round2(v) {
			t.Fatalf("%s = %v, not rounded to cents", what, v)
		}
	}
	for _, s := range ds.Sessions {
		check(s.PlayerID+" session deposits_eur", s.DepositsEUR)
		check(s.PlayerID+" session net_result_eur", s.NetResultEUR)
	}
	for _, a := range ds.Activity {
		check(a.PlayerID+" activity deposits_eur", a.DepositsEUR)
		check(a.PlayerID+" activity net_result_eur", a.NetResultEUR)
		check(a.PlayerID+" activity end_balance_eur", a.EndBalanceEUR)
	}
}
