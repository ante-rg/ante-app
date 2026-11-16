package http

import (
	"context"
	"errors"

	"github.com/ante-rg/ante-app/api/internal/gen"
)

func (s *Server) GetReplay(ctx context.Context, _ gen.GetReplayRequestObject) (gen.GetReplayResponseObject, error) {
	m, err := s.loadActive(ctx)
	if errors.Is(err, errNoActiveModel) {
		return gen.GetReplay404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noActiveModel())}, nil
	}
	if errors.Is(err, errNoRanks) {
		return gen.GetReplay404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noRanks())}, nil
	}
	if err != nil {
		return nil, err
	}
	return gen.GetReplay200JSONResponse{
		MinDay:       0,
		MaxDay:       int(m.MaxDay),
		DefaultDay:   int(m.defaultDay()),
		ModelVersion: m.Version,
		Meta:         m.meta(m.Card.DefaultCapacity),
	}, nil
}
