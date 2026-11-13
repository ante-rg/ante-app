package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	playerCount     = 100
	perOperator     = 20
	escalatingCount = 15
	minLastDay      = 50
	maxLastDay      = 700
	defaultDay      = 120
	sampleListSize  = 10

	escalateFrom  = 0.6
	activeProb    = 0.35
	activeProbEsc = 0.6
	nightProb     = 0.15
	nightProbEsc  = 0.4
	negProbEsc    = 0.8
	growthMax     = 2.0

	burstPlayers = 3
	burstFrom    = 20
	burstTo      = 80

	chaseSeconds  = 7200
	chaseDeposits = 3
	ruleThreshold = 2000.0
	ruleID        = "R_DEP_2000_30D"
	ruleText      = "Deposits over 2,000 € in 30 days"
	minScoreDay   = 7
)

type Player struct {
	ID, Operator                        string
	LastDay, SessionCount, DepositCount int32
	Escalating                          bool
}

type Session struct {
	PlayerID                           string
	No, StartDay, DurationS, DepositsN int32
	DepositsEUR, NetResultEUR          float64
	IsNight, IsChase                   bool
}

type Activity struct {
	PlayerID                                                  string
	Day, DepositsN, SessionsN, ChaseSessionsN, NightSessionsN int32
	DepositsEUR, PlayMinutes, NetResultEUR, EndBalanceEUR     float64
}

type Reason struct {
	Code   string  `json:"code"`
	Text   string  `json:"text"`
	Weight float64 `json:"weight"`
}

type Score struct {
	PlayerID string
	Day      int32
	Score    *float64
	Reasons  []Reason
}

type RuleFlag struct {
	PlayerID         string
	Day              int32
	RuleID, RuleText string
}

type SampleRow struct {
	PlayerID, Source string
	Position         int32
}

type Dataset struct {
	Players    []Player
	Sessions   []Session
	Activity   []Activity
	Scores     []Score
	RuleFlags  []RuleFlag
	Sample     []SampleRow
	DefaultDay int32
}

type dense struct {
	dep          []float64
	night, chase []int
	dep30        []float64
}

func Generate(seed int64) Dataset {
	rng := rand.New(rand.NewPCG(uint64(seed), uint64(seed)))
	rng2 := rand.New(rand.NewPCG(uint64(seed), uint64(seed)+1))
	ds := Dataset{Players: genPlayers(rng), DefaultDay: defaultDay}
	burst := pickBursts(rng2, ds.Players)
	series := make([]dense, len(ds.Players))
	maxDep30 := 0.0
	for i := range ds.Players {
		p := &ds.Players[i]
		sess, act := genHistory(rng, rng2, p, burst[i])
		ds.Sessions = append(ds.Sessions, sess...)
		ds.Activity = append(ds.Activity, act...)
		series[i] = buildDense(p, act)
		for _, v := range series[i].dep30 {
			maxDep30 = max(maxDep30, v)
		}
	}
	for i, p := range ds.Players {
		ds.Scores = append(ds.Scores, scoresFor(p, series[i], maxDep30)...)
		ds.RuleFlags = append(ds.RuleFlags, flagsFor(p, series[i])...)
	}
	ds.Sample = buildSample(rng, ds)
	return ds
}

func genPlayers(rng *rand.Rand) []Player {
	ps := make([]Player, playerCount)
	for i := range ps {
		ps[i] = Player{
			ID:       fmt.Sprintf("S%03d", i+1),
			Operator: fmt.Sprintf("P%d", i/perOperator+1),
			LastDay:  int32(minLastDay + rng.IntN(maxLastDay-minLastDay+1)),
		}
	}
	for _, i := range rng.Perm(playerCount)[:escalatingCount] {
		ps[i].Escalating = true
	}
	return ps
}

func pickBursts(rng2 *rand.Rand, ps []Player) []int32 {
	burst := make([]int32, len(ps))
	for i := range burst {
		burst[i] = -1
	}
	var ord []int
	for i, p := range ps {
		if !p.Escalating {
			ord = append(ord, i)
		}
	}
	for _, j := range rng2.Perm(len(ord))[:burstPlayers] {
		burst[ord[j]] = int32(burstFrom + rng2.IntN(min(burstTo, int(ps[ord[j]].LastDay))-burstFrom+1))
	}
	return burst
}

