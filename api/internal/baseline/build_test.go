package baseline

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"

	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

type evRow struct {
	Acct      string   `parquet:"share_account_id"`
	Project   string   `parquet:"project_code"`
	Order     int64    `parquet:"event_order"`
	SessionNo int64    `parquet:"session_no"`
	Day       int64    `parquet:"days_from_first_event"`
	Secs      int64    `parquet:"seconds_from_session_start"`
	Hour      int32    `parquet:"local_hour"`
	Type      string   `parquet:"event_type"`
	Pay       *float64 `parquet:"payment_amount_eur,optional"`
	Bal       *float64 `parquet:"balance_after_eur,optional"`
	Extra     string   `parquet:"unused_extra"`
}

type evNoHour struct {
	Acct      string   `parquet:"share_account_id"`
	Project   string   `parquet:"project_code"`
	Order     int64    `parquet:"event_order"`
	SessionNo int64    `parquet:"session_no"`
	Day       int64    `parquet:"days_from_first_event"`
	Secs      int64    `parquet:"seconds_from_session_start"`
	Type      string   `parquet:"event_type"`
	Pay       *float64 `parquet:"payment_amount_eur,optional"`
	Bal       *float64 `parquet:"balance_after_eur,optional"`
}

type rdRow struct {
	Acct      string   `parquet:"share_account_id"`
	SessionNo int64    `parquet:"bet_session_no"`
	Stake     *float64 `parquet:"stake_eur,optional"`
	Returned  *float64 `parquet:"returned_eur,optional"`
}

func f(x float64) *float64 { return &x }

func writeFile[T any](t *testing.T, dir, name string, rows []T) {
	t.Helper()
	fh, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fh.Close() }()
	w := parquet.NewGenericWriter[T](fh)
	if _, err := w.Write(rows); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeExport(t *testing.T, dir string, events []evRow, rounds []rdRow) {
	t.Helper()
	writeFile(t, dir, "events.parquet", events)
	writeFile(t, dir, "rounds.parquet", rounds)
}

func ev(acct, project string, order, sess, day, secs int64, hour int32, typ string, pay, bal *float64) evRow {
	return evRow{acct, project, order, sess, day, secs, hour, typ, pay, bal, "x"}
}

func fixture() ([]evRow, []rdRow) {
	events := []evRow{
		ev("S001", "P1", 3, 1, 2, 200, 3, "DEPOSIT", f(50), nil),
		ev("S001", "P1", 7, 3, 4, 0, 23, "BET", nil, nil),
		ev("S001", "P1", 1, 1, 2, 0, 3, "BET", nil, f(100)),
		ev("S001", "P1", 4, 1, 2, 7300, 3, "DEPOSIT", f(50), f(120)),
		ev("S001", "P1", 6, 2, 3, 8000, 6, "BET", nil, nil),
		ev("S001", "P1", 2, 1, 2, 100, 3, "DEPOSIT", f(50), f(150)),
		ev("S001", "P1", 8, 3, 5, 4000, 0, "BET", nil, f(77)),
		ev("S001", "P1", 5, 2, 3, 0, 6, "DEPOSIT", f(20), f(90)),
		ev("S002", "P2", 1, 1, 1, 0, 10, "BET", nil, nil),
		ev("S002", "P2", 2, 1, 1, 600, 10, "BET", nil, nil),
		ev("S002", "P2", 3, 2, 1, 0, 12, "BET", nil, nil),
		ev("S002", "P2", 4, 2, 1, 1200, 12, "BET", nil, nil),
		ev("S002", "P2", 5, 3, 3, 0, 1, "BET", nil, f(40)),
		ev("S002", "P2", 6, 3, 3, 60, 1, "BET", nil, nil),
	}
	rounds := []rdRow{
		{"S001", 1, f(60), f(30)},
		{"S001", 1, f(40), f(10)},
		{"S001", 2, f(10), f(15)},
	}
	return events, rounds
}

func build(t *testing.T) ([]fakeartifacts.Player, []fakeartifacts.Session, []fakeartifacts.Activity) {
	t.Helper()
	dir := t.TempDir()
	events, rounds := fixture()
	writeExport(t, dir, events, rounds)
	p, s, a, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p, s, a
}

func TestBuild_Players(t *testing.T) {
	p, _, _ := build(t)
	want := []fakeartifacts.Player{
		{ID: "S001", Operator: "P1", LastDay: 5, SessionCount: 3, DepositCount: 4},
		{ID: "S002", Operator: "P2", LastDay: 3, SessionCount: 3, DepositCount: 0},
	}
	if len(p) != len(want) {
		t.Fatalf("got %d players", len(p))
	}
	for i := range want {
		if p[i] != want[i] {
			t.Errorf("player %d: got %+v want %+v", i, p[i], want[i])
		}
	}
}

func TestBuild_Sessions(t *testing.T) {
	_, s, _ := build(t)
	want := []fakeartifacts.Session{
		{PlayerID: "S001", No: 1, StartDay: 2, DurationS: 7300, DepositsN: 3, DepositsEUR: 150, NetResultEUR: -60, IsNight: true, IsChase: true},
		{PlayerID: "S001", No: 2, StartDay: 3, DurationS: 8000, DepositsN: 1, DepositsEUR: 20, NetResultEUR: 5},
		{PlayerID: "S001", No: 3, StartDay: 4, DurationS: 4000},
	}
	if len(s) != 6 {
		t.Fatalf("got %d sessions", len(s))
	}
	for i := range want {
		if s[i] != want[i] {
			t.Errorf("session %d: got %+v want %+v", i, s[i], want[i])
		}
	}
	if s[0].PlayerID != "S001" || s[3].PlayerID != "S002" || s[5].No != 3 || !s[5].IsNight {
		t.Errorf("order or night wrong: %+v", s[3:])
	}
}

