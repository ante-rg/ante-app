package baseline

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

const (
	maxDay      = 36500
	nightEnd    = 5
	sessionGap  = 60 * time.Minute
	requiredCol = "event_order timestamp event_type bet_amount win_amount amount balance_after"
)

type Error struct {
	File   string
	Row    int
	Column string
	Rule   string
}

func (e *Error) Error() string {
	switch {
	case e.Row > 0:
		return fmt.Sprintf("%s: row %d: %s: %s", e.File, e.Row, e.Column, e.Rule)
	case e.Column != "":
		return fmt.Sprintf("%s: %s: %s", e.File, e.Column, e.Rule)
	default:
		return fmt.Sprintf("%s: %s", e.File, e.Rule)
	}
}

type balance struct {
	order int64
	eur   float64
}

type acct struct {
	balances map[int32]balance
}

func Build(in string) ([]fakeartifacts.Player, []fakeartifacts.Session, []fakeartifacts.Activity, int, error) {
	return build(in, 5)
}

func build(in string, brands int) ([]fakeartifacts.Player, []fakeartifacts.Session, []fakeartifacts.Activity, int, error) {
	r, err := loadRates()
	if err != nil {
		return nil, nil, nil, 0, err
	}
	accts, skipped, err := listAccounts(in, r, brands)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	var players []fakeartifacts.Player
	var sessions []fakeartifacts.Session
	byID := map[string]*acct{}
	for _, a := range accts {
		p, ss, ac, err := readAccount(a, r)
		if err != nil {
			return nil, nil, nil, 0, err
		}
		players, sessions, byID[a.ID] = append(players, p), append(sessions, ss...), ac
	}
	return players, sessions, activity(byID, sessions), skipped, nil
}

type event struct {
	line     int
	order    int64
	ts       time.Time
	eur      float64
	typ      string
	bet, win float64
	amt, bal float64
}

func readAccount(a account, r rates) (p fakeartifacts.Player, ss []fakeartifacts.Session, ac *acct, err error) {
	file := "account " + a.ID
	bad := func(line int, column, rule string) error {
		return &Error{File: file, Row: line, Column: column, Rule: rule}
	}
	fh, err := os.Open(a.Path)
	if err != nil {
		return p, nil, nil, bad(0, "", "cannot read file")
	}
	defer func() { _ = fh.Close() }()
	cr := csv.NewReader(fh)
	cr.FieldsPerRecord = -1
	head, err := cr.Read()
	if err != nil {
		return p, nil, nil, bad(0, "", "malformed csv")
	}
	col := map[string]int{}
	for i, h := range head {
		col[h] = i
	}
	for _, name := range strings.Fields(requiredCol) {
		if _, ok := col[name]; !ok {
			return p, nil, nil, bad(0, name, "missing column")
		}
	}
	var evs []event
	for line := 1; ; line++ {
		rec, rerr := cr.Read()
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return p, nil, nil, bad(line, "", "malformed csv")
		}
		e, err := parseEvent(rec, col, line, a.Currency, r, bad)
		if err != nil {
			return p, nil, nil, err
		}
		if n := len(evs); n > 0 && e.order <= evs[n-1].order {
			return p, nil, nil, bad(line, "event_order", "must increase")
		}
		evs = append(evs, e)
	}
	if len(evs) == 0 {
		return p, nil, nil, bad(0, "", "no events")
	}
	t0 := evs[0].ts
	for _, e := range evs {
		if e.ts.Before(t0) {
			t0 = e.ts
		}
	}
	ac = &acct{balances: map[int32]balance{}}
	p = fakeartifacts.Player{ID: a.ID, Operator: a.Operator}
	var cur *fakeartifacts.Session
	var minTs, maxTs time.Time
	var dep, net float64
	closeSession := func() {
		if cur == nil {
			return
		}
		cur.DurationS = int32(maxTs.Sub(minTs) / time.Second)
		cur.DepositsEUR, cur.NetResultEUR = cents(dep), cents(net)
		cur.IsChase = fakeartifacts.IsChase(cur.DurationS, cur.DepositsN, cur.NetResultEUR)
		ss = append(ss, *cur)
	}
	for i, e := range evs {
		d := int64(e.ts.Sub(t0) / (24 * time.Hour))
		if d > maxDay {
			return p, nil, nil, bad(e.line, "timestamp", "out of range")
		}
		day := int32(d)
		p.LastDay = max(p.LastDay, day)
		if b, ok := ac.balances[day]; !ok || e.order > b.order {
			ac.balances[day] = balance{e.order, cents(e.bal * e.eur)}
		}
		if i == 0 || e.ts.Sub(evs[i-1].ts) > sessionGap {
			closeSession()
			cur = &fakeartifacts.Session{PlayerID: a.ID, No: int32(len(ss) + 1), StartDay: day,
				IsNight: e.ts.UTC().Hour() <= nightEnd}
			minTs, maxTs, dep, net = e.ts, e.ts, 0, 0
		}
		if e.ts.Before(minTs) {
			minTs = e.ts
		}
		if e.ts.After(maxTs) {
			maxTs = e.ts
		}
		switch e.typ {
		case "ROUND":
			net += (e.win - e.bet) * e.eur
		case "GameAction:bet":
			net -= e.bet * e.eur
		case "GameAction:win", "FreeRound":
			net += e.win * e.eur
		case "DepositWithdraw":
			if e.amt > 0 {
				cur.DepositsN++
				p.DepositCount++
				dep += e.amt * e.eur
			}
		}
	}
	closeSession()
	p.SessionCount = int32(len(ss))
	return p, ss, ac, nil
}

