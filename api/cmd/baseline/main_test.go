package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ante-rg/ante-app/api/internal/artifacts"
)

func export(t *testing.T, lastDay int) string {
	t.Helper()
	dir := t.TempDir()
	batch := filepath.Join(dir, "b1")
	day := func(d int, clock string) string {
		return time.Date(2021, 11, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, d).Format("2006-01-02") + "T" + clock + "Z"
	}
	events := map[string][]string{
		"1/9911223344": {
			"1,1," + day(0, "03:00:00") + ",ROUND,60,30,,100",
			"2,2," + day(0, "03:01:40") + ",DepositWithdraw,,,50,150",
			"3,3," + day(lastDay, "12:00:00") + ",ROUND,,,,95",
			"4,4," + day(lastDay, "12:10:00") + ",ROUND,,,,90",
		},
		"2/1":    {"1,1," + day(1, "10:00:00") + ",ROUND,,,,10", "2,2," + day(1, "10:01:00") + ",ROUND,,,,10"},
		"3/1":    {"1,1," + day(0, "10:00:00") + ",ROUND,,,,10"},
		"4/1":    {"1,1," + day(0, "10:00:00") + ",ROUND,,,,10"},
		"4455/1": {"1,1," + day(0, "10:00:00") + ",ROUND,,,,10"},
		"4455/2": {"1,1," + day(0, "10:00:00") + ",ROUND,,,,10"},
	}
	list := "file,net_id,player_id,currency,currency_basis\n"
	for k, rows := range events {
		net, player, _ := strings.Cut(k, "/")
		rel := filepath.Join("csv", net, player+".csv")
		currency := "EUR"
		if k == "4455/2" {
			currency = "RUB"
		}
		list += rel + "," + net + "," + player + "," + currency + ",x\n"
		body := "player_id,event_id,event_order,timestamp,event_type,bet_amount,win_amount,amount,balance_after\n"
		for _, row := range rows {
			body += player + "," + row + "\n"
		}
		write(t, filepath.Join(batch, rel), body)
	}
	write(t, filepath.Join(batch, "account_currency.csv"), list)
	return dir
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readCard(t *testing.T, out string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(out, "model_card.json"))
	if err != nil {
		t.Fatal(err)
	}
	var card map[string]any
	if err := json.Unmarshal(b, &card); err != nil {
		t.Fatal(err)
	}
	return card
}

func TestRun_WritesValidArtifacts(t *testing.T) {
	in := export(t, 5)
	out := filepath.Join(t.TempDir(), "tsi-test-v0")
	var stderr bytes.Buffer
	if err := run([]string{"-in", in, "-out", out}, &stderr); err != nil {
		t.Fatal(err)
	}
	folder, err := artifacts.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := folder.Validate(); err != nil {
		t.Fatal(err)
	}
	card := readCard(t, out)
	if card["synthetic"] != false || card["model_version"] != "tsi-test-v0" || card["dataset_version"] != "ANTE financial CSV v4 (2026-11-23)" {
		t.Errorf("card = %v", card)
	}
	if m, ok := card["metrics"].([]any); !ok || len(m) != 0 {
		t.Errorf("metrics = %#v, want []", card["metrics"])
	}
	limits, _ := card["limits"].([]any)
	found := false
	for _, l := range limits {
		found = found || strings.Contains(l.(string), "1 accounts")
	}
	if !found {
		t.Errorf("limits = %v, want a skipped-accounts entry with 1 accounts", limits)
	}
	if card["default_day"] != float64(5) {
		t.Errorf("default_day = %v, want 5", card["default_day"])
	}
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	parquets := 0
	for _, l := range lines {
		if strings.Contains(l, ".parquet ") && strings.HasSuffix(l, " rows") {
			parquets++
		}
	}
	if parquets != 6 {
		t.Errorf("stderr = %q", stderr.String())
	}
	logged := strings.NewReplacer(in, "", out, "").Replace(stderr.String())
	for _, id := range []string{"9911223344", "4455"} {
		if strings.Contains(logged, id) {
			t.Errorf("stderr leaks operator id %s: %q", id, logged)
		}
	}
	files, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 7 {
		t.Errorf("%d files in output, want 6 parquet + model_card.json", len(files))
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(out, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"9911223344", "4455"} {
			if bytes.Contains(b, []byte(id)) {
				t.Errorf("%s contains operator id %s", f.Name(), id)
			}
		}
	}
}

func TestRun_DefaultDayClamped(t *testing.T) {
	out := filepath.Join(t.TempDir(), "v0")
	var stderr bytes.Buffer
	if err := run([]string{"-in", export(t, 300), "-out", out}, &stderr); err != nil {
		t.Fatal(err)
	}
	if got := readCard(t, out)["default_day"]; got != float64(120) {
		t.Errorf("default_day = %v, want 120", got)
	}
	folder, err := artifacts.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := folder.Validate(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "review_sample.parquet 0 rows") {
		t.Errorf("review_sample empty: %q", stderr.String())
	}
}

func TestMoveIntoPlace(t *testing.T) {
	newTmp := func(t *testing.T) string {
		tmp := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmp, "new.txt"), []byte("n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return tmp
	}
	t.Run("missing target", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "out")
		if err := moveIntoPlace(newTmp(t), dir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
			t.Error(err)
		}
	})
	t.Run("empty target", func(t *testing.T) {
		dir := t.TempDir()
		if err := moveIntoPlace(newTmp(t), dir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
			t.Error(err)
		}
	})
	t.Run("non-empty target", func(t *testing.T) {
		dir := t.TempDir()
		keep := filepath.Join(dir, "keep.txt")
		if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := moveIntoPlace(newTmp(t), dir)
		if err == nil || !strings.Contains(err.Error(), "delete it first") {
			t.Fatalf("err = %v", err)
		}
		if _, statErr := os.Stat(keep); statErr != nil {
			t.Error("existing file was deleted")
		}
	})
}

func TestRun_RefusesNonEmptyOut(t *testing.T) {
	out := t.TempDir()
	keep := filepath.Join(out, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"-in", export(t, 5), "-out", out}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "delete it first") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(keep); statErr != nil {
		t.Error("existing file was touched")
	}
}

func TestRun_RequiresIn(t *testing.T) {
	out := filepath.Join(t.TempDir(), "v0")
	if err := run([]string{"-out", out}, &bytes.Buffer{}); err == nil {
		t.Error("want error without -in")
	}
	err := run([]string{"-in", t.TempDir(), "-out", out}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no batch folder") {
		t.Errorf("err = %v, want no batch folder", err)
	}
}
