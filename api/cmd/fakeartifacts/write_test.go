package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/parquet-go/parquet-go"
)

var parquetColumns = map[string][]string{
	"players":        {"player_id", "operator_code", "last_day", "session_count", "deposit_count"},
	"daily_scores":   {"player_id", "day", "score", "reasons"},
	"daily_activity": {"player_id", "day", "deposits_n", "deposits_eur", "sessions_n", "play_minutes", "net_result_eur", "chase_sessions_n", "night_sessions_n", "end_balance_eur"},
	"sessions":       {"player_id", "session_no", "start_day", "duration_s", "deposits_n", "deposits_eur", "net_result_eur", "is_night", "is_chase"},
	"rule_flags":     {"player_id", "day", "rule_id", "rule_text"},
	"review_sample":  {"player_id", "source", "position"},
}

var allFiles = []string{"players.parquet", "daily_scores.parquet", "daily_activity.parquet", "sessions.parquet", "rule_flags.parquet", "review_sample.parquet", "model_card.json"}

func openParquet(t *testing.T, path string) *parquet.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	pf, err := parquet.OpenFile(f, st.Size())
	if err != nil {
		t.Fatal(err)
	}
	return pf
}

func TestRun_WritesAllFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dummy-v0")
	if err := run([]string{"-out", dir, "-seed", "1"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	ds := Generate(1)
	wantRows := map[string]int{
		"players": len(ds.Players), "daily_scores": len(ds.Scores), "daily_activity": len(ds.Activity),
		"sessions": len(ds.Sessions), "rule_flags": len(ds.RuleFlags), "review_sample": len(ds.Sample),
	}
	for _, name := range allFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	for table, cols := range parquetColumns {
		pf := openParquet(t, filepath.Join(dir, table+".parquet"))
		var got []string
		for _, f := range pf.Schema().Fields() {
			got = append(got, f.Name())
		}
		if !slices.Equal(got, cols) {
			t.Errorf("%s columns = %v, want %v", table, got, cols)
		}
		if int(pf.NumRows()) != wantRows[table] {
			t.Errorf("%s rows = %d, want %d", table, pf.NumRows(), wantRows[table])
		}
	}

	type scoreRow struct {
		PlayerID string   `parquet:"player_id"`
		Day      int32    `parquet:"day"`
		Score    *float64 `parquet:"score,optional"`
		Reasons  string   `parquet:"reasons"`
	}
	rows, err := parquet.ReadFile[scoreRow](filepath.Join(dir, "daily_scores.parquet"))
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Score != nil || rows[0].Reasons != "[]" {
		t.Errorf("first row = %+v, want null score and []", rows[0])
	}
	i := slices.IndexFunc(rows, func(r scoreRow) bool { return r.Day == 7 })
	var rs []Reason
	if err := json.Unmarshal([]byte(rows[i].Reasons), &rs); err != nil || len(rs) != 3 || rows[i].Score == nil {
		t.Errorf("day 7 row = %+v (err %v), want score and 3 reasons", rows[i], err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "model_card.json"))
	if err != nil {
		t.Fatal(err)
	}
	var card struct {
		ModelVersion    string            `json:"model_version"`
		Synthetic       bool              `json:"synthetic"`
		DefaultDay      int               `json:"default_day"`
		DefaultCapacity int               `json:"default_capacity"`
		Sections        map[string]string `json:"sections"`
	}
	if err := json.Unmarshal(raw, &card); err != nil {
		t.Fatal(err)
	}
	if card.ModelVersion != "dummy-v0" || !card.Synthetic || card.DefaultDay != 120 || card.DefaultCapacity != 10 {
		t.Errorf("card = %+v", card)
	}
	for _, k := range []string{"summary", "intended_use", "limitations"} {
		if card.Sections[k] == "" {
			t.Errorf("sections.%s empty", k)
		}
	}
}

func TestRun_Deterministic(t *testing.T) {
	a, b := filepath.Join(t.TempDir(), "x"), filepath.Join(t.TempDir(), "x")
	for _, d := range []string{a, b} {
		if err := run([]string{"-out", d, "-seed", "1"}, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range allFiles {
		ba, err := os.ReadFile(filepath.Join(a, name))
		if err != nil {
			t.Fatal(err)
		}
		bb, err := os.ReadFile(filepath.Join(b, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(ba, bb) {
			t.Errorf("%s differs between runs", name)
		}
	}
}

func TestRun_ModelVersionFromDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "foo-v9")
	if err := run([]string{"-out", dir}, io.Discard); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "model_card.json"))
	var card struct {
		ModelVersion string `json:"model_version"`
	}
	if err := json.Unmarshal(raw, &card); err != nil || card.ModelVersion != "foo-v9" {
		t.Errorf("model_version = %q (err %v)", card.ModelVersion, err)
	}
}

func TestRun_ReplacesOutDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, body := range map[string]string{"model_card.json": `{"synthetic": true}`, "stale.txt": "x"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := run([]string{"-out", dir}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.txt")); !os.IsNotExist(err) {
		t.Errorf("stale.txt survived: %v", err)
	}
	if _, err := os.Stat(dir + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("%s.tmp left behind: %v", dir, err)
	}
}

func TestRun_RefusesNonSyntheticCard(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	card, keep := filepath.Join(dir, "model_card.json"), filepath.Join(dir, "keep.txt")
	for p, body := range map[string]string{card: `{"synthetic": false}`, keep: "x"} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := run([]string{"-out", dir}, io.Discard); err == nil {
		t.Error("run succeeded on a non-synthetic folder")
	}
	for _, p := range []string{card, keep} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s lost: %v", p, err)
		}
	}
}

func TestRun_RefusesForeignDir(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-out", dir}, io.Discard); err == nil {
		t.Error("run succeeded on a foreign dir")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("keep.txt lost: %v", err)
	}
}

func TestRun_RejectsEmptyOut(t *testing.T) {
	if err := run([]string{"-out", ""}, io.Discard); err == nil {
		t.Error("run succeeded with empty -out")
	}
}

func TestRun_TrailingSlashOut(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dummy-v0")
	for i := 0; i < 2; i++ {
		if err := run([]string{"-out", dir + "/"}, io.Discard); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	for _, name := range allFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s missing: %v", name, err)
		}
	}
}

func TestRun_KeepsSiblingTmpDir(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "x.tmp", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-out", filepath.Join(root, "x")}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("x.tmp/keep.txt lost: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("leftover entries in %s: %v", root, entries)
	}
}

func TestRun_RejectsPositionalArgs(t *testing.T) {
	if err := run([]string{"-out", filepath.Join(t.TempDir(), "x"), "extra"}, io.Discard); err == nil {
		t.Error("run succeeded with a positional argument")
	}
}

func TestRun_HelpIsNotAnError(t *testing.T) {
	if err := run([]string{"-h"}, io.Discard); err != nil {
		t.Errorf("run -h: %v", err)
	}
}
