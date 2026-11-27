package baseline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
)

type csvRow struct{ Order, TS, Type, Bet, Win, Amt, Bal string }

func r(order int, ts, typ, bet, win, amt, bal string) csvRow {
	return csvRow{fmt.Sprint(order), ts, typ, bet, win, amt, bal}
}

func writeAccount(t *testing.T, path string, rows []csvRow) {
	t.Helper()
	var b strings.Builder
	b.WriteString(accountHeader)
	for _, x := range rows {
		fmt.Fprintf(&b, "9911223344,e,%s,%s,%s,g,%s,%s,%s,%s,f\n", x.Order, x.TS, x.Type, x.Bet, x.Win, x.Amt, x.Bal)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	testNet    = "4455"
	testPlayer = "9911223344"
)

func fixture(t *testing.T, currency string, rows []csvRow) string {
	t.Helper()
	root := t.TempDir()
	writeBatch(t, root, "b1", []currencyRow{
		{NetID: "7", PlayerID: "1", Currency: "EUR"},
		{NetID: testNet, PlayerID: testPlayer, Currency: currency},
	})
	writeAccount(t, filepath.Join(root, "b1", "csv", "7", "1.csv"), []csvRow{r(1, "2021-11-01T00:00:00Z", "status", "", "", "", "0")})
	writeAccount(t, filepath.Join(root, "b1", "csv", testNet, testPlayer+".csv"), rows)
	return root
}

func built(t *testing.T, currency string, rows []csvRow) (fakeartifacts.Player, []fakeartifacts.Session, []fakeartifacts.Activity) {
	t.Helper()
	ps, ss, act, _, err := build(fixture(t, currency, rows), 2)
	if err != nil {
		t.Fatal(err)
	}
	var p fakeartifacts.Player
	for _, x := range ps {
		if x.Operator == "P2" {
			p = x
		}
	}
	var ms []fakeartifacts.Session
	for _, s := range ss {
		if s.PlayerID == p.ID {
			ms = append(ms, s)
		}
	}
	var ma []fakeartifacts.Activity
	for _, a := range act {
		if a.PlayerID == p.ID {
			ma = append(ma, a)
		}
	}
	return p, ms, ma
}

func TestBuild_Sessions(t *testing.T) {
	_, ss, _ := built(t, "EUR", []csvRow{
		r(1, "2021-11-01T03:00:00Z", "ROUND", "1", "0", "", "99"),
		r(2, "2021-11-01T03:59:00Z", "ROUND", "1", "0", "", "98"),
		r(3, "2021-11-01T05:00:00Z", "ROUND", "1", "0", "", "97"),
		r(4, "2021-11-02T06:00:00Z", "ROUND", "1", "0", "", "96"),
		r(5, "2021-11-03T10:00:00Z", "DepositWithdraw", "", "", "10", "50"),
		r(6, "2021-11-03T10:50:00Z", "DepositWithdraw", "", "", "10", "50"),
		r(7, "2021-11-03T11:40:00Z", "DepositWithdraw", "", "", "10", "50"),
		r(8, "2021-11-03T12:30:00Z", "ROUND", "5", "1", "", "46"),
		r(9, "2021-11-04T10:00:00Z", "ROUND", "1", "4", "", "49"),
	})
	if len(ss) != 5 {
		t.Fatalf("got %d sessions: %+v", len(ss), ss)
	}
	if !ss[0].IsNight || ss[0].DurationS != 59*60 || !ss[1].IsNight || ss[2].IsNight || ss[0].No != 1 || ss[1].No != 2 {
		t.Errorf("night/duration: %+v %+v %+v", ss[0], ss[1], ss[2])
	}
	if !ss[3].IsChase || ss[3].DepositsN != 3 || ss[3].DurationS != 9000 || ss[3].NetResultEUR != -4 {
		t.Errorf("chase: %+v", ss[3])
	}
	if ss[4].IsChase || ss[4].NetResultEUR != 3 {
		t.Errorf("win: %+v", ss[4])
	}
}

func TestBuild_MoneyAndCurrency(t *testing.T) {
	rt, err := loadRates()
	if err != nil {
		t.Fatal(err)
	}
	nov, _ := rt.rate("PLN", "2021-11")
	dec, _ := rt.rate("PLN", "2021-12")
	f := func(x float64) string { return fmt.Sprintf("%.10f", x) }
	_, ss, _ := built(t, "PLN", []csvRow{
		r(1, "2021-11-10T10:00:00Z", "DepositWithdraw", "", "", "464.62", "1"),
		r(2, "2021-11-30T23:30:00Z", "ROUND", f(nov*2), "0", "", "1"),
		r(3, "2021-12-01T00:10:00Z", "ROUND", "0", f(dec*3), "", "1"),
		r(4, "2021-12-10T10:00:00Z", "DepositWithdraw", "", "", f(dec*50), "1"),
		r(5, "2021-12-15T10:00:00Z", "FreeRound", "", f(dec*5), "", "1"),
		r(6, "2021-12-15T10:01:00Z", "GameAction:fbwin", "", "100", "", "1"),
		r(7, "2021-12-15T10:02:00Z", "BetRefund", "100", "", "100", "1"),
		r(8, "2021-12-15T10:03:00Z", "WithdrawalReserved", "", "", "100", "1"),
		r(9, "2021-12-15T10:04:00Z", "WithdrawalCancelled", "", "", "100", "1"),
	})
	if len(ss) != 4 {
		t.Fatalf("got %d sessions: %+v", len(ss), ss)
	}
	if ss[0].DepositsEUR != 100 || ss[0].DepositsN != 1 {
		t.Errorf("nov deposit: %+v", ss[0])
	}
	if ss[1].NetResultEUR != 1 || ss[1].DepositsN != 0 {
		t.Errorf("month boundary: %+v", ss[1])
	}
	if ss[2].DepositsEUR != 50 {
		t.Errorf("dec deposit: %+v", ss[2])
	}
	if ss[3].NetResultEUR != 5 || ss[3].DepositsN != 0 || ss[3].DepositsEUR != 0 {
		t.Errorf("free round: %+v", ss[3])
	}
}

func TestBuild_ConvertedBalanceIsRounded(t *testing.T) {
	rt, err := loadRates()
	if err != nil {
		t.Fatal(err)
	}
	if nov, _ := rt.rate("PLN", "2021-11"); nov != 4.646209 {
		t.Fatalf("rate fixture moved: %v", nov)
	}
	_, _, act := built(t, "PLN", []csvRow{r(1, "2021-11-10T10:00:00Z", "ROUND", "1", "0", "", "100.00")})
	if len(act) != 1 || act[0].EndBalanceEUR != 21.52 {
		t.Errorf("activity: %+v, want EndBalanceEUR 21.52", act)
	}
}

func TestBuild_DaysAndBalance(t *testing.T) {
	p, ss, act := built(t, "EUR", []csvRow{
		r(1, "2021-11-01T12:00:00Z", "ROUND", "1", "0", "", "10"),
		r(2, "2021-11-01T10:00:00Z", "selfexclusevOn", "", "", "", "20"),
		r(3, "2021-11-02T11:00:00Z", "ROUND", "1", "0", "", "30"),
		r(4, "2021-11-03T09:50:00Z", "ROUND", "1", "0", "", "40"),
		r(5, "2021-11-03T10:20:00Z", "ROUND", "1", "0", "", "45"),
		r(6, "2021-11-04T11:00:00Z", "ROUND", "1", "0", "", "50"),
	})
	if p.LastDay != 3 || p.SessionCount != 4 {
		t.Errorf("player: %+v", p)
	}
	for _, s := range ss {
		if s.StartDay < 0 {
			t.Errorf("negative day: %+v", s)
		}
	}
	if ss[0].StartDay != 0 || ss[0].DurationS != 7200 || ss[0].NetResultEUR != -1 {
		t.Errorf("out of order: %+v", ss[0])
	}
	if ss[2].StartDay != 1 || ss[2].DurationS != 30*60 {
		t.Errorf("over midnight: %+v", ss[2])
	}
	want := []struct {
		day      int32
		sessions int32
		bal      float64
	}{{0, 1, 20}, {1, 2, 40}, {3, 1, 50}}
	if len(act) != len(want) {
		t.Fatalf("activity: %+v", act)
	}
	for i, w := range want {
		if act[i].Day != w.day || act[i].SessionsN != w.sessions || act[i].EndBalanceEUR != w.bal {
			t.Errorf("activity[%d] = %+v, want %+v", i, act[i], w)
		}
	}
}

func TestBuild_PlayersCarryNoRealIDs(t *testing.T) {
	root := fixture(t, "EUR", []csvRow{r(1, "2021-11-01T10:00:00Z", "ROUND", "1", "0", "", "5")})
	ps, ss, act, _, err := build(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	idRe := regexp.MustCompile(`^A\d{5}$`)
	var strs []string
	for _, v := range []any{ps, ss, act} {
		rv := reflect.ValueOf(v)
		for i := 0; i < rv.Len(); i++ {
			for j := 0; j < rv.Index(i).NumField(); j++ {
				if f := rv.Index(i).Field(j); f.Kind() == reflect.String {
					strs = append(strs, f.String())
				}
			}
		}
	}
	for _, s := range strs {
		if strings.Contains(s, testPlayer) || strings.Contains(s, testNet) {
			t.Errorf("real id in %q", s)
		}
	}
	for _, p := range ps {
		if !idRe.MatchString(p.ID) || (p.Operator != "P1" && p.Operator != "P2") {
			t.Errorf("player %+v", p)
		}
	}
}

func TestBuild_Errors(t *testing.T) {
	ok := r(1, "2021-11-01T10:00:00Z", "ROUND", "1", "0", "", "5")
	tests := []struct {
		name, cur string
		rows      []csvRow
		raw       string
		rule      string
	}{
		{"missing column", "EUR", nil, "player_id,event_order,timestamp,event_type,bet_amount,win_amount,amount\n", "missing column"},
		{"order not increasing", "EUR", []csvRow{ok, r(1, "2021-11-01T11:00:00Z", "ROUND", "1", "0", "", "5")}, "", "must increase"},
		{"bad timestamp", "EUR", []csvRow{r(1, "2024-13-01", "ROUND", "1", "0", "", "5")}, "", "RFC 3339"},
		{"negative bet", "EUR", []csvRow{r(1, "2021-11-01T10:00:00Z", "ROUND", "-1", "0", "", "5")}, "", "must not be negative"},
		{"empty balance", "EUR", []csvRow{r(1, "2021-11-01T10:00:00Z", "ROUND", "1", "0", "", "")}, "", "must not be empty"},
		{"no events", "EUR", nil, "", "no events"},
		{"no rate", "PLN", []csvRow{r(1, "2020-01-15T10:00:00Z", "ROUND", "1", "0", "", "5")}, "", "no EUR rate for PLN 2020-01"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t, tc.cur, tc.rows)
			if tc.raw != "" {
				p := filepath.Join(root, "b1", "csv", testNet, testPlayer+".csv")
				if err := os.WriteFile(p, []byte(tc.raw), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, _, _, _, err := build(root, 2)
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("want *Error, got %v", err)
			}
			if !strings.Contains(e.Error(), tc.rule) || !strings.Contains(e.Error(), "account A0000") {
				t.Errorf("error %q lacks %q or the pseudonym", e.Error(), tc.rule)
			}
			for _, m := range []string{testPlayer, testNet, "csv", "2024-13", "2020-01-15", "2021-11-01"} {
				if strings.Contains(e.Error(), m) {
					t.Errorf("error %q leaks %q", e.Error(), m)
				}
			}
		})
	}
}

func TestBuild_SkippedCount(t *testing.T) {
	root := t.TempDir()
	writeBatch(t, root, "b1", []currencyRow{
		{NetID: "7", PlayerID: "1", Currency: "EUR"},
		{NetID: "7", PlayerID: "2", Currency: "RUB"},
		{NetID: testNet, PlayerID: testPlayer, Currency: "EUR"},
	})
	row := []csvRow{r(1, "2021-11-01T10:00:00Z", "ROUND", "1", "0", "", "5")}
	writeAccount(t, filepath.Join(root, "b1", "csv", "7", "1.csv"), row)
	writeAccount(t, filepath.Join(root, "b1", "csv", testNet, testPlayer+".csv"), row)
	ps, _, _, skipped, err := build(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 1 || len(ps) != 2 {
		t.Errorf("skipped %d, players %d", skipped, len(ps))
	}
}

func TestBuild_MissingFileNamesFolder(t *testing.T) {
	parent := t.TempDir()
	_, _, _, _, err := build(filepath.Join(parent, "nope"), 2)
	if err == nil || !strings.Contains(err.Error(), "nope") || strings.Contains(err.Error(), parent) {
		t.Errorf("err = %v", err)
	}
}