func parseEvent(rec []string, col map[string]int, line int, cur string, r rates,
	bad func(int, string, string) error,
) (e event, err error) {
	get := func(name string) string {
		if i := col[name]; i < len(rec) {
			return rec[i]
		}
		return ""
	}
	num := func(name string, required, nonNeg bool) (float64, error) {
		s := get(name)
		if s == "" {
			if required {
				return 0, bad(line, name, "must not be empty")
			}
			return 0, nil
		}
		x, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, bad(line, name, "must be a finite number")
		}
		if nonNeg && x < 0 {
			return 0, bad(line, name, "must not be negative")
		}
		return x, nil
	}
	e = event{line: line, typ: get("event_type")}
	if e.order, err = strconv.ParseInt(get("event_order"), 10, 64); err != nil {
		return e, bad(line, "event_order", "must be an integer")
	}
	if e.ts, err = time.Parse(time.RFC3339, get("timestamp")); err != nil {
		return e, bad(line, "timestamp", "must be an RFC 3339 time")
	}
	if e.bet, err = num("bet_amount", false, true); err != nil {
		return e, err
	}
	if e.win, err = num("win_amount", false, true); err != nil {
		return e, err
	}
	if e.amt, err = num("amount", false, false); err != nil {
		return e, err
	}
	if e.bal, err = num("balance_after", true, false); err != nil {
		return e, err
	}
	month := e.ts.UTC().Format("2006-01")
	rate, ok := r.rate(cur, month)
	if !ok {
		return e, bad(line, "timestamp", "no EUR rate for "+cur+" "+month)
	}
	e.eur = 1 / rate
	return e, nil
}

func cents(x float64) float64 { return math.Round(x*100) / 100 }

func activity(accts map[string]*acct, ss []fakeartifacts.Session) []fakeartifacts.Activity {
	type dayKey struct {
		acct string
		day  int32
	}
	byDay := map[dayKey]*fakeartifacts.Activity{}
	for _, s := range ss {
		k := dayKey{s.PlayerID, s.StartDay}
		a := byDay[k]
		if a == nil {
			a = &fakeartifacts.Activity{PlayerID: s.PlayerID, Day: s.StartDay}
			byDay[k] = a
		}
		a.DepositsN += s.DepositsN
		a.DepositsEUR = cents(a.DepositsEUR + s.DepositsEUR)
		a.SessionsN++
		a.PlayMinutes += float64(s.DurationS) / 60
		a.NetResultEUR = cents(a.NetResultEUR + s.NetResultEUR)
		if s.IsChase {
			a.ChaseSessionsN++
		}
		if s.IsNight {
			a.NightSessionsN++
		}
	}
	out := make([]fakeartifacts.Activity, 0, len(byDay))
	for _, a := range byDay {
		out = append(out, *a)
	}
	slices.SortFunc(out, func(a, b fakeartifacts.Activity) int {
		if c := strings.Compare(a.PlayerID, b.PlayerID); c != 0 {
			return c
		}
		return int(a.Day) - int(b.Day)
	})
	var cur string
	var days []int32
	var eur float64
	next := 0
	for i := range out {
		if out[i].PlayerID != cur {
			cur = out[i].PlayerID
			days = days[:0]
			for d := range accts[cur].balances {
				days = append(days, d)
			}
			slices.Sort(days)
			eur, next = 0, 0
		}
		for next < len(days) && days[next] <= out[i].Day {
			eur = accts[cur].balances[days[next]].eur
			next++
		}
		out[i].EndBalanceEUR = eur
	}
	return out
}
