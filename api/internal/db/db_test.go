package db

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/ante-rg/ante-app/api/internal/store"
	"github.com/ante-rg/ante-app/api/internal/testdb"
	"github.com/ante-rg/ante-app/api/migrations"
)

func migrated(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := Open(ctx, testdb.Start(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

func seed(t *testing.T, tx pgx.Tx) {
	t.Helper()
	stmts := []string{
		`INSERT INTO operators (code, name) VALUES ('P1', 'P1')`,
		`INSERT INTO players (player_id, operator_code, last_day, session_count, deposit_count)
		 VALUES ('S001', 'P1', 2, 1, 1)`,
		`INSERT INTO models (model_version, card, is_active) VALUES ('test-v0', '{}', true)`,
		`INSERT INTO daily_scores (model_version, player_id, day, score) VALUES
		 ('test-v0', 'S001', 0, NULL), ('test-v0', 'S001', 1, 0.2), ('test-v0', 'S001', 2, 0.9)`,
		`INSERT INTO daily_activity (player_id, day, deposits_n, deposits_eur, sessions_n, play_minutes,
		 net_result_eur, chase_sessions_n, night_sessions_n, end_balance_eur)
		 VALUES ('S001', 1, 1, 50, 1, 30, -20, 0, 0, 10)`,
		`INSERT INTO sessions (player_id, session_no, start_day, duration_s, deposits_n, deposits_eur,
		 net_result_eur, is_night, is_chase) VALUES ('S001', 1, 1, 1800, 1, 50, -20, false, false)`,
		`INSERT INTO rule_flags (player_id, day, rule_id, rule_text) VALUES ('S001', 2, 'R1', 'test rule')`,
		`INSERT INTO review_sample (model_version, player_id, source, position)
		 VALUES ('test-v0', 'S001', 'model', 1)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(context.Background(), s); err != nil {
			t.Fatalf("seed %q: %v", s, err)
		}
	}
}

func TestMigrate_FreshDatabase(t *testing.T) {
	ctx := context.Background()
	pool, err := Open(ctx, testdb.Start(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, pool); err != nil {
			t.Fatalf("migrate call %d: %v", i+1, err)
		}
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = sqlDB.Close() })
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}

func TestSchema(t *testing.T) {
	ctx := context.Background()
	pool := migrated(t)

	inTx := func(t *testing.T, f func(tx pgx.Tx)) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		seed(t, tx)
		f(tx)
	}
	count := func(t *testing.T, tx pgx.Tx, q string) int {
		t.Helper()
		var n int
		if err := tx.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}

	t.Run("tables", func(t *testing.T) {
		rows, err := pool.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"operators", "players", "models", "daily_scores", "daily_activity", "sessions",
			"rule_flags", "review_sample", "audit_log", "goose_db_version"}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("tables = %v, want %v", got, want)
		}
	})

	t.Run("null_score_ok", func(t *testing.T) {
		inTx(t, func(tx pgx.Tx) {
			if _, err := tx.Exec(ctx,
				`INSERT INTO daily_scores (model_version, player_id, day, score, reasons)
				 VALUES ('test-v0', 'S001', 3, NULL, '[]')`); err != nil {
				t.Fatal(err)
			}
		})
	})

	t.Run("rejects", func(t *testing.T) {
		cases := []struct{ name, sql, sqlstate string }{
			{"second active model", `INSERT INTO models (model_version, card, is_active) VALUES ('test-v1', '{}', true)`, "23505"},
			{"score above 1", `INSERT INTO daily_scores (model_version, player_id, day, score) VALUES ('test-v0', 'S001', 3, 1.5)`, "23514"},
			{"negative day", `INSERT INTO daily_scores (model_version, player_id, day, score) VALUES ('test-v0', 'S001', -1, 0.5)`, "23514"},
			{"bad source", `WITH p AS (
				INSERT INTO players (player_id, operator_code, last_day, session_count, deposit_count)
				VALUES ('S002', 'P1', 0, 0, 0) RETURNING player_id)
				INSERT INTO review_sample (model_version, player_id, source, position)
				SELECT 'test-v0', player_id, 'other', 2 FROM p`, "23514"},
			{"duplicate position", `WITH p AS (
				INSERT INTO players (player_id, operator_code, last_day, session_count, deposit_count)
				VALUES ('S002', 'P1', 0, 0, 0) RETURNING player_id)
				INSERT INTO review_sample (model_version, player_id, source, position)
				SELECT 'test-v0', player_id, 'rules', 1 FROM p`, "23505"},
			{"unknown operator", `INSERT INTO players (player_id, operator_code, last_day, session_count, deposit_count) VALUES ('S003', 'P9', 0, 0, 0)`, "23503"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				inTx(t, func(tx pgx.Tx) {
					sp, err := tx.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					_, err = sp.Exec(ctx, c.sql)
					_ = sp.Rollback(ctx)
					var pgErr *pgconn.PgError
					if !errors.As(err, &pgErr) {
						t.Fatalf("err = %v, want PgError %s", err, c.sqlstate)
					}
					if pgErr.Code != c.sqlstate {
						t.Fatalf("sqlstate = %s (%v), want %s", pgErr.Code, pgErr, c.sqlstate)
					}
				})
			})
		}
	})

	t.Run("cascade_model", func(t *testing.T) {
		inTx(t, func(tx pgx.Tx) {
			if _, err := tx.Exec(ctx, `DELETE FROM models WHERE model_version = 'test-v0'`); err != nil {
				t.Fatal(err)
			}
			for _, tbl := range []string{"daily_scores", "review_sample"} {
				if n := count(t, tx, `SELECT count(*) FROM `+tbl); n != 0 {
					t.Errorf("%s has %d rows, want 0", tbl, n)
				}
			}
		})
	})

	t.Run("cascade_player", func(t *testing.T) {
		inTx(t, func(tx pgx.Tx) {
			if _, err := tx.Exec(ctx, `DELETE FROM players WHERE player_id = 'S001'`); err != nil {
				t.Fatal(err)
			}
			for _, tbl := range []string{"daily_scores", "daily_activity", "sessions", "rule_flags", "review_sample"} {
				if n := count(t, tx, `SELECT count(*) FROM `+tbl+` WHERE player_id = 'S001'`); n != 0 {
					t.Errorf("%s has %d rows, want 0", tbl, n)
				}
			}
		})
	})

	t.Run("get_active_model", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err := store.New(tx).GetActiveModel(ctx); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("no models: want pgx.ErrNoRows, got %v", err)
		}

		seed(t, tx)
		m, err := store.New(tx).GetActiveModel(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if m.ModelVersion != "test-v0" || string(m.Card) != "{}" {
			t.Fatalf("got %q card %q", m.ModelVersion, m.Card)
		}
	})
}
