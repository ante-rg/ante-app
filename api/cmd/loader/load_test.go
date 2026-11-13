package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/parquet-go/parquet-go"

	"github.com/ante-rg/ante-app/api/internal/artifacts"
	"github.com/ante-rg/ante-app/api/internal/db"
	"github.com/ante-rg/ante-app/api/internal/fakeartifacts"
	"github.com/ante-rg/ante-app/api/internal/testdb"
)

var tables = []string{"operators", "players", "models", "daily_scores", "daily_activity", "sessions", "rule_flags", "review_sample"}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Open(ctx, testdb.Start(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func folder(t *testing.T, version string) (*artifacts.Folder, fakeartifacts.Dataset) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), version)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ds := fakeartifacts.Generate(1)
	if err := fakeartifacts.Write(dir, version, ds); err != nil {
		t.Fatal(err)
	}
	f, err := artifacts.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return f, ds
}

func counts(t *testing.T, pool *pgxpool.Pool) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, tb := range tables {
		var n int64
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+tb).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[tb] = n
	}
	return out
}

func activeVersions(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT model_version FROM models WHERE is_active")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var vs []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		vs = append(vs, v)
	}
	return vs
}

func mustLoad(t *testing.T, pool *pgxpool.Pool, f *artifacts.Folder) map[string]int64 {
	t.Helper()
	n, err := load(context.Background(), pool, f)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLoad_DummyV0(t *testing.T) {
	pool := newPool(t)
	f, ds := folder(t, "dummy-v0")
	n := mustLoad(t, pool, f)
	want := map[string]int64{
		"players": int64(len(ds.Players)), "daily_scores": int64(len(ds.Scores)),
		"daily_activity": int64(len(ds.Activity)), "sessions": int64(len(ds.Sessions)),
		"rule_flags": int64(len(ds.RuleFlags)), "review_sample": int64(len(ds.Sample)),
		"operators": 5, "models": 1,
	}
	got := counts(t, pool)
	for tb, w := range want {
		if got[tb] != w {
			t.Errorf("%s: %d rows, want %d", tb, got[tb], w)
		}
		if c, ok := n[tb]; ok && c != w {
			t.Errorf("returned %s: %d, want %d", tb, c, w)
		}
	}
	if a := activeVersions(t, pool); len(a) != 1 || a[0] != "dummy-v0" {
		t.Errorf("active = %v", a)
	}
	var bad int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM operators WHERE name <> code OR code NOT IN ('P1','P2','P3','P4','P5')`).Scan(&bad); err != nil || bad != 0 {
		t.Errorf("operators: bad=%d err=%v", bad, err)
	}
}

func TestLoad_Idempotent(t *testing.T) {
	pool := newPool(t)
	f, _ := folder(t, "dummy-v0")
	mustLoad(t, pool, f)
	first := counts(t, pool)
	mustLoad(t, pool, f)
	second := counts(t, pool)
	for _, tb := range tables {
		if first[tb] != second[tb] {
			t.Errorf("%s: %d then %d", tb, first[tb], second[tb])
		}
	}
	if a := activeVersions(t, pool); len(a) != 1 || a[0] != "dummy-v0" {
		t.Errorf("active = %v", a)
	}
}

func TestLoad_SecondVersion(t *testing.T) {
	pool := newPool(t)
	f0, _ := folder(t, "dummy-v0")
	f1, _ := folder(t, "dummy-v1")
	mustLoad(t, pool, f0)
	mustLoad(t, pool, f1)
	if a := activeVersions(t, pool); len(a) != 1 || a[0] != "dummy-v1" {
		t.Fatalf("active = %v", a)
	}
	var n0, n1 int64
	q := `SELECT count(*) FROM daily_scores WHERE model_version = $1`
	if err := pool.QueryRow(context.Background(), q, "dummy-v0").Scan(&n0); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), q, "dummy-v1").Scan(&n1); err != nil {
		t.Fatal(err)
	}
	if n0 == 0 || n0 != n1 {
		t.Errorf("dummy-v0 scores %d, dummy-v1 scores %d", n0, n1)
	}
}

type sampleRow struct {
	PlayerID string `parquet:"player_id"`
	Source   string `parquet:"source"`
	Position int32  `parquet:"position"`
}

func TestLoad_FailureRollsBack(t *testing.T) {
	pool := newPool(t)
	f, ds := folder(t, "dummy-v0")
	mustLoad(t, pool, f)
	before := counts(t, pool)

	rows := make([]sampleRow, len(ds.Sample))
	for i, r := range ds.Sample {
		rows[i] = sampleRow{r.PlayerID, r.Source, r.Position}
	}
	rows[1].Position = rows[0].Position
	fh, err := os.Create(filepath.Join(f.Dir, artifacts.FileSample))
	if err != nil {
		t.Fatal(err)
	}
	w := parquet.NewGenericWriter[sampleRow](fh)
	if _, err := w.Write(rows); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := load(context.Background(), pool, f); err == nil {
		t.Fatal("want error")
	}
	after := counts(t, pool)
	for _, tb := range tables {
		if before[tb] != after[tb] {
			t.Errorf("%s: %d before, %d after", tb, before[tb], after[tb])
		}
	}
	if a := activeVersions(t, pool); len(a) != 1 || a[0] != "dummy-v0" {
		t.Errorf("active = %v", a)
	}
}
