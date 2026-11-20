package http_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	apihttp "github.com/ante-rg/ante-app/api/internal/http"
)

func operatorsFixture() *fixture {
	var ps []fxPlayer
	for i := 1; i <= 20; i++ {
		p := fxPlayer{ID: fmt.Sprintf("S%02d", i), Operator: "P1", LastDay: 5}
		if i > 10 {
			p.Operator = "P2"
		}
		if i == 11 {
			p.LastDay = 2
		}
		for d := 0; d <= int(p.LastDay); d++ {
			var s *float64
			if i < 20 {
				s = sc(0.5 - 0.01*float64(i))
			}
			p.Scores = append(p.Scores, s)
		}
		ps = append(ps, p)
	}
	ps[0].Flags = []int32{2}
	ps[1].Flags = []int32{5}
	return &fixture{Players: ps}
}

func operatorsServer(t *testing.T) *apihttp.Server {
	t.Helper()
	s := serve(t, operatorsFixture())
	if _, err := s.DB.Exec(context.Background(), `INSERT INTO operators (code, name) VALUES ('P3', 'P3')`); err != nil {
		t.Fatal(err)
	}
	return s
}

func operatorRows(t *testing.T, s *apihttp.Server, query string) (day float64, order []string, rows map[string][]int) {
	t.Helper()
	rec, body := get(t, s, "/api/v1/operators"+query)
	if rec.Code != 200 {
		t.Fatalf("code = %d %v", rec.Code, body)
	}
	rows = map[string][]int{}
	for _, it := range itemsOf(t, body) {
		code := it["operator_code"].(string)
		order = append(order, code)
		var v []int
		for _, k := range []string{"players", "high", "elevated", "low", "insufficient_data", "flagged_by_rules_too", "reviewed"} {
			v = append(v, int(it[k].(float64)))
		}
		rows[code] = v
	}
	return body["as_of_day"].(float64), order, rows
}

func wantRow(t *testing.T, rows map[string][]int, code string, want []int) {
	t.Helper()
	if !reflect.DeepEqual(rows[code], want) {
		t.Fatalf("%s = %v, want %v", code, rows[code], want)
	}
}

func TestOperators_Counts(t *testing.T) {
	s := operatorsServer(t)
	day, order, rows := operatorRows(t, s, "?as_of_day=3")
	if day != 3 || !reflect.DeepEqual(order, []string{"P1", "P2", "P3"}) {
		t.Fatalf("day = %v order = %v", day, order)
	}
	wantRow(t, rows, "P1", []int{10, 2, 2, 6, 0, 1, 0})
	wantRow(t, rows, "P2", []int{10, 0, 0, 9, 1, 0, 0})
	wantRow(t, rows, "P3", []int{0, 0, 0, 0, 0, 0, 0})
}

func TestOperators_DayCutoff(t *testing.T) {
	s := operatorsServer(t)
	if rec, b := post(t, s, "S01", reviewBody(4, "no_concern", "n", "normal")); rec.Code != 201 {
		t.Fatalf("review = %d %v", rec.Code, b)
	}
	_, _, rows := operatorRows(t, s, "?as_of_day=3")
	wantRow(t, rows, "P1", []int{10, 2, 2, 6, 0, 1, 0})
	_, _, rows = operatorRows(t, s, "?as_of_day=5")
	wantRow(t, rows, "P1", []int{10, 2, 2, 6, 0, 2, 1})
}

func TestOperators_EndedHistory(t *testing.T) {
	s := operatorsServer(t)
	_, _, rows := operatorRows(t, s, "?as_of_day=5")
	wantRow(t, rows, "P2", []int{10, 0, 0, 9, 1, 0, 0})
}

func TestOperators_DefaultDay(t *testing.T) {
	s := operatorsServer(t)
	day, _, _ := operatorRows(t, s, "")
	if day != 2 {
		t.Fatalf("day = %v", day)
	}
}

func TestOperators_Errors(t *testing.T) {
	s := operatorsServer(t)
	rec, body := get(t, s, "/api/v1/operators?as_of_day=99")
	if rec.Code != 400 || body["code"] != "bad_request" {
		t.Fatalf("day 99 = %d %v", rec.Code, body)
	}
	rec, body = get(t, serve(t, nil), "/api/v1/operators")
	if rec.Code != 404 || body["code"] != "no_active_model" {
		t.Fatalf("empty = %d %v", rec.Code, body)
	}
}

func TestOperators_BandAtDay(t *testing.T) {
	var ps []fxPlayer
	for i := 1; i <= 10; i++ {
		p := fxPlayer{ID: fmt.Sprintf("S%02d", i), Operator: "P1", LastDay: 5}
		for d := 0; d <= 5; d++ {
			var s *float64
			switch {
			case i < 10:
				s = sc(0.9 - 0.05*float64(i))
			case d >= 4:
				s = sc(0.95)
			}
			p.Scores = append(p.Scores, s)
		}
		ps = append(ps, p)
	}
	s := serve(t, &fixture{Players: ps})
	_, _, rows := operatorRows(t, s, "?as_of_day=3")
	wantRow(t, rows, "P1", []int{10, 1, 1, 7, 1, 0, 0})
	_, _, rows = operatorRows(t, s, "?as_of_day=5")
	wantRow(t, rows, "P1", []int{10, 1, 1, 8, 0, 0, 0})
}
