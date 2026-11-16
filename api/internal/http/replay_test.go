package http_test

import "testing"

func flat(last int32) []*float64 {
	out := make([]*float64, last+1)
	for i := range out {
		out[i] = sc(0.5)
	}
	return out
}

func players() []fxPlayer {
	return []fxPlayer{
		{ID: "p1", Operator: "P1", LastDay: 2, Scores: flat(2)},
		{ID: "p2", Operator: "P1", LastDay: 4, Scores: flat(4)},
		{ID: "p3", Operator: "P2", LastDay: 7, Scores: flat(7)},
	}
}

func replayWithCard(t *testing.T, card string) map[string]any {
	t.Helper()
	rec, body := get(t, serve(t, &fixture{Card: card, Players: players()}), "/api/v1/replay")
	if rec.Code != 200 {
		t.Fatalf("code = %d %v", rec.Code, body)
	}
	return body
}

func TestReplay_NoActiveModel(t *testing.T) {
	rec, body := get(t, serve(t, nil), "/api/v1/replay")
	if rec.Code != 404 || body["code"] != "no_active_model" {
		t.Fatalf("got %d %v", rec.Code, body)
	}
}

func TestReplay_NoRanks(t *testing.T) {
	rec, body := get(t, serve(t, &fixture{SkipRanks: true, Players: players()}), "/api/v1/replay")
	if rec.Code != 404 || body["code"] != "no_ranks" {
		t.Fatalf("got %d %v", rec.Code, body)
	}
}

func TestReplay_OK(t *testing.T) {
	body := replayWithCard(t, `{"model_version":"test-v0","default_day":3}`)
	meta, _ := body["meta"].(map[string]any)
	want := map[string]any{"min_day": 0.0, "max_day": 7.0, "default_day": 3.0, "model_version": "test-v0"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %v, want %v", k, body[k], v)
		}
	}
	if meta["capacity"] != 10.0 || meta["dataset_version"] != nil || meta["feature_set_version"] != nil || meta["model_version"] != "test-v0" {
		t.Errorf("meta = %v", meta)
	}
}

func TestReplay_DefaultDayClamped(t *testing.T) {
	if got := replayWithCard(t, `{"default_day":999}`)["default_day"]; got != 7.0 {
		t.Fatalf("default_day = %v", got)
	}
}

func TestReplay_CardVersions(t *testing.T) {
	body := replayWithCard(t, `{"dataset_version":"tsi-100-v1","feature_set_version":"f1","default_capacity":25}`)
	meta, _ := body["meta"].(map[string]any)
	if meta["dataset_version"] != "tsi-100-v1" || meta["feature_set_version"] != "f1" || meta["capacity"] != 25.0 {
		t.Fatalf("meta = %v", meta)
	}
}

func TestReplay_BadCapacityInCard(t *testing.T) {
	body := replayWithCard(t, `{"default_capacity":0}`)
	if meta, _ := body["meta"].(map[string]any); meta["capacity"] != 10.0 {
		t.Fatalf("meta = %v", meta)
	}
}
