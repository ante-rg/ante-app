package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/ante-rg/ante-app/api/internal/gen"
	"github.com/ante-rg/ante-app/api/internal/store"
)

const (
	defaultCapacity = 10
	maxCapacity     = 1000
)

var (
	errNoActiveModel = errors.New("no active model")
	errNoRanks       = errors.New("no ranks for the active model")
)

type card struct {
	DefaultDay        *int32
	DefaultCapacity   int32
	DatasetVersion    *string
	FeatureSetVersion *string
}

type activeModel struct {
	Version string
	Card    card
	MaxDay  int32
}

func parseCard(raw []byte) (card, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return card{}, err
	}
	c := card{DefaultCapacity: defaultCapacity}
	if f, ok := m["default_day"].(float64); ok && math.Abs(f) < 1e9 {
		d := int32(f)
		c.DefaultDay = &d
	}
	if f, ok := m["default_capacity"].(float64); ok && f >= 1 && f <= maxCapacity {
		c.DefaultCapacity = int32(f)
	}
	if v, ok := m["dataset_version"].(string); ok {
		c.DatasetVersion = &v
	}
	if v, ok := m["feature_set_version"].(string); ok {
		c.FeatureSetVersion = &v
	}
	return c, nil
}

func (s *Server) loadActive(ctx context.Context) (activeModel, error) {
	q := store.New(s.DB)
	row, err := q.GetActiveModel(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return activeModel{}, errNoActiveModel
	}
	if err != nil {
		return activeModel{}, err
	}
	c, err := parseCard(row.Card)
	if err != nil {
		return activeModel{}, fmt.Errorf("parse model card: %w", err)
	}
	r, err := q.GetMaxRankDay(ctx, row.ModelVersion)
	if err != nil {
		return activeModel{}, err
	}
	if r.Ranks == 0 {
		return activeModel{}, errNoRanks
	}
	return activeModel{Version: row.ModelVersion, Card: c, MaxDay: r.MaxDay}, nil
}

func (m activeModel) defaultDay() int32 {
	if m.Card.DefaultDay == nil {
		return m.MaxDay
	}
	return min(max(*m.Card.DefaultDay, 0), m.MaxDay)
}

func (m activeModel) dayFor(asOf *int) (int32, error) {
	if asOf == nil {
		return m.defaultDay(), nil
	}
	if *asOf < 0 || *asOf > int(m.MaxDay) {
		return 0, fmt.Errorf("as_of_day must be between 0 and %d", m.MaxDay)
	}
	return int32(*asOf), nil
}

func (m activeModel) meta(capacity int32) gen.Meta {
	return gen.Meta{
		ModelVersion:      m.Version,
		DatasetVersion:    m.Card.DatasetVersion,
		FeatureSetVersion: m.Card.FeatureSetVersion,
		Capacity:          int(capacity),
	}
}

func noActiveModel() gen.Error {
	return gen.Error{Code: "no_active_model", Message: "No model is active"}
}

func noRanks() gen.Error {
	return gen.Error{Code: "no_ranks", Message: "Ranks missing for the active model: re-run the loader"}
}
