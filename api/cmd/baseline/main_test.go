package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"

	"github.com/ante-rg/ante-app/api/internal/artifacts"
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
}

type rdRow struct {
	Acct      string   `parquet:"share_account_id"`
	SessionNo int64    `parquet:"bet_session_no"`
	Stake     *float64 `parquet:"stake_eur,optional"`
	Returned  *float64 `parquet:"returned_eur,optional"`
}

func writeParquet[T any](t *testing.T, path string, rows []T) {
	t.Helper()
	fh, err := os.Create(path)
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

func f(x float64) *float64 { return &x }

func export(t *testing.T, lastDay int64) string {
	t.Helper()
	dir := t.TempDir()
	events := []evRow{
		{"ZQ-ACCT-1", "P1", 1, 1, 2, 0, 3, "BET", nil, f(100)},
		{"ZQ-ACCT-1", "P1", 2, 1, 2, 100, 3, "DEPOSIT", f(50), f(150)},
		{"ZQ-ACCT-1", "P1", 3, 2, lastDay, 0, 12, "BET", nil, nil},
		{"ZQ-ACCT-1", "P1", 4, 2, lastDay, 600, 12, "BET", nil, f(90)},
		{"ZQ-ACCT-2", "P2", 1, 1, 1, 0, 10, "BET", nil, nil},
		{"ZQ-ACCT-2", "P2", 2, 1, 1, 60, 10, "BET", nil, nil},
	}
	rounds := []rdRow{{"ZQ-ACCT-1", 1, f(60), f(30)}}
	writeParquet(t, filepath.Join(dir, "events.parquet"), events)
	writeParquet(t, filepath.Join(dir, "rounds.parquet"), rounds)
	return dir
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
	if card["synthetic"] != false || card["model_version"] != "tsi-test-v0" || card["dataset_version"] != "TSI-100-v1" {
		t.Errorf("card = %v", card)
	}
	if m, ok := card["metrics"].([]any); !ok || len(m) != 0 {
		t.Errorf("metrics = %#v, want []", card["metrics"])
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
	if parquets != 6 || strings.Contains(stderr.String(), "ZQ-ACCT") {
		t.Errorf("stderr = %q", stderr.String())
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
	if err == nil || !strings.Contains(err.Error(), "events.parquet") {
		t.Errorf("err = %v, want events.parquet", err)
	}
}
