package http_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ante-rg/ante-app/api/internal/db"
	apihttp "github.com/ante-rg/ante-app/api/internal/http"
	"github.com/ante-rg/ante-app/api/internal/store"
	"github.com/ante-rg/ante-app/api/internal/testdb"
)

type fxActivity struct {
	Day                                    int32
	DepositsN, SessionsN                   int32
	ChaseSessionsN, NightSessionsN         int32
	DepositsEUR, PlayMinutes, NetResultEUR float64
	EndBalanceEUR                          *float64
}

type fxSession struct {
	SessionNo, StartDay, DurationS, DepositsN int32
	DepositsEUR, NetResultEUR                 float64
	IsNight, IsChase                          bool
}

type fxPlayer struct {
	ID, Operator string
	LastDay      int32
	Scores       []*float64
	Reasons      map[int32]string
	Activity     []fxActivity
	Sessions     []fxSession
	Flags        []int32
	OtherFlags   []fxFlag
}

type fxFlag struct {
	Day          int32
	RuleID, Text string
}

type fixture struct {
	Version   string
	Card      string
	Players   []fxPlayer
	SkipRanks bool
}

func sc(v float64) *float64 { return &v }

const defaultCard = `{"model_version":"test-v0","default_day":2,"default_capacity":10}`

func serve(t *testing.T, f *fixture) *apihttp.Server {
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
	if f != nil {
		seed(t, ctx, pool, f)
		if !f.SkipRanks {
			if _, err := store.New(pool).FillDailyRanks(ctx, version(f)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return &apihttp.Server{DB: pool, Client: &http.Client{Timeout: time.Second}, Version: "test-sha"}
}

func version(f *fixture) string {
	if f.Version == "" {
		return "test-v0"
	}
	return f.Version
}

func seed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, f *fixture) {
	t.Helper()
	card := f.Card
	if card == "" {
		card = defaultCard
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql, err)
		}
	}
	v := version(f)
	exec(`INSERT INTO models (model_version, card, is_active) VALUES ($1, $2::jsonb, true)`, v, card)
	for _, p := range f.Players {
		exec(`INSERT INTO operators (code, name) VALUES ($1, $1) ON CONFLICT DO NOTHING`, p.Operator)
		exec(`INSERT INTO players (player_id, operator_code, last_day, session_count, deposit_count)
		      VALUES ($1, $2, $3, $4, 0)`, p.ID, p.Operator, p.LastDay, len(p.Sessions))
		for day, score := range p.Scores {
			reasons := "[]"
			if r, ok := p.Reasons[int32(day)]; ok {
				reasons = r
			}
			exec(`INSERT INTO daily_scores (model_version, player_id, day, score, reasons)
			      VALUES ($1, $2, $3, $4, $5::jsonb)`, v, p.ID, day, score, reasons)
		}
		for _, a := range p.Activity {
			exec(`INSERT INTO daily_activity (player_id, day, deposits_n, deposits_eur, sessions_n, play_minutes,
			        net_result_eur, chase_sessions_n, night_sessions_n, end_balance_eur)
			      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				p.ID, a.Day, a.DepositsN, a.DepositsEUR, a.SessionsN, a.PlayMinutes, a.NetResultEUR,
				a.ChaseSessionsN, a.NightSessionsN, a.EndBalanceEUR)
		}
		for _, s := range p.Sessions {
			exec(`INSERT INTO sessions (player_id, session_no, start_day, duration_s, deposits_n, deposits_eur,
			        net_result_eur, is_night, is_chase)
			      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				p.ID, s.SessionNo, s.StartDay, s.DurationS, s.DepositsN, s.DepositsEUR, s.NetResultEUR, s.IsNight, s.IsChase)
		}
		for _, day := range p.Flags {
			exec(`INSERT INTO rule_flags (player_id, day, rule_id, rule_text)
			      VALUES ($1, $2, 'R_DEP_2000_30D', 'Deposits over 2,000 € in 30 days')`, p.ID, day)
		}
		for _, f := range p.OtherFlags {
			exec(`INSERT INTO rule_flags (player_id, day, rule_id, rule_text) VALUES ($1, $2, $3, $4)`,
				p.ID, f.Day, f.RuleID, f.Text)
		}
	}
}
