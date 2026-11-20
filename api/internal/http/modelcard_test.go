package http_test

import (
	"testing"
)

const fullCard = `{"model_version":"test-v0","synthetic":true,"trained_at":"2026-11-01T10:00:00Z",
"summary":"S","label_definition":"L","intended_use":"I","data_protection":"D","ethical_risks":"E",
"limits":["a","b"],"default_capacity":12,"extra":1,
"metrics":[
{"window_days":7,"split":"P2","method":"model","early_recall":0.5,"median_lead_days":3.5},
{"window_days":14,"split":"all","method":"model","early_recall":0.6,"median_lead_days":null},
{"window_days":30,"split":"all","method":"rules","early_recall":0.2,"median_lead_days":1}]}`

func TestModelCard_Full(t *testing.T) {
	srv := serve(t, &fixture{Card: fullCard, Players: players()})
	rec, body := get(t, srv, "/api/v1/model")
	if rec.Code != 200 {
		t.Fatalf("got %d %v", rec.Code, body)
	}
	for k, want := range map[string]any{
		"model_version": "test-v0", "synthetic": true, "trained_at": "2026-11-01T10:00:00Z",
		"summary": "S", "label_definition": "L", "intended_use": "I", "data_protection": "D",
		"ethical_risks": "E", "default_capacity": float64(12),
	} {
		if body[k] != want {
			t.Errorf("%s = %v, want %v", k, body[k], want)
		}
	}
	if _, ok := body["extra"]; ok {
		t.Error("extra key leaked")
	}
	limits := body["limits"].([]any)
	if len(limits) != 2 || limits[0] != "a" || limits[1] != "b" {
		t.Errorf("limits = %v", limits)
	}
	m := body["metrics"].([]any)
	if len(m) != 3 {
		t.Fatalf("metrics = %v", m)
	}
	first, second, third := m[0].(map[string]any), m[1].(map[string]any), m[2].(map[string]any)
	if first["split"] != "P2" || first["method"] != "model" || first["window_days"] != float64(7) ||
		first["early_recall"] != 0.5 || first["median_lead_days"] != 3.5 {
		t.Errorf("first = %v", first)
	}
	if v, ok := second["median_lead_days"]; !ok || v != nil {
		t.Errorf("median_lead_days should be null, got %v (present %v)", v, ok)
	}
	if third["split"] != "all" || third["method"] != "rules" {
		t.Errorf("third = %v", third)
	}
}

func TestModelCard_MissingArrays(t *testing.T) {
	card := `{"model_version":"test-v0","summary":"S","label_definition":"L","intended_use":"I","data_protection":"D","ethical_risks":"E","default_capacity":10}`
	srv := serve(t, &fixture{Card: card, Players: players()})
	rec, body := get(t, srv, "/api/v1/model")
	if rec.Code != 200 {
		t.Fatalf("got %d %v", rec.Code, body)
	}
	for _, k := range []string{"limits", "metrics"} {
		a, ok := body[k].([]any)
		if !ok || len(a) != 0 {
			t.Errorf("%s = %v, want []", k, body[k])
		}
	}
}

func TestModelCard_OldShape(t *testing.T) {
	srv := serve(t, &fixture{Players: players()})
	rec, body := get(t, srv, "/api/v1/model")
	if rec.Code != 404 || body["code"] != "no_model_card" ||
		body["message"] != "The active model has no card in the current format. Load its artifacts again." {
		t.Fatalf("got %d %v", rec.Code, body)
	}
}

func TestModelCard_MetricsObject(t *testing.T) {
	srv := serve(t, &fixture{Card: `{"model_version":"test-v0","summary":"S","metrics":{}}`, Players: players()})
	rec, body := get(t, srv, "/api/v1/model")
	if rec.Code != 404 || body["code"] != "no_model_card" {
		t.Fatalf("got %d %v", rec.Code, body)
	}
}

func TestModelCard_NoModel(t *testing.T) {
	rec, body := get(t, serve(t, nil), "/api/v1/model")
	if rec.Code != 404 || body["code"] != "no_active_model" {
		t.Fatalf("got %d %v", rec.Code, body)
	}
}
