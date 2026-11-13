package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ante-rg/ante-app/api/internal/artifacts"
	"github.com/ante-rg/ante-app/api/internal/store"
)

const copyChunk = 10_000

func dbErr(table string, err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch {
		case pe.ConstraintName != "":
			return fmt.Errorf("load %s: %s (%s)", table, pe.ConstraintName, pe.Code)
		case pe.ColumnName != "":
			return fmt.Errorf("load %s: %s (%s)", table, pe.ColumnName, pe.Code)
		}
		return fmt.Errorf("load %s: %s", table, pe.Code)
	}
	return fmt.Errorf("load %s: %w", table, err)
}

func copyTable[T any](ctx context.Context, tx pgx.Tx, table string, cols []string,
	iter func(func(int, T) error) error, toRow func(T) []any,
) (int64, error) {
	var total int64
	buf := make([][]any, 0, copyChunk)
	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		n, err := tx.CopyFrom(ctx, pgx.Identifier{table}, cols, pgx.CopyFromRows(buf))
		total += n
		buf = buf[:0]
		if err != nil {
			return dbErr(table, err)
		}
		return nil
	}
	err := iter(func(_ int, v T) error {
		buf = append(buf, toRow(v))
		if len(buf) == copyChunk {
			return flush()
		}
		return nil
	})
	if err != nil {
		return total, err
	}
	return total, flush()
}

func score32(p *float64) *float32 {
	if p == nil {
		return nil
	}
	v := float32(*p)
	return &v
}

func load(ctx context.Context, pool *pgxpool.Pool, f *artifacts.Folder) (map[string]int64, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, dbErr("transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(tx)
	n := map[string]int64{}

	var ids []string
	seenOp := map[string]bool{}
	err = f.Players(func(_ int, p artifacts.Player) error {
		if !seenOp[p.OperatorCode] {
			seenOp[p.OperatorCode] = true
			if err := q.UpsertOperator(ctx, p.OperatorCode); err != nil {
				return dbErr("operators", err)
			}
		}
		if err := q.UpsertPlayer(ctx, store.UpsertPlayerParams{PlayerID: p.PlayerID, OperatorCode: p.OperatorCode,
			LastDay: p.LastDay, SessionCount: p.SessionCount, DepositCount: p.DepositCount}); err != nil {
			return dbErr("players", err)
		}
		ids = append(ids, p.PlayerID)
		n["players"]++
		return nil
	})
	if err != nil {
		return nil, err
	}

	for table, del := range map[string]func(context.Context, []string) error{
		"daily_activity": q.DeleteActivityForPlayers, "sessions": q.DeleteSessionsForPlayers,
		"rule_flags": q.DeleteRuleFlagsForPlayers,
	} {
		if err := del(ctx, ids); err != nil {
			return nil, dbErr(table, err)
		}
	}
	if n["daily_activity"], err = copyTable(ctx, tx, "daily_activity",
		[]string{"player_id", "day", "deposits_n", "deposits_eur", "sessions_n", "play_minutes",
			"net_result_eur", "chase_sessions_n", "night_sessions_n", "end_balance_eur"},
		f.Activity, func(a artifacts.Activity) []any {
			return []any{a.PlayerID, a.Day, a.DepositsN, a.DepositsEUR, a.SessionsN, a.PlayMinutes,
				a.NetResultEUR, a.ChaseSessionsN, a.NightSessionsN, a.EndBalanceEUR}
		}); err != nil {
		return nil, err
	}
	if n["sessions"], err = copyTable(ctx, tx, "sessions",
		[]string{"player_id", "session_no", "start_day", "duration_s", "deposits_n", "deposits_eur",
			"net_result_eur", "is_night", "is_chase"},
		f.Sessions, func(s artifacts.Session) []any {
			return []any{s.PlayerID, s.SessionNo, s.StartDay, s.DurationS, s.DepositsN, s.DepositsEUR,
				s.NetResultEUR, s.IsNight, s.IsChase}
		}); err != nil {
		return nil, err
	}
	if n["rule_flags"], err = copyTable(ctx, tx, "rule_flags",
		[]string{"player_id", "day", "rule_id", "rule_text"},
		f.RuleFlags, func(r artifacts.RuleFlag) []any { return []any{r.PlayerID, r.Day, r.RuleID, r.RuleText} }); err != nil {
		return nil, err
	}

	if err := q.DeleteModel(ctx, f.ModelVersion); err != nil {
		return nil, dbErr("models", err)
	}
	if err := q.DeactivateModels(ctx); err != nil {
		return nil, dbErr("models", err)
	}
	if err := q.InsertActiveModel(ctx, store.InsertActiveModelParams{ModelVersion: f.ModelVersion, Card: f.Card}); err != nil {
		return nil, dbErr("models", err)
	}

	if n["daily_scores"], err = copyTable(ctx, tx, "daily_scores",
		[]string{"model_version", "player_id", "day", "score", "reasons"},
		f.Scores, func(s artifacts.Score) []any {
			return []any{f.ModelVersion, s.PlayerID, s.Day, score32(s.Score), s.Reasons}
		}); err != nil {
		return nil, err
	}
	if n["review_sample"], err = copyTable(ctx, tx, "review_sample",
		[]string{"model_version", "player_id", "source", "position"},
		f.Sample, func(s artifacts.Sample) []any { return []any{f.ModelVersion, s.PlayerID, s.Source, s.Position} }); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, dbErr("commit", err)
	}
	return n, nil
}
