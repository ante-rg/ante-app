package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

type playerRow struct {
	PlayerID     string `parquet:"player_id"`
	OperatorCode string `parquet:"operator_code"`
	LastDay      int32  `parquet:"last_day"`
	SessionCount int32  `parquet:"session_count"`
	DepositCount int32  `parquet:"deposit_count"`
}

type scoreRow struct {
	PlayerID string   `parquet:"player_id"`
	Day      int32    `parquet:"day"`
	Score    *float64 `parquet:"score,optional"`
	Reasons  string   `parquet:"reasons"`
}

type activityRow struct {
	PlayerID       string  `parquet:"player_id"`
	Day            int32   `parquet:"day"`
	DepositsN      int32   `parquet:"deposits_n"`
	DepositsEUR    float64 `parquet:"deposits_eur"`
	SessionsN      int32   `parquet:"sessions_n"`
	PlayMinutes    float64 `parquet:"play_minutes"`
	NetResultEUR   float64 `parquet:"net_result_eur"`
	ChaseSessionsN int32   `parquet:"chase_sessions_n"`
	NightSessionsN int32   `parquet:"night_sessions_n"`
	EndBalanceEUR  float64 `parquet:"end_balance_eur"`
}

type sessionRow struct {
	PlayerID     string  `parquet:"player_id"`
	SessionNo    int32   `parquet:"session_no"`
	StartDay     int32   `parquet:"start_day"`
	DurationS    int32   `parquet:"duration_s"`
	DepositsN    int32   `parquet:"deposits_n"`
	DepositsEUR  float64 `parquet:"deposits_eur"`
	NetResultEUR float64 `parquet:"net_result_eur"`
	IsNight      bool    `parquet:"is_night"`
	IsChase      bool    `parquet:"is_chase"`
}

type flagRow struct {
	PlayerID string `parquet:"player_id"`
	Day      int32  `parquet:"day"`
	RuleID   string `parquet:"rule_id"`
	RuleText string `parquet:"rule_text"`
}

type sampleRow struct {
	PlayerID string `parquet:"player_id"`
	Source   string `parquet:"source"`
	Position int32  `parquet:"position"`
}

type cardSections struct {
	Summary     string `json:"summary"`
	IntendedUse string `json:"intended_use"`
	Limitations string `json:"limitations"`
}

type modelCard struct {
	ModelVersion    string         `json:"model_version"`
	Synthetic       bool           `json:"synthetic"`
	DefaultDay      int32          `json:"default_day"`
	DefaultCapacity int            `json:"default_capacity"`
	Sections        cardSections   `json:"sections"`
	Metrics         map[string]any `json:"metrics"`
	KnownLimits     []string       `json:"known_limits"`
}

func Write(dir, version string, ds Dataset) error {
	if err := writeAll(dir, ds); err != nil {
		return err
	}
	return writeCard(dir, version, ds.DefaultDay)
}

func writeAll(dir string, ds Dataset) error {
	players := make([]playerRow, len(ds.Players))
	for i, p := range ds.Players {
		players[i] = playerRow{p.ID, p.Operator, p.LastDay, p.SessionCount, p.DepositCount}
	}
	scores := make([]scoreRow, len(ds.Scores))
	for i, s := range ds.Scores {
		reasons := []byte("[]")
		if len(s.Reasons) > 0 {
			var err error
			if reasons, err = json.Marshal(s.Reasons); err != nil {
				return fmt.Errorf("encode reasons: %w", err)
			}
		}
		scores[i] = scoreRow{s.PlayerID, s.Day, s.Score, string(reasons)}
	}
	act := make([]activityRow, len(ds.Activity))
	for i, a := range ds.Activity {
		act[i] = activityRow{a.PlayerID, a.Day, a.DepositsN, a.DepositsEUR, a.SessionsN, a.PlayMinutes,
			a.NetResultEUR, a.ChaseSessionsN, a.NightSessionsN, a.EndBalanceEUR}
	}
	sess := make([]sessionRow, len(ds.Sessions))
	for i, s := range ds.Sessions {
		sess[i] = sessionRow{s.PlayerID, s.No, s.StartDay, s.DurationS, s.DepositsN, s.DepositsEUR,
			s.NetResultEUR, s.IsNight, s.IsChase}
	}
	flags := make([]flagRow, len(ds.RuleFlags))
	for i, f := range ds.RuleFlags {
		flags[i] = flagRow(f)
	}
	sample := make([]sampleRow, len(ds.Sample))
	for i, s := range ds.Sample {
		sample[i] = sampleRow(s)
	}
	if err := writeParquet(dir, "players", players); err != nil {
		return err
	}
	if err := writeParquet(dir, "daily_scores", scores); err != nil {
		return err
	}
	if err := writeParquet(dir, "daily_activity", act); err != nil {
		return err
	}
	if err := writeParquet(dir, "sessions", sess); err != nil {
		return err
	}
	if err := writeParquet(dir, "rule_flags", flags); err != nil {
		return err
	}
	return writeParquet(dir, "review_sample", sample)
}

func writeParquet[T any](dir, name string, rows []T) (err error) {
	f, err := os.Create(filepath.Join(dir, name+".parquet"))
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("write %s: %w", name, cerr)
		}
	}()
	w := parquet.NewGenericWriter[T](f)
	if _, err = w.Write(rows); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func writeCard(dir, version string, defaultDay int32) error {
	card := modelCard{
		ModelVersion:    version,
		Synthetic:       true,
		DefaultDay:      defaultDay,
		DefaultCapacity: sampleListSize,
		Sections: cardSections{
			Summary:     "Synthetic data for development. Not a model; scores are normalised 30-day deposits.",
			IntendedUse: "Building and demonstrating the app before the data science team publishes artifacts.",
			Limitations: "No real players. Numbers carry no meaning about gambling harm.",
		},
		Metrics:     map[string]any{},
		KnownLimits: []string{"Synthetic data only"},
	}
	b, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		return fmt.Errorf("write model_card.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "model_card.json"), append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write model_card.json: %w", err)
	}
	return nil
}