func burstSession(rng2 *rand.Rand, p *Player, day int32) Session {
	return Session{
		PlayerID:     p.ID,
		StartDay:     day,
		DurationS:    int32(5+rng2.IntN(56)) * 60,
		DepositsN:    1,
		DepositsEUR:  float64(ruleThreshold + 100 + float64(rng2.IntN(400))),
		NetResultEUR: round2(-30 + 55*rng2.Float64()),
	}
}

func genHistory(rng, rng2 *rand.Rand, p *Player, burstDay int32) ([]Session, []Activity) {
	var sess []Session
	var act []Activity
	start := int32(escalateFrom * float64(p.LastDay))
	balance := 0.0
	for d := int32(0); d <= p.LastDay; d++ {
		late := p.Escalating && d >= start
		prob := activeProb
		if late {
			prob = activeProbEsc
		}
		active := rng.Float64() < prob
		if !active && d != 0 && d != p.LastDay && d != burstDay {
			continue
		}
		r := rng
		if !active {
			r = rng2
		}
		ramp, growth := 0.0, 1.0
		if late {
			ramp = float64(d-start) / float64(max(1, p.LastDay-start))
			growth = 1 + (growthMax-1)*ramp
		}
		a := Activity{PlayerID: p.ID, Day: d}
		var day []Session
		for n := 1 + r.IntN(3); n > 0; n-- {
			day = append(day, genSession(r, p, d, late, ramp, growth))
		}
		if d == burstDay {
			day = []Session{burstSession(rng2, p, d)}
		}
		for _, s := range day {
			s.No = int32(len(sess) + 1)
			sess = append(sess, s)
			addToActivity(&a, s)
		}
		balance = max(0, balance+a.DepositsEUR+a.NetResultEUR)
		a.EndBalanceEUR = round2(balance)
		act = append(act, a)
	}
	p.SessionCount = int32(len(sess))
	for _, a := range act {
		p.DepositCount += a.DepositsN
	}
	return sess, act
}

func genSession(rng *rand.Rand, p *Player, day int32, late bool, ramp, growth float64) Session {
	s := Session{PlayerID: p.ID, StartDay: day}
	night := nightProb
	if late {
		night = nightProbEsc
		s.DurationS = int32(30+rng.IntN(211)) * 60
		s.DepositsN = int32(1 + rng.IntN(1+int(4*ramp*ramp)))
	} else {
		s.DurationS = int32(5+rng.IntN(86)) * 60
		s.DepositsN = int32(rng.IntN(3))
	}
	s.IsNight = rng.Float64() < night
	for i := int32(0); i < s.DepositsN; i++ {
		s.DepositsEUR += round2(float64(10+rng.IntN(51)) * growth)
	}
	s.DepositsEUR = round2(s.DepositsEUR)
	switch {
	case late && rng.Float64() < negProbEsc:
		s.NetResultEUR = -round2((20 + 80*rng.Float64()) * growth)
	case late:
		s.NetResultEUR = round2(10 + 50*rng.Float64())
	default:
		s.NetResultEUR = round2(-30 + 55*rng.Float64())
	}
	s.IsChase = s.DurationS >= chaseSeconds && s.DepositsN >= chaseDeposits && s.NetResultEUR < 0
	return s
}

func addToActivity(a *Activity, s Session) {
	a.SessionsN++
	a.DepositsN += s.DepositsN
	a.DepositsEUR = round2(a.DepositsEUR + s.DepositsEUR)
	a.NetResultEUR = round2(a.NetResultEUR + s.NetResultEUR)
	a.PlayMinutes += float64(s.DurationS) / 60
	if s.IsNight {
		a.NightSessionsN++
	}
	if s.IsChase {
		a.ChaseSessionsN++
	}
}

func buildDense(p *Player, act []Activity) dense {
	n := int(p.LastDay) + 1
	d := dense{dep: make([]float64, n), night: make([]int, n), chase: make([]int, n), dep30: make([]float64, n)}
	for _, a := range act {
		d.dep[a.Day] = a.DepositsEUR
		d.night[a.Day] = int(a.NightSessionsN)
		d.chase[a.Day] = int(a.ChaseSessionsN)
	}
	for day := range d.dep30 {
		d.dep30[day] = sumF(d.dep, day, 30)
	}
	return d
}

func sumF(x []float64, day, w int) float64 {
	var s float64
	for i := max(0, day-w+1); i <= day; i++ {
		s += x[i]
	}
	return s
}

func sumI(x []int, day, w int) int {
	var s int
	for i := max(0, day-w+1); i <= day; i++ {
		s += x[i]
	}
	return s
}

