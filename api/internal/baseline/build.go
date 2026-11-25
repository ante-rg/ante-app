package baseline

import (
	"math"
	"slices"
	"strings"

	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

const (
	fileEvents = "events.parquet"
	fileRounds = "rounds.parquet"
	maxDay     = 36500
	nightEnd   = 5
	depositTyp = "DEPOSIT"
)

var colsEvents = []col{
	{"share_account_id", kStr, false}, {"project_code", kStr, false},
	{"event_order", kInt, false}, {"session_no", kInt, false},
	{"days_from_first_event", kInt, false}, {"seconds_from_session_start", kInt, false},
	{"local_hour", kInt, false}, {"event_type", kStr, false},
	{"payment_amount_eur", kFloat, true}, {"balance_after_eur", kFloat, true},
}

var colsRounds = []col{
	{"share_account_id", kStr, false}, {"bet_session_no", kInt, false},
	{"stake_eur", kFloat, true}, {"returned_eur", kFloat, true},
}

type sessKey struct {
	acct string
	no   int32
}

type sess struct {
	minOrder       int64
	startDay       int32
	startHour      int32
	maxSecs        int32
	depN           int32
	depEUR, netEUR float64
}

type balance struct {
	order int64
	eur   float64
}

type acct struct {
	project  string
	lastDay  int32
	deposits int32
	sessions int32
	balances map[int32]balance
}

func Build(dir string) ([]fakeartifacts.Player, []fakeartifacts.Session, []fakeartifacts.Activity, error) {
	accts := map[string]*acct{}
	sessions := map[sessKey]*sess{}
	if err := scan(dir, fileEvents, colsEvents, func(d *dec) error {
		return readEvent(d, accts, sessions)
	}); err != nil {
		return nil, nil, nil, err
	}
	if err := scan(dir, fileRounds, colsRounds, func(d *dec) error {
		return readRound(d, sessions)
	}); err != nil {
		return nil, nil, nil, err
	}
	players, ss := assemble(accts, sessions)
	return players, ss, activity(accts, ss), nil
}

func readEvent(d *dec, accts map[string]*acct, sessions map[sessKey]*sess) error {
	id, project := d.str(0), d.str(1)
	order, no, day, secs, hour := d.int(2), d.int(3), d.int(4), d.int(5), d.int(6)
	typ := d.str(7)
	pay, hasPay := d.float(8)
	bal, hasBal := d.float(9)
	if d.err != nil {
		return nil
	}
	switch {
	case id == "":
		d.fail(0, "must not be empty")
	case project == "":
		d.fail(1, "must not be empty")
	case day < 0 || day > maxDay:
		d.fail(4, "out of range")
	case no < 0 || no > math.MaxInt32:
		d.fail(3, "out of range")
	case secs < 0 || secs > math.MaxInt32:
		d.fail(5, "out of range")
	case hour < 0 || hour > 23:
		d.fail(6, "out of range")
	case typ == depositTyp && !hasPay:
		d.fail(8, "must not be null on a deposit")
	case typ == depositTyp && pay < 0:
		d.fail(8, "must not be negative")
	}
	if d.err != nil {
		return nil
	}
	a := accts[id]
	if a == nil {
		a = &acct{project: project, balances: map[int32]balance{}}
		accts[id] = a
	} else if a.project != project {
		d.fail(1, "account on more than one project")
		return nil
	}
	a.lastDay = max(a.lastDay, int32(day))
	if hasBal && typ != "GAME_STATE" {
		if b, ok := a.balances[int32(day)]; !ok || order > b.order {
			a.balances[int32(day)] = balance{order, bal}
		}
	}
	k := sessKey{id, int32(no)}
	s := sessions[k]
	if s == nil {
		s = &sess{minOrder: math.MaxInt64}
		sessions[k] = s
		a.sessions++
	}
	if order < s.minOrder {
		s.minOrder, s.startDay, s.startHour = order, int32(day), int32(hour)
	}
	s.maxSecs = max(s.maxSecs, int32(secs))
	if typ == depositTyp {
		s.depN++
		s.depEUR += pay
		a.deposits++
	}
	return nil
}

func readRound(d *dec, sessions map[sessKey]*sess) error {
	id, no := d.str(0), d.int(1)
	stake, _ := d.float(2)
	ret, _ := d.float(3)
	if d.err != nil {
		return nil
	}
	s := sessions[sessKey{id, int32(no)}]
	if no < 0 || no > math.MaxInt32 || s == nil {
		d.fail(1, "no event session for this round")
		return nil
	}
	s.netEUR += ret - stake
	return nil
}

func cents(x float64) float64 { return math.Round(x*100) / 100 }

func assemble(accts map[string]*acct, sessions map[sessKey]*sess) ([]fakeartifacts.Player, []fakeartifacts.Session) {
	players := make([]fakeartifacts.Player, 0, len(accts))
	for id, a := range accts {
		players = append(players, fakeartifacts.Player{ID: id, Operator: a.project, LastDay: a.lastDay,
			SessionCount: a.sessions, DepositCount: a.deposits})
	}
	slices.SortFunc(players, func(a, b fakeartifacts.Player) int { return strings.Compare(a.ID, b.ID) })
	out := make([]fakeartifacts.Session, 0, len(sessions))
	for k, s := range sessions {
		net := cents(s.netEUR)
		out = append(out, fakeartifacts.Session{
			PlayerID: k.acct, No: k.no, StartDay: s.startDay, DurationS: s.maxSecs, DepositsN: s.depN,
			DepositsEUR: cents(s.depEUR), NetResultEUR: net,
			IsNight: s.startHour <= nightEnd, IsChase: fakeartifacts.IsChase(s.maxSecs, s.depN, net),
		})
	}
	slices.SortFunc(out, func(a, b fakeartifacts.Session) int {
		if c := strings.Compare(a.PlayerID, b.PlayerID); c != 0 {
			return c
		}
		return int(a.No) - int(b.No)
	})
	return players, out
}

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
