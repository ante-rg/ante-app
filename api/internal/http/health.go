package http

import (
	"context"
	"net/http"
	"time"

	"github.com/ante-rg/ante-app/api/internal/gen"
)

const healthDBTimeout = 2 * time.Second

func (s *Server) GetHealth(ctx context.Context, _ gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	dbCtx, cancel := context.WithTimeout(ctx, healthDBTimeout)
	defer cancel()
	if err := s.DB.Ping(dbCtx); err != nil {
		db, sc := gen.HealthDatabaseDown, s.scorerState(ctx)
		return gen.GetHealth503JSONResponse{Status: gen.HealthStatusDown, Database: &db, Scorer: &sc, Version: s.Version}, nil
	}
	db, sc, status := gen.HealthDatabaseOk, s.scorerState(ctx), gen.HealthStatusOk
	if sc == gen.HealthScorerDown {
		status = gen.HealthStatusDegraded
	}
	return gen.GetHealth200JSONResponse{Status: status, Database: &db, Scorer: &sc, Version: s.Version}, nil
}

func (s *Server) scorerState(ctx context.Context) gen.HealthScorer {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.ScorerURL+"/healthz", nil)
	if err != nil {
		return gen.HealthScorerDown
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return gen.HealthScorerDown
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return gen.HealthScorerDown
	}
	return gen.HealthScorerOk
}