func scoresFor(p Player, d dense, maxDep30 float64) []Score {
	out := make([]Score, 0, len(d.dep))
	for day := range d.dep {
		s := Score{PlayerID: p.ID, Day: int32(day)}
		if day >= minScoreDay {
			v := math.Round(d.dep30[day]/maxDep30*1e4) / 1e4
			s.Score = &v
			s.Reasons = reasons(d.dep30[day], sumI(d.night, day, 7), sumI(d.chase, day, 30))
		}
		out = append(out, s)
	}
	return out
}

func reasons(dep30 float64, night, chase int) []Reason {
	rs := []Reason{
		{Code: "deposits_30d", Text: fmt.Sprintf("Deposited %s € in the last 30 days.", thousands(int(math.Round(dep30))))},
		{Code: "night_sessions_7d", Text: fmt.Sprintf("%d %s at night in the last 7 days.", night, plural(night, "session"))},
		{Code: "chase_sessions_30d", Text: fmt.Sprintf("%d loss-chasing %s in the last 30 days.", chase, plural(chase, "session"))},
	}
	v := [3]float64{min(dep30/ruleThreshold, 1), min(float64(night)/7, 1), min(float64(chase)/5, 1)}
	total := v[0] + v[1] + v[2]
	if total == 0 {
		v, total = [3]float64{1, 0, 0}, 1
	}
	big, rest := 0, 0.0
	for i := range v {
		rs[i].Weight = round2(v[i] / total)
		if v[i] > v[big] {
			big = i
		}
	}
	for i := range rs {
		if i != big {
			rest += rs[i].Weight
		}
	}
	rs[big].Weight = round2(1 - rest)
	slices.SortStableFunc(rs, func(a, b Reason) int {
		switch {
		case a.Weight > b.Weight:
			return -1
		case a.Weight < b.Weight:
			return 1
		}
		return 0
	})
	return rs
}

func flagsFor(p Player, d dense) []RuleFlag {
	var out []RuleFlag
	for day, v := range d.dep30 {
		if v > ruleThreshold {
			out = append(out, RuleFlag{PlayerID: p.ID, Day: int32(day), RuleID: ruleID, RuleText: ruleText})
		}
	}
	return out
}

func buildSample(rng *rand.Rand, ds Dataset) []SampleRow {
	eff := func(p Player) int32 { return min(defaultDay, p.LastDay) }
	scoreAt := map[string][]*float64{}
	for _, s := range ds.Scores {
		scoreAt[s.PlayerID] = append(scoreAt[s.PlayerID], s.Score)
	}
	lastDay := map[string]int32{}
	for _, p := range ds.Players {
		lastDay[p.ID] = p.LastDay
	}
	flagDays := map[string]int{}
	for _, f := range ds.RuleFlags {
		if f.Day <= min(defaultDay, lastDay[f.PlayerID]) {
			flagDays[f.PlayerID]++
		}
	}
	model := topN(ds.Players, func(p Player) float64 {
		if s := scoreAt[p.ID][eff(p)]; s != nil {
			return *s
		}
		return -1
	}, 0)
	rules := topN(ds.Players, func(p Player) float64 { return float64(flagDays[p.ID]) }, 1)
	return mergeSample(rng, model, rules)
}

func topN(ps []Player, key func(Player) float64, floor float64) []string {
	type row struct {
		id string
		k  float64
	}
	var rows []row
	for _, p := range ps {
		if k := key(p); k >= floor {
			rows = append(rows, row{p.ID, k})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].k > rows[j].k })
	ids := make([]string, 0, sampleListSize)
	for _, r := range rows[:min(sampleListSize, len(rows))] {
		ids = append(ids, r.id)
	}
	return ids
}

func mergeSample(rng *rand.Rand, model, rules []string) []SampleRow {
	var rows []SampleRow
	for _, id := range model {
		src := "model"
		if slices.Contains(rules, id) {
			src = "both"
		}
		rows = append(rows, SampleRow{PlayerID: id, Source: src})
	}
	for _, id := range rules {
		if !slices.Contains(model, id) {
			rows = append(rows, SampleRow{PlayerID: id, Source: "rules"})
		}
	}
	rng.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
	for i := range rows {
		rows[i].Position = int32(i + 1)
	}
	slices.SortFunc(rows, func(a, b SampleRow) int { return strings.Compare(a.PlayerID, b.PlayerID) })
	return rows
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

func plural(n int, noun string) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}

func thousands(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
