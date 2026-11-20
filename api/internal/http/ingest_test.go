package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ante-rg/ante-app/api/internal/gen"
	apihttp "github.com/ante-rg/ante-app/api/internal/http"
	"github.com/ante-rg/ante-app/api/internal/ingestkey"
	"github.com/ante-rg/ante-app/api/internal/store"
)

const forbiddenBody = `{"code":"forbidden","message":"The key belongs to another operator"}`

func ingestServer(t *testing.T) *apihttp.Server {
	t.Helper()
	s := serve(t, &fixture{Players: []fxPlayer{{ID: "S01", Operator: "P1"}}})
	if _, err := s.DB.Exec(context.Background(), `INSERT INTO operators (code, name) VALUES ('P3', 'P3')`); err != nil {
		t.Fatal(err)
	}
	return s
}

func ingest(t *testing.T, s *apihttp.Server, code, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/ingest/v1/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	apihttp.NewAPIHandler(s).ServeHTTP(rec, req.WithContext(apihttp.WithOperatorForTest(req.Context(), code)))
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec, m
}

func batch(player string, orders ...int) string {
	evs := make([]string, len(orders))
	for i, o := range orders {
		evs[i] = fmt.Sprintf(`{"event_order":%d,"event_type":"DEPOSIT","seconds_from_first_event":%d}`, o, o*10)
	}
	return fmt.Sprintf(`{"player_id":%q,"events":[%s]}`, player, strings.Join(evs, ","))
}

func countEvents(t *testing.T, s *apihttp.Server, player string) (n int, owners string) {
	t.Helper()
	err := s.DB.QueryRow(context.Background(),
		`SELECT count(*), coalesce(string_agg(DISTINCT operator_code, ','), '') FROM events WHERE player_id = $1`, player).Scan(&n, &owners)
	if err != nil {
		t.Fatal(err)
	}
	return n, owners
}

