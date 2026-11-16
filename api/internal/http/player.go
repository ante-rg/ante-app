package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/ante-rg/ante-app/api/internal/gen"
	"github.com/ante-rg/ante-app/api/internal/store"
)

var errPlayerNotFound = errors.New("player not found")

func playerNotFound() gen.Error {
	return gen.Error{Code: "not_found", Message: "Player not found"}
}

func dayPtr(d int32) *int {
	if d < 0 {
		return nil
	}
	v := int(d)
	return &v
}

func (s *Server) playerRow(ctx context.Context, m activeModel, id string, day int32) (store.GetPlayerCaseRow, error) {
	row, err := store.New(s.DB).GetPlayerCase(ctx, store.GetPlayerCaseParams{ModelVersion: m.Version, Day: day, PlayerID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errPlayerNotFound
	}
	return row, err
}

func (s *Server) GetPlayer(ctx context.Context, req gen.GetPlayerRequestObject) (gen.GetPlayerResponseObject, error) {
	m, err := s.loadActive(ctx)
	if errors.Is(err, errNoActiveModel) {
		return gen.GetPlayer404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noActiveModel())}, nil
	}
	if errors.Is(err, errNoRanks) {
		return gen.GetPlayer404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noRanks())}, nil
	}
	if err != nil {
		return nil, err
	}
	if req.Params.Mode != nil && !req.Params.Mode.Valid() {
		return gen.GetPlayer400JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "bad_request", Message: "mode must be normal or blind"})}, nil
	}
	day, err := m.dayFor(req.Params.AsOfDay)
	if err != nil {
		return gen.GetPlayer400JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "bad_request", Message: err.Error()})}, nil
	}
	if req.Params.Mode != nil && *req.Params.Mode == gen.GetPlayerParamsModeBlind {
		return gen.GetPlayer501JSONResponse{NotImplementedJSONResponse: gen.NotImplementedJSONResponse(
			gen.Error{Code: "not_implemented", Message: "Blind mode is not implemented yet"})}, nil
	}
	r, err := s.playerRow(ctx, m, req.PlayerId, day)
	if errors.Is(err, errPlayerNotFound) {
		return gen.GetPlayer404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(playerNotFound())}, nil
	}
	if err != nil {
		return nil, err
	}
	eff := min(day, r.LastDay)
	band := gen.Band(r.Band)
	out := gen.PlayerCase{
		PlayerId:          r.PlayerID,
		OperatorCode:      r.OperatorCode,
		AsOfDay:           int(day),
		EffectiveDay:      int(eff),
		DaysObserved:      int(eff) + 1,
		SessionsToDate:    int(r.SessionsToDate),
		DepositsToDate:    int(r.DepositsToDate),
		DepositsEurToDate: &r.DepositsEurToDate,
		Band:              &band,
		RankOf:            new(int(r.RankOf)),
		FirstHighDay:      dayPtr(r.FirstHighDay),
		FirstRuleFlagDay:  dayPtr(r.FirstRuleFlagDay),
		Meta:              m.meta(m.Card.DefaultCapacity),
	}
	if day > r.LastDay {
		out.HistoryEndedOnDay = new(int(r.LastDay))
	}
	if r.Score.Valid {
		out.Score = &r.Score.Float32
	}
	if r.Rank.Valid {
		out.Rank = new(int(r.Rank.Int32))
	}
	reasons := []gen.Reason{}
	if r.Reasons != nil {
		if err := json.Unmarshal(r.Reasons, &reasons); err != nil {
			return nil, fmt.Errorf("decode reasons: %w", err)
		}
	}
	out.Reasons = &reasons
	flags, err := store.New(s.DB).ListPlayerRuleFlags(ctx, store.ListPlayerRuleFlagsParams{PlayerID: r.PlayerID, EffectiveDay: eff})
	if err != nil {
		return nil, err
	}
	rf := make([]struct {
		FirstDay int    `json:"first_day"`
		RuleId   string `json:"rule_id"`
		Text     string `json:"text"`
	}, len(flags))
	for i, f := range flags {
		rf[i].FirstDay, rf[i].RuleId, rf[i].Text = int(f.FirstDay), f.RuleID, f.RuleText
	}
	out.RuleFlags = &rf
	return gen.GetPlayer200JSONResponse(out), nil
}

func (s *Server) GetPlayerTimeline(ctx context.Context, req gen.GetPlayerTimelineRequestObject) (gen.GetPlayerTimelineResponseObject, error) {
	m, err := s.loadActive(ctx)
	if errors.Is(err, errNoActiveModel) {
		return gen.GetPlayerTimeline404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noActiveModel())}, nil
	}
	if errors.Is(err, errNoRanks) {
		return gen.GetPlayerTimeline404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noRanks())}, nil
	}
	if err != nil {
		return nil, err
	}
	day, err := m.dayFor(req.Params.AsOfDay)
	if err != nil {
		return gen.GetPlayerTimeline400JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "bad_request", Message: err.Error()})}, nil
	}
	r, err := s.playerRow(ctx, m, req.PlayerId, day)
	if errors.Is(err, errPlayerNotFound) {
		return gen.GetPlayerTimeline404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(playerNotFound())}, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := store.New(s.DB).ListTimelineDays(ctx, store.ListTimelineDaysParams{
		ModelVersion: m.Version, PlayerID: r.PlayerID, EffectiveDay: min(day, r.LastDay),
	})
	if err != nil {
		return nil, err
	}
	days := make([]gen.TimelineDay, len(rows))
	for i, a := range rows {
		d := gen.TimelineDay{
			Day:            int(a.Day),
			DepositsN:      new(int(a.DepositsN)),
			DepositsEur:    new(a.DepositsEur),
			SessionsN:      new(int(a.SessionsN)),
			PlayMinutes:    new(a.PlayMinutes),
			NetResultEur:   new(a.NetResultEur),
			ChaseSessionsN: new(int(a.ChaseSessionsN)),
			NightSessionsN: new(int(a.NightSessionsN)),
		}
		if a.EndBalanceEur.Valid {
			d.EndBalanceEur = &a.EndBalanceEur.Float64
		}
		if a.Score.Valid {
			d.Score = &a.Score.Float32
		}
		if a.Band.Valid {
			d.Band = new(gen.Band(a.Band.String))
		}
		days[i] = d
	}
	return gen.GetPlayerTimeline200JSONResponse(gen.Timeline{
		PlayerId: r.PlayerID, AsOfDay: int(day), Days: days,
		FirstHighDay: dayPtr(r.FirstHighDay), FirstRuleFlagDay: dayPtr(r.FirstRuleFlagDay),
		Meta: m.meta(m.Card.DefaultCapacity),
	}), nil
}
