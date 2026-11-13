package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ante-rg/ante-app/api/internal/store"
)

type rankRow struct {
	Day    int
	Player string
	Score  *float32
	Rank   *int
	Band   string
}

func seedRanks(t *testing.T, pool *pgxpool.Pool, version string, scores map[string][]*float64) {
	t.Helper()
	ctx := context.Background()
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	mustExec(`INSERT INTO operators (code, name) VALUES ('P1', 'P1') ON CONFLICT DO NOTHING`)
	mustExec(`INSERT INTO models (model_version, card) VALUES ($1, '{}')`, version)
	for id, ss := range scores {
		mustExec(`INSERT INTO players (player_id, operator_code, last_day, session_count, deposit_count)
			VALUES ($1, 'P1', $2, 0, 0) ON CONFLICT DO NOTHING`, id, len(ss)-1)
		for day, s := range ss {
			mustExec(`INSERT INTO daily_scores (model_version, player_id, day, score) VALUES ($1, $2, $3, $4)`,
				version, id, day, s)
		}
	}
}

func fill(t *testing.T, pool *pgxpool.Pool, version string) {
	t.Helper()
	if _, err := store.New(pool).FillDailyRanks(context.Background(), version); err != nil {
		t.Fatal(err)
	}
}

func ranksOf(t *testing.T, pool *pgxpool.Pool, version string, day int) map[string]rankRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT day, player_id, score, rank, band FROM daily_ranks WHERE model_version = $1 AND day = $2`, version, day)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]rankRow{}
	for rows.Next() {
		var r rankRow
		if err := rows.Scan(&r.Day, &r.Player, &r.Score, &r.Rank, &r.Band); err != nil {
			t.Fatal(err)
		}
		out[r.Player] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func f64(v float64) *float64 { return &v }

func TestFillDailyRanks_Bands(t *testing.T) {
	pool := migrated(t)
	scores := map[string][]*float64{}
	for i := 1; i <= 20; i++ {
		scores[fmt.Sprintf("S%02d", i)] = []*float64{f64(float64(21-i) / 100)}
	}
	seedRanks(t, pool, "m1", scores)
	fill(t, pool, "m1")

	got := ranksOf(t, pool, "m1", 0)
	if len(got) != 20 {
		t.Fatalf("rows = %d, want 20", len(got))
	}
	bands := map[string]int{}
	for i := 1; i <= 20; i++ {
		r := got[fmt.Sprintf("S%02d", i)]
		if r.Rank == nil || *r.Rank != i {
			t.Errorf("S%02d rank = %v, want %d", i, r.Rank, i)
		}
		bands[r.Band]++
	}
	if bands["high"] != 2 || bands["elevated"] != 2 || bands["low"] != 16 {
		t.Errorf("bands = %v, want 2/2/16", bands)
	}
}

func TestFillDailyRanks_NullScore(t *testing.T) {
	pool := migrated(t)
	scores := map[string][]*float64{"S00": {nil}}
	for i := 1; i <= 10; i++ {
		scores[fmt.Sprintf("S%02d", i)] = []*float64{f64(float64(11-i) / 100)}
	}
	seedRanks(t, pool, "m1", scores)
	fill(t, pool, "m1")

	got := ranksOf(t, pool, "m1", 0)
	if r := got["S00"]; r.Rank != nil || r.Score != nil || r.Band != "insufficient_data" {
		t.Errorf("null row = %+v", r)
	}
	bands := map[string]int{}
	for i := 1; i <= 10; i++ {
		r := got[fmt.Sprintf("S%02d", i)]
		if r.Rank == nil || *r.Rank != i {
			t.Errorf("S%02d rank = %v, want %d", i, r.Rank, i)
		}
		bands[r.Band]++
	}
	if bands["high"] != 1 || bands["elevated"] != 1 || bands["low"] != 8 {
		t.Errorf("bands = %v, want 1/1/8", bands)
	}
}

func TestFillDailyRanks_TieBreak(t *testing.T) {
	pool := migrated(t)
	seedRanks(t, pool, "m1", map[string][]*float64{
		"S2": {f64(0.5)}, "S1": {f64(0.5)}, "S3": {f64(0.1)},
	})
	fill(t, pool, "m1")

	got := ranksOf(t, pool, "m1", 0)
	for id, want := range map[string]int{"S1": 1, "S2": 2, "S3": 3} {
		if r := got[id]; r.Rank == nil || *r.Rank != want {
			t.Errorf("%s rank = %v, want %d", id, r.Rank, want)
		}
	}
}

func TestFillDailyRanks_EffectiveDay(t *testing.T) {
	pool := migrated(t)
	seedRanks(t, pool, "m1", map[string][]*float64{
		"A": {f64(0.1), f64(0.9)},
		"B": {f64(0.2), f64(0.3), f64(0.5), f64(0.95)},
	})
	fill(t, pool, "m1")

	for day := 0; day <= 3; day++ {
		if n := len(ranksOf(t, pool, "m1", day)); n != 2 {
			t.Errorf("day %d: %d rows, want 2", day, n)
		}
	}
	if n := len(ranksOf(t, pool, "m1", 4)); n != 0 {
		t.Errorf("day 4: %d rows, want 0", n)
	}
	got := ranksOf(t, pool, "m1", 3)
	if a := got["A"]; a.Score == nil || *a.Score != float32(0.9) || a.Rank == nil || *a.Rank != 2 {
		t.Errorf("A on day 3 = %+v, want score 0.9 rank 2", a)
	}
	if b := got["B"]; b.Rank == nil || *b.Rank != 1 {
		t.Errorf("B on day 3 rank = %v, want 1", b.Rank)
	}
}

func TestFillDailyRanks_OtherVersionUntouched(t *testing.T) {
	pool := migrated(t)
	ctx := context.Background()
	seedRanks(t, pool, "m1", map[string][]*float64{"S1": {f64(0.3)}, "S2": {f64(0.4)}})
	if _, err := pool.Exec(ctx, `INSERT INTO models (model_version, card) VALUES ('m2', '{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO daily_scores (model_version, player_id, day, score) VALUES ('m2', 'S1', 0, 0.7)`); err != nil {
		t.Fatal(err)
	}
	fill(t, pool, "m1")
	if n := len(ranksOf(t, pool, "m2", 0)); n != 0 {
		t.Fatalf("m2 rows = %d, want 0", n)
	}
	if n := len(ranksOf(t, pool, "m1", 0)); n != 2 {
		t.Fatalf("m1 rows = %d, want 2", n)
	}
	fill(t, pool, "m2")
	if got := ranksOf(t, pool, "m2", 0); len(got) != 1 || got["S1"].Score == nil || *got["S1"].Score != float32(0.7) {
		t.Fatalf("m2 = %+v", got)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM models WHERE model_version = 'm1'`); err != nil {
		t.Fatal(err)
	}
	if n := len(ranksOf(t, pool, "m1", 0)); n != 0 {
		t.Errorf("m1 rows after model delete = %d, want 0", n)
	}
	if n := len(ranksOf(t, pool, "m2", 0)); n != 1 {
		t.Errorf("m2 rows after m1 delete = %d, want 1", n)
	}
}