func TestIngest_Stores(t *testing.T) {
	s := ingestServer(t)
	body := `{"player_id":"X01","events":[
		{"event_order":1,"event_type":"DEPOSIT","seconds_from_first_event":1},
		{"event_order":2,"event_type":"BET_PLACED","seconds_from_first_event":2,"foo":1,"product":null},
		{"event_order":3,"event_type":"BET_RESULT","seconds_from_first_event":3},
		{"event_order":4,"event_type":"BET_PLACED","seconds_from_first_event":4,"total_balance_eur":200000.01,"stake_eur":12.3456789}]}`
	rec, m := ingest(t, s, "P3", body)
	if rec.Code != 503 || m["accepted"] != 4.0 || m["duplicates"] != 0.0 || m["last_event_order"] != 4.0 {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if v, ok := m["score"]; !ok || v != nil {
		t.Fatalf("score must be present and null: %s", rec.Body)
	}
	if n, owners := countEvents(t, s, "X01"); n != 4 || owners != "P3" {
		t.Fatalf("rows = %d under %q", n, owners)
	}
	var raw []byte
	if err := s.DB.QueryRow(context.Background(),
		`SELECT payload FROM events WHERE operator_code='P3' AND player_id='X01' AND event_order=2`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p["foo"] != 1.0 || p["product"] != nil {
		t.Fatalf("payload = %s", raw)
	}
	if err := s.DB.QueryRow(context.Background(),
		`SELECT payload FROM events WHERE operator_code='P3' AND player_id='X01' AND event_order=4`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	p = nil
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p["total_balance_eur"] != 200000.01 || p["stake_eur"] != 12.3456789 {
		t.Fatalf("money rounded: %s", raw)
	}
}

func TestEventMoneyKeepsDouble(t *testing.T) {
	var e gen.Event
	in := `{"event_order":1,"event_type":"DEPOSIT","seconds_from_first_event":0,"total_balance_eur":200000.01}`
	if err := json.Unmarshal([]byte(in), &e); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(e)
	if err != nil || !strings.Contains(string(out), "200000.01") {
		t.Fatalf("got %s, %v", out, err)
	}
}

func TestIngest_NulCharacter(t *testing.T) {
	s := ingestServer(t)
	rec, m := ingest(t, s, "P3", `{"player_id":"X01","events":[{"event_order":1,"event_type":"DEPOSIT","seconds_from_first_event":1,"game_code":"a\u0000b"}]}`)
	if rec.Code != 422 || m["code"] != "invalid" || m["message"] != "events contain a NUL character" {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if n, _ := countEvents(t, s, "X01"); n != 0 {
		t.Fatalf("rows = %d", n)
	}
}

func TestIngest_Idempotent(t *testing.T) {
	s := ingestServer(t)
	ingest(t, s, "P3", batch("X01", 1, 2, 3))
	rec, m := ingest(t, s, "P3", batch("X01", 1, 2, 3))
	if rec.Code != 503 || m["accepted"] != 0.0 || m["duplicates"] != 3.0 || m["last_event_order"] != 3.0 {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestIngest_RepeatInBatch(t *testing.T) {
	s := ingestServer(t)
	rec, m := ingest(t, s, "P3", batch("X01", 1, 1, 2))
	if rec.Code != 503 || m["accepted"] != 2.0 || m["duplicates"] != 1.0 {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestIngest_OtherBrandPlayer(t *testing.T) {
	s := ingestServer(t)
	rec, _ := ingest(t, s, "P3", batch("S01", 1))
	if rec.Code != 403 || strings.TrimSpace(rec.Body.String()) != forbiddenBody {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if n, _ := countEvents(t, s, "S01"); n != 0 {
		t.Fatalf("rows = %d", n)
	}
}

func TestIngest_FirstSenderOwns(t *testing.T) {
	s := ingestServer(t)
	if rec, _ := ingest(t, s, "P1", batch("X02", 1)); rec.Code != 503 {
		t.Fatalf("P1: %d %s", rec.Code, rec.Body)
	}
	if rec, _ := ingest(t, s, "P3", batch("X02", 2)); rec.Code != 403 {
		t.Fatalf("P3: %d %s", rec.Code, rec.Body)
	}
	if n, owners := countEvents(t, s, "X02"); n != 1 || owners != "P1" {
		t.Fatalf("rows = %d under %q", n, owners)
	}
}

func TestIngest_ConcurrentClaim(t *testing.T) {
	s := ingestServer(t)
	start := make(chan struct{})
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i, op := range []string{"P1", "P3"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			rec, _ := ingest(t, s, op, batch("X03", 1))
			codes[i] = rec.Code
		}()
	}
	close(start)
	wg.Wait()
	if codes[0]+codes[1] != 503+403 || codes[0] == codes[1] {
		t.Fatalf("codes = %v, want one 503 and one 403", codes)
	}
	if n, owners := countEvents(t, s, "X03"); n != 1 || strings.Contains(owners, ",") {
		t.Fatalf("rows = %d under %q", n, owners)
	}
}

func TestIngest_Invalid(t *testing.T) {
	s := ingestServer(t)
	ev := func(f string) string {
		return `{"player_id":"X01","events":[{"event_order":1,"event_type":"DEPOSIT","seconds_from_first_event":0` + f + `}]}`
	}
	many := make([]string, 5001)
	for i := range many {
		many[i] = fmt.Sprintf(`{"event_order":%d,"event_type":"DEPOSIT","seconds_from_first_event":0}`, i+1)
	}
	cases := map[string]string{
		"event_type":   strings.Replace(ev(""), `"DEPOSIT"`, `"NOPE"`, 1),
		"event_order":  strings.Replace(ev(""), `"event_order":1`, `"event_order":0`, 1),
		"seconds":      strings.Replace(ev(""), `"seconds_from_first_event":0`, `"seconds_from_first_event":-1`, 1),
		"local_hour":   ev(`,"local_hour":24`),
		"weekday":      ev(`,"weekday":0`),
		"no seconds":   strings.Replace(ev(""), `,"seconds_from_first_event":0`, "", 1),
		"product":      ev(`,"product":"POKER"`),
		"too many":     `{"player_id":"X01","events":[` + strings.Join(many, ",") + `]}`,
		"player_id":    strings.Replace(ev(""), `"X01"`, `"bad id"`, 1),
		"empty events": `{"player_id":"X01","events":[]}`,
	}
	for name, body := range cases {
		rec, m := ingest(t, s, "P3", body)
		if rec.Code != 422 || m["code"] != "invalid" {
			t.Errorf("%s: got %d %s", name, rec.Code, rec.Body)
		}
	}
	if n, _ := countEvents(t, s, "X01"); n != 0 {
		t.Fatalf("rows = %d", n)
	}
}

func TestIngest_RotatedKeyRejected(t *testing.T) {
	s := ingestServer(t)
	ctx := context.Background()
	q := store.New(s.DB)
	for _, k := range []string{"k1", "k2"} {
		if _, err := q.SetOperatorKeyHash(ctx, store.SetOperatorKeyHashParams{Hash: pgtype.Text{String: ingestkey.Hash(k), Valid: true}, Code: "P3"}); err != nil {
			t.Fatal(err)
		}
	}
	h := apihttp.NewRouter(apihttp.NewAPIHandler(s), t.TempDir(), apihttp.Auth{DevUser: "dev@localhost"}, noAudit{}, q)
	post := func(key string) int {
		req := httptest.NewRequest(http.MethodPost, "/ingest/v1/events", strings.NewReader(batch("X01", 1)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := post("k1"); c != 401 {
		t.Fatalf("old key: %d", c)
	}
	if c := post("k2"); c != 503 {
		t.Fatalf("new key: %d", c)
	}
}
