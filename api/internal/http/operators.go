package http

import (
	"context"
	"errors"

	"github.com/ante-rg/ante-app/api/internal/gen"
	"github.com/ante-rg/ante-app/api/internal/store"
)

func (s *Server) ListOperators(ctx context.Context, req gen.ListOperatorsRequestObject) (gen.ListOperatorsResponseObject, error) {
	m, err := s.loadActive(ctx)
	if errors.Is(err, errNoActiveModel) {
		return gen.ListOperators404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noActiveModel())}, nil
	}
	if errors.Is(err, errNoRanks) {
		return gen.ListOperators404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noRanks())}, nil
	}
	if err != nil {
		return nil, err
	}
	day, err := m.dayFor(req.Params.AsOfDay)
	if err != nil {
		return gen.ListOperators400JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "bad_request", Message: err.Error()})}, nil
	}
	rows, err := store.New(s.DB).ListOperatorSummaries(ctx, store.ListOperatorSummariesParams{ModelVersion: m.Version, Day: day})
	if err != nil {
		return nil, err
	}
	items := make([]gen.OperatorSummary, len(rows))
	for i, r := range rows {
		items[i] = gen.OperatorSummary{
			OperatorCode: r.OperatorCode, Players: int(r.Players), High: int(r.High), Elevated: int(r.Elevated),
			Low: int(r.Low), InsufficientData: int(r.InsufficientData),
			FlaggedByRulesToo: int(r.FlaggedByRulesToo), Reviewed: int(r.Reviewed),
		}
	}
	return gen.ListOperators200JSONResponse{AsOfDay: int(day), Items: items}, nil
}
