package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func editCard(t *testing.T, edit func(map[string]any)) *Folder {
	t.Helper()
	dir := dummy(t)
	path := filepath.Join(dir, FileCard)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]any
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	edit(c)
	if raw, err = json.Marshal(c); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	fo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return fo
}

func metric(c map[string]any, i int) map[string]any { return c["metrics"].([]any)[i].(map[string]any) }

func baseCard(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dummy(t), FileCard))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func checkCard(raw []byte) error {
	ops := map[string]bool{"P1": true, "P2": true, "P3": true, "P4": true, "P5": true}
	return (&validator{fo: &Folder{Card: raw}, ops: ops}).card()
}

func mapEdit(t *testing.T, raw []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var c map[string]any
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	edit(c)
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func wantCardErr(t *testing.T, raw []byte, column, rule string) {
	t.Helper()
	e := asErr(t, checkCard(raw))
	if *e != (Error{File: FileCard, Column: column, Rule: rule}) {
		t.Errorf("got %+v, want column %q rule %q", *e, column, rule)
	}
}

func TestValidate_Card(t *testing.T) {
	const text = "must be a non-empty string"
	set := func(k string, v any) func(map[string]any) { return func(c map[string]any) { c[k] = v } }
	del := func(k string) func(map[string]any) { return func(c map[string]any) { delete(c, k) } }
	setM := func(k string, v any) func(map[string]any) {
		return func(c map[string]any) { metric(c, 0)[k] = v }
	}
	base := baseCard(t)
	cases := []struct {
		name         string
		edit         func(map[string]any)
		column, rule string
	}{
		{"no summary", del("summary"), "summary", text},
		{"no label_definition", del("label_definition"), "label_definition", text},
		{"no intended_use", del("intended_use"), "intended_use", text},
		{"no data_protection", del("data_protection"), "data_protection", text},
		{"no ethical_risks", del("ethical_risks"), "ethical_risks", text},
		{"empty summary", set("summary", ""), "summary", text},
		{"limits string", set("limits", "x"), "limits", "must be an array of strings"},
		{"limits number", set("limits", []any{1}), "limits", "must be an array of strings"},
		{"no capacity", del("default_capacity"), "default_capacity", "must be an integer from 1 to 1000"},
		{"capacity 0", set("default_capacity", 0), "default_capacity", "must be an integer from 1 to 1000"},
		{"capacity 1001", set("default_capacity", 1001), "default_capacity", "must be an integer from 1 to 1000"},
		{"capacity 2.5", set("default_capacity", 2.5), "default_capacity", "must be an integer from 1 to 1000"},
		{"metrics object", set("metrics", map[string]any{}), "metrics", "must be an array"},
		{"metrics[0] string", func(c map[string]any) { c["metrics"].([]any)[0] = "x" }, "metrics[0]", "must be an object"},
		{"window 5", setM("window_days", 5), "metrics[0].window_days", "must be 7, 14 or 30"},
		{"split P9", setM("split", "P9"), "metrics[0].split", "must be 'all' or an operator code in players.parquet"},
		{"method score", setM("method", "score"), "metrics[0].method", "must be 'model' or 'rules'"},
		{"recall 1.2", setM("early_recall", 1.2), "metrics[0].early_recall", "must be a number from 0 to 1"},
		{"recall -0.1", setM("early_recall", -0.1), "metrics[0].early_recall", "must be a number from 0 to 1"},
		{"lead -1", setM("median_lead_days", -1), "metrics[0].median_lead_days", "must be null or a number from 0 to 36500"},
		{"lead 36501", setM("median_lead_days", 36501), "metrics[0].median_lead_days", "must be null or a number from 0 to 36500"},
		{"lead 1e39", setM("median_lead_days", 1e39), "metrics[0].median_lead_days", "must be null or a number from 0 to 36500"},
		{"duplicate row", func(c map[string]any) { c["metrics"].([]any)[1] = metric(c, 0) }, "metrics[1]",
			"duplicates window_days, split and method of an earlier row"},
		{"synthetic yes", set("synthetic", "yes"), "synthetic", "must be a boolean"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wantCardErr(t, mapEdit(t, base, tc.edit), tc.column, tc.rule) })
	}
}

func TestValidate_CardRawJSON(t *testing.T) {
	base := baseCard(t)
	sub := func(re, repl string) []byte {
		r := regexp.MustCompile(re)
		if !r.Match(base) {
			t.Fatalf("pattern %q not in dummy card", re)
		}
		return r.ReplaceAll(base, []byte(repl))
	}
	const rfc = "must be an RFC 3339 date-time"
	cases := []struct {
		name         string
		raw          []byte
		column, rule string
	}{
		{"capacity 10.0", sub(`"default_capacity":\s*\d+`, `"default_capacity":10.0`), "default_capacity", "must be an integer from 1 to 1000"},
		{"window 7.0", sub(`"window_days":\s*\d+`, `"window_days":7.0`), "metrics[0].window_days", "must be 7, 14 or 30"},
		{"trained_at date only", mapEdit(t, base, func(c map[string]any) { c["trained_at"] = "2026-11-01" }), "trained_at", rfc},
		{"trained_at number", mapEdit(t, base, func(c map[string]any) { c["trained_at"] = 5 }), "trained_at", rfc},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wantCardErr(t, tc.raw, tc.column, tc.rule) })
	}
	ok := mapEdit(t, base, func(c map[string]any) { c["trained_at"] = "2026-11-01T10:00:00Z" })
	if err := checkCard(ok); err != nil {
		t.Errorf("RFC 3339 trained_at: want pass, got %v", err)
	}
}

func TestValidate_CardNoLeak(t *testing.T) {
	err := editCard(t, func(c map[string]any) { metric(c, 0)["split"] = leak }).Validate()
	if err == nil || strings.Contains(err.Error(), leak) {
		t.Errorf("error must exist and not contain the value: %v", err)
	}
}

func TestValidate_CardAccepts(t *testing.T) {
	base := baseCard(t)
	cases := map[string]func(map[string]any){
		"empty limits":    func(c map[string]any) { c["limits"] = []any{} },
		"empty metrics":   func(c map[string]any) { c["metrics"] = []any{} },
		"unknown key":     func(c map[string]any) { c["extra"] = 1 },
		"null lead":       func(c map[string]any) { metric(c, 0)["median_lead_days"] = nil },
		"recall 0":        func(c map[string]any) { metric(c, 0)["early_recall"] = 0 },
		"recall 1":        func(c map[string]any) { metric(c, 0)["early_recall"] = 1 },
		"no synthetic":    func(c map[string]any) { delete(c, "synthetic") },
		"null trained_at": func(c map[string]any) { c["trained_at"] = nil },
		"null synthetic":  func(c map[string]any) { c["synthetic"] = nil },
		"lead 36500":      func(c map[string]any) { metric(c, 0)["median_lead_days"] = 36500 },
		"no 'all' split": func(c map[string]any) {
			var keep []any
			for _, m := range c["metrics"].([]any) {
				if m.(map[string]any)["split"] != "all" {
					keep = append(keep, m)
				}
			}
			c["metrics"] = keep
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			if err := checkCard(mapEdit(t, base, edit)); err != nil {
				t.Fatalf("want pass, got %v", err)
			}
		})
	}
}
