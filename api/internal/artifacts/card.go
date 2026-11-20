package artifacts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

var cardTextKeys = []string{"summary", "label_definition", "intended_use", "data_protection", "ethical_risks"}

func cardErr(column, rule string) error { return bad(FileCard, 0, column, rule) }

func num(x any) (float64, bool) {
	n, ok := x.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	return f, err == nil && finite(f)
}

func intLit(x any) (int, bool) {
	n, ok := x.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(n.String())
	return i, err == nil
}

func (v *validator) card() error {
	var c map[string]any
	dec := json.NewDecoder(bytes.NewReader(v.fo.Card))
	dec.UseNumber()
	if err := dec.Decode(&c); err != nil || c == nil {
		return cardErr("", "must be a JSON object")
	}
	for _, k := range cardTextKeys {
		if s, ok := c[k].(string); !ok || s == "" {
			return cardErr(k, "must be a non-empty string")
		}
	}
	limits, ok := c["limits"].([]any)
	if !ok {
		return cardErr("limits", "must be an array of strings")
	}
	for _, l := range limits {
		if _, ok := l.(string); !ok {
			return cardErr("limits", "must be an array of strings")
		}
	}
	if n, ok := intLit(c["default_capacity"]); !ok || n < 1 || n > 1000 {
		return cardErr("default_capacity", "must be an integer from 1 to 1000")
	}
	if t, ok := c["trained_at"]; ok && t != nil {
		s, isStr := t.(string)
		if _, err := time.Parse(time.RFC3339, s); !isStr || err != nil {
			return cardErr("trained_at", "must be an RFC 3339 date-time")
		}
	}
	if s, ok := c["synthetic"]; ok && s != nil {
		if _, isBool := s.(bool); !isBool {
			return cardErr("synthetic", "must be a boolean")
		}
	}
	metrics, ok := c["metrics"].([]any)
	if !ok {
		return cardErr("metrics", "must be an array")
	}
	type key struct {
		window        int
		split, method string
	}
	seen := map[key]bool{}
	for i, raw := range metrics {
		col := func(f string) string { return fmt.Sprintf("metrics[%d].%s", i, f) }
		m, ok := raw.(map[string]any)
		if !ok {
			return cardErr(fmt.Sprintf("metrics[%d]", i), "must be an object")
		}
		w, ok := intLit(m["window_days"])
		if !ok || (w != 7 && w != 14 && w != 30) {
			return cardErr(col("window_days"), "must be 7, 14 or 30")
		}
		split, ok := m["split"].(string)
		if !ok || (split != "all" && !v.ops[split]) {
			return cardErr(col("split"), "must be 'all' or an operator code in players.parquet")
		}
		method, ok := m["method"].(string)
		if !ok || (method != "model" && method != "rules") {
			return cardErr(col("method"), "must be 'model' or 'rules'")
		}
		if r, ok := num(m["early_recall"]); !ok || r < 0 || r > 1 {
			return cardErr(col("early_recall"), "must be a number from 0 to 1")
		}
		if l := m["median_lead_days"]; l != nil {
			if f, ok := num(l); !ok || f < 0 || f > maxLastDay {
				return cardErr(col("median_lead_days"), fmt.Sprintf("must be null or a number from 0 to %d", maxLastDay))
			}
		}
		k := key{w, split, method}
		if seen[k] {
			return cardErr(fmt.Sprintf("metrics[%d]", i), "duplicates window_days, split and method of an earlier row")
		}
		seen[k] = true
	}
	return nil
}