func TestBuild_Activity(t *testing.T) {
	_, _, a := build(t)
	want := []fakeartifacts.Activity{
		{PlayerID: "S001", Day: 2, DepositsN: 3, SessionsN: 1, ChaseSessionsN: 1, NightSessionsN: 1,
			DepositsEUR: 150, PlayMinutes: 7300.0 / 60, NetResultEUR: -60, EndBalanceEUR: 120},
		{PlayerID: "S001", Day: 3, DepositsN: 1, SessionsN: 1,
			DepositsEUR: 20, PlayMinutes: 8000.0 / 60, NetResultEUR: 5, EndBalanceEUR: 90},
		{PlayerID: "S001", Day: 4, SessionsN: 1, PlayMinutes: 4000.0 / 60, EndBalanceEUR: 90},
		{PlayerID: "S002", Day: 1, SessionsN: 2, PlayMinutes: 30},
		{PlayerID: "S002", Day: 3, SessionsN: 1, NightSessionsN: 1, PlayMinutes: 1, EndBalanceEUR: 40},
	}
	if len(a) != len(want) {
		t.Fatalf("got %d rows: %+v", len(a), a)
	}
	for i := range want {
		if a[i] != want[i] {
			t.Errorf("activity %d: got %+v want %+v", i, a[i], want[i])
		}
	}
}

func TestBuild_Activity_IgnoresGameStateBalance(t *testing.T) {
	dir := t.TempDir()
	writeExport(t, dir, []evRow{
		ev("G1", "P1", 1, 1, 2, 0, 3, "BET_PLACED", nil, f(42.5)),
		ev("G1", "P1", 2, 1, 2, 5, 3, "GAME_STATE", nil, f(0)),
	}, nil)
	_, _, a, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 1 || a[0].EndBalanceEUR != 42.5 {
		t.Fatalf("got %+v, want end balance 42.5", a)
	}
}

func TestBuild_Errors(t *testing.T) {
	const m = "LEAK-MARKER"
	base := func() evRow { return ev("A1", "P1", 1, 1, 0, 0, 0, "BET", nil, nil) }
	badEv := func(mut func(*evRow)) func(t *testing.T, dir string) {
		return func(t *testing.T, dir string) {
			r := ev(m, "P1", 2, 1, 0, 0, 0, "BET", nil, nil)
			mut(&r)
			writeExport(t, dir, []evRow{base(), r}, nil)
		}
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  Error
	}{
		{"missing local_hour", func(t *testing.T, dir string) {
			writeFile(t, dir, "events.parquet", []evNoHour{{Acct: m, Project: "P1", Type: "BET"}})
			writeFile(t, dir, "rounds.parquet", []rdRow{})
		}, Error{File: "events.parquet", Rule: "missing column local_hour"}},
		{"negative day", badEv(func(r *evRow) { r.Day = -1 }),
			Error{File: "events.parquet", Row: 2, Column: "days_from_first_event"}},
		{"hour 24", badEv(func(r *evRow) { r.Hour = 24 }),
			Error{File: "events.parquet", Row: 2, Column: "local_hour"}},
		{"null deposit", badEv(func(r *evRow) { r.Type = "DEPOSIT" }),
			Error{File: "events.parquet", Row: 2, Column: "payment_amount_eur"}},
		{"negative deposit", badEv(func(r *evRow) { r.Type = "DEPOSIT"; r.Pay = f(-5) }),
			Error{File: "events.parquet", Row: 2, Column: "payment_amount_eur"}},
		{"two projects", func(t *testing.T, dir string) {
			a := ev(m, "P1", 1, 1, 0, 0, 0, "BET", nil, nil)
			b := ev(m, "P2", 2, 1, 0, 0, 0, "BET", nil, nil)
			writeExport(t, dir, []evRow{a, b}, nil)
		}, Error{File: "events.parquet", Row: 2, Column: "project_code"}},
		{"orphan round", func(t *testing.T, dir string) {
			writeExport(t, dir, []evRow{base(), ev(m, "P1", 2, 1, 0, 0, 0, "BET", nil, nil)}, []rdRow{{m, 99, f(1), f(1)}})
		}, Error{File: "rounds.parquet", Row: 1, Column: "bet_session_no"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			c.setup(t, dir)
			_, _, _, err := Build(dir)
			var be *Error
			if !errors.As(err, &be) {
				t.Fatalf("want *Error, got %v", err)
			}
			if be.File != c.want.File || be.Row != c.want.Row || be.Column != c.want.Column {
				t.Errorf("got %+v want %+v", *be, c.want)
			}
			if c.want.Rule != "" && be.Rule != c.want.Rule {
				t.Errorf("rule: got %q want %q", be.Rule, c.want.Rule)
			}
			if be.Rule == "" || strings.Contains(err.Error(), m) {
				t.Errorf("bad message %q", err)
			}
		})
	}
}

func TestBuild_MissingFileNamesFolder(t *testing.T) {
	dir := t.TempDir()
	_, _, _, err := Build(dir)
	var be *Error
	if !errors.As(err, &be) {
		t.Fatalf("want *Error, got %v", err)
	}
	if be.File != "events.parquet" || !strings.Contains(be.Rule, dir) {
		t.Errorf("got %+v, want events.parquet with folder %q in rule", *be, dir)
	}
}
