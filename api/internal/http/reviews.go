package http

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/ante-rg/ante-app/api/internal/auth"
	"github.com/ante-rg/ante-app/api/internal/gen"
	"github.com/ante-rg/ante-app/api/internal/store"
)

const maxNoteRunes = 2000

func invalid(msg string) gen.CreateReview422JSONResponse {
	return gen.CreateReview422JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "invalid", Message: msg})}
}

func reviewOut(r store.Review) gen.Review {
	out := gen.Review{
		Id: int(r.ID), PlayerId: r.PlayerID, AsOfDay: int(r.AsOfDay), Outcome: gen.Outcome(r.Outcome),
		Reviewer: openapi_types.Email(r.Reviewer), Blind: r.Blind, ModelVersion: r.ModelVersion, CreatedAt: r.CreatedAt.Time,
	}
	if r.Note.Valid {
		out.Note = &r.Note.String
	}
	return out
}

func (s *Server) CreateReview(ctx context.Context, req gen.CreateReviewRequestObject) (gen.CreateReviewResponseObject, error) {
	b := req.Body
	if b.AsOfDay == nil {
		return invalid("as_of_day is required"), nil
	}
	if !b.Outcome.Valid() {
		return invalid("outcome must be intervention_needed, no_concern or not_enough_data"), nil
	}
	if !b.Mode.Valid() {
		return invalid("mode must be normal or blind"), nil
	}
	if b.Note != nil && utf8.RuneCountInString(*b.Note) > maxNoteRunes {
		return invalid("note must be at most 2000 characters"), nil
	}
	m, err := s.loadActive(ctx)
	if errors.Is(err, errNoActiveModel) {
		return gen.CreateReview404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noActiveModel())}, nil
	}
	if errors.Is(err, errNoRanks) {
		return gen.CreateReview404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noRanks())}, nil
	}
	if err != nil {
		return nil, err
	}
	day, err := m.dayFor(b.AsOfDay)
	if err != nil {
		return invalid(err.Error()), nil
	}
	note := pgtype.Text{}
	if b.Note != nil {
		note = pgtype.Text{String: *b.Note, Valid: strings.TrimSpace(*b.Note) != ""}
	}
	email, ok := auth.Email(ctx)
	if !ok {
		return nil, errors.New("no authenticated user")
	}
	q := store.New(s.DB)
	last, err := q.PlayerLastDay(ctx, req.PlayerId)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.CreateReview404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(playerNotFound())}, nil
	}
	if err != nil {
		return nil, err
	}
	r, err := q.InsertReview(ctx, store.InsertReviewParams{
		PlayerID: req.PlayerId, AsOfDay: min(day, last), Outcome: string(b.Outcome), Note: note,
		Reviewer: email, Blind: b.Mode == gen.ReviewCreateModeBlind, ModelVersion: m.Version,
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateReview201JSONResponse(reviewOut(r)), nil
}

func (s *Server) ListPlayerReviews(ctx context.Context, req gen.ListPlayerReviewsRequestObject) (gen.ListPlayerReviewsResponseObject, error) {
	m, err := s.loadActive(ctx)
	if errors.Is(err, errNoActiveModel) {
		return gen.ListPlayerReviews404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noActiveModel())}, nil
	}
	if errors.Is(err, errNoRanks) {
		return gen.ListPlayerReviews404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noRanks())}, nil
	}
	if err != nil {
		return nil, err
	}
	day, err := m.dayFor(req.Params.AsOfDay)
	if err != nil {
		return gen.ListPlayerReviews400JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "bad_request", Message: err.Error()})}, nil
	}
	params := store.ListPlayerReviewsParams{PlayerID: req.PlayerId, Day: day}
	if mode := req.Params.Mode; mode != nil {
		if !mode.Valid() {
			return gen.ListPlayerReviews400JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "bad_request", Message: "mode must be normal or blind"})}, nil
		}
		if *mode == gen.ListPlayerReviewsParamsModeBlind {
			email, ok := auth.Email(ctx)
			if !ok {
				return nil, errors.New("no authenticated user")
			}
			params.BlindReviewer = pgtype.Text{String: email, Valid: true}
		}
	}
	q := store.New(s.DB)
	if _, err := q.PlayerLastDay(ctx, req.PlayerId); errors.Is(err, pgx.ErrNoRows) {
		return gen.ListPlayerReviews404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(playerNotFound())}, nil
	} else if err != nil {
		return nil, err
	}
	rows, err := q.ListPlayerReviews(ctx, params)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Review, len(rows))
	for i, r := range rows {
		items[i] = reviewOut(r)
	}
	return gen.ListPlayerReviews200JSONResponse{Items: items}, nil
}
