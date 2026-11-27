package baseline

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const accountHeader = "player_id,event_id,event_order,timestamp,event_type,game_id,bet_amount,win_amount,amount,balance_after,funding_source\n"

type currencyRow struct {
	NetID, PlayerID, Currency string
	File                      string
	NoFile                    bool
}

func writeBatch(t *testing.T, root, batch string, rows []currencyRow) {
	t.Helper()
	dir := filepath.Join(root, batch)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("file,net_id,player_id,currency,currency_basis\n")
	for _, r := range rows {
		f := r.File
		if f == "" {
			f = filepath.Join("csv", r.NetID, r.PlayerID+".csv")
		}
		fmt.Fprintf(&b, "%s,%s,%s,%s,x\n", f, r.NetID, r.PlayerID, r.Currency)
		if r.NoFile || r.File != "" {
			continue
		}
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(accountHeader), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "account_currency.csv"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

var testRates = rates{"SEK": {"2020-01": 10}}

func TestListAccounts_PseudonymsAndBrands(t *testing.T) {
	rowsA := []currencyRow{{NetID: "30", PlayerID: "501", Currency: "EUR"}, {NetID: "4", PlayerID: "502", Currency: "SEK"}}
	rowsB := []currencyRow{{NetID: "100", PlayerID: "503", Currency: "RUB"}, {NetID: "4", PlayerID: "504", Currency: "EUR"}}
	root := t.TempDir()
	writeBatch(t, root, "b1", rowsA)
	writeBatch(t, root, "b2", rowsB)

	accts, skipped, err := listAccounts(root, testRates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(accts) != 3 || skipped != 1 {
		t.Fatalf("got %d accounts, %d skipped", len(accts), skipped)
	}

	keys := []string{"30/501", "4/502", "100/503", "4/504"}
	sort.Slice(keys, func(i, j int) bool {
		a, b := sha256.Sum256([]byte(keys[i])), sha256.Sum256([]byte(keys[j]))
		return string(a[:]) < string(b[:])
	})
	wantID := map[string]string{}
	for i, k := range keys {
		wantID[k] = fmt.Sprintf("A%05d", i+1)
	}
	wantBrand := map[string]string{"4": "P1", "30": "P2", "100": "P3"}
	for _, a := range accts {
		var key string
		for k, id := range wantID {
			if id == a.ID {
				key = k
			}
		}
		if key == "" || strings.HasPrefix(key, "100/") {
			t.Fatalf("unexpected account %+v", a)
		}
		if a.Operator != wantBrand[strings.Split(key, "/")[0]] {
			t.Errorf("%s: operator %s", a.ID, a.Operator)
		}
		if !filepath.IsAbs(a.Path) {
			t.Errorf("path not absolute: %s", a.Path)
		}
	}
	if !sort.SliceIsSorted(accts, func(i, j int) bool { return accts[i].ID < accts[j].ID }) {
		t.Error("accts not sorted by ID")
	}

	root2 := t.TempDir()
	writeBatch(t, root2, "b1", rowsB)
	writeBatch(t, root2, "b2", rowsA)
	accts2, skipped2, err := listAccounts(root2, testRates, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i := range accts2 {
		accts2[i].Path = ""
	}
	for i := range accts {
		accts[i].Path = ""
	}
	if skipped2 != skipped || !reflect.DeepEqual(accts, accts2) {
		t.Errorf("batch order changed result:\n%v\n%v", accts, accts2)
	}
}

func TestListAccounts_SamePlayerIDOnTwoBrands(t *testing.T) {
	root := t.TempDir()
	writeBatch(t, root, "b1", []currencyRow{{NetID: "4", PlayerID: "777", Currency: "EUR"}, {NetID: "30", PlayerID: "777", Currency: "EUR"}})
	accts, _, err := listAccounts(root, testRates, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(accts) != 2 || accts[0].ID == accts[1].ID {
		t.Fatalf("got %+v", accts)
	}
}

func TestListAccounts_Errors(t *testing.T) {
	good := func(net, id string) currencyRow { return currencyRow{NetID: net, PlayerID: id, Currency: "EUR"} }
	tests := []struct {
		name   string
		setup  func(t *testing.T, root string)
		brands int
		rule   string
	}{
		{"no batch", func(t *testing.T, root string) {}, 1, "no batch folder with account_currency.csv"},
		{"missing column", func(t *testing.T, root string) {
			d := filepath.Join(root, "b1")
			_ = os.MkdirAll(d, 0o755)
			_ = os.WriteFile(filepath.Join(d, "account_currency.csv"), []byte("file,net_id,player_id\nx.csv,30,9911223344\n"), 0o644)
		}, 1, "missing column"},
		{"bad net_id", func(t *testing.T, root string) {
			writeBatch(t, root, "b1", []currencyRow{{NetID: "x", PlayerID: "9911223344", Currency: "EUR", NoFile: true}})
		}, 1, "net_id"},
		{"file escapes", func(t *testing.T, root string) {
			writeBatch(t, root, "b1", []currencyRow{{NetID: "30", PlayerID: "9911223344", Currency: "EUR", File: "../x.csv"}})
		}, 1, "file"},
		{"duplicate", func(t *testing.T, root string) {
			writeBatch(t, root, "b1", []currencyRow{good("30", "9911223344")})
			writeBatch(t, root, "b2", []currencyRow{good("30", "9911223344")})
		}, 1, "duplicate account"},
		{"unknown currency", func(t *testing.T, root string) {
			writeBatch(t, root, "b1", []currencyRow{{NetID: "30", PlayerID: "9911223344", Currency: "XAU"}})
		}, 1, "no EUR rate for currency XAU"},
		{"data-like currency", func(t *testing.T, root string) {
			writeBatch(t, root, "b1", []currencyRow{{NetID: "30", PlayerID: "9911223344", Currency: "9911223344"}})
		}, 1, "unknown currency"},
		{"file missing", func(t *testing.T, root string) {
			writeBatch(t, root, "b1", []currencyRow{{NetID: "30", PlayerID: "9911223344", Currency: "EUR", NoFile: true}})
		}, 1, "file listed in account_currency.csv is missing"},
		{"brand count", func(t *testing.T, root string) {
			writeBatch(t, root, "b1", []currencyRow{good("4", "9911223344"), good("30", "777"), good("100", "778")})
		}, 5, "want 5 brands, found 3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			_, _, err := listAccounts(root, testRates, tc.brands)
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("want *Error, got %v", err)
			}
			if !strings.Contains(e.Error(), tc.rule) {
				t.Errorf("error %q lacks %q", e.Error(), tc.rule)
			}
			for _, m := range []string{"9911223344", "777", "30/", "../x.csv"} {
				if strings.Contains(e.Error(), m) {
					t.Errorf("error %q leaks %q", e.Error(), m)
				}
			}
		})
	}
}
