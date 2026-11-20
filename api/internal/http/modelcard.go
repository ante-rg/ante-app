package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/ante-rg/ante-app/api/internal/gen"
	"github.com/ante-rg/ante-app/api/internal/store"
)

func noModelCard() gen.Error {
	return gen.Error{Code: "no_model_card", Message: "The active model has no card in the current format. Load its artifacts again."}
}

func (s *Server) GetModelCard(ctx context.Context, _ gen.GetModelCardRequestObject) (gen.GetModelCardResponseObject, error) {
	row, err := store.New(s.DB).GetActiveModel(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.GetModelCard404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noActiveModel())}, nil
	}
	if err != nil {
		return nil, err
	}
	var c gen.ModelCard
	if err := json.Unmarshal(row.Card, &c); err != nil {
		slog.Warn("model card does not decode", "err", err, "model_version", row.ModelVersion)
		return gen.GetModelCard404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noModelCard())}, nil
	}
	if c.Summary == "" {
		return gen.GetModelCard404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noModelCard())}, nil
	}
	if c.Limits == nil {
		c.Limits = []string{}
	}
	if c.Metrics == nil {
		c.Metrics = []gen.ModelMetric{}
	}
	return gen.GetModelCard200JSONResponse(c), nil
}
