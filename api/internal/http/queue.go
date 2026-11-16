package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ante-rg/ante-app/api/internal/gen"
	"github.com/ante-rg/ante-app/api/internal/store"
)

func queueBadRequest(msg string) gen.GetQueue400JSONResponse {
	return gen.GetQueue400JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "bad_request", Message: msg})}
}

func parseBands(raw *string) ([]string, error) {
	if raw == nil {
		return []string{string(gen.High), string(gen.Elevated)}, nil
	}
	parts := strings.Split(*raw, ",")
	for _, b := range parts {
		if !gen.Band(b).Valid() {
			return nil, fmt.Errorf("unknown band %q", b)
		}
	}
	return parts, nil
}

func (s *Server) GetQueue(ctx context.Context, req gen.GetQueueRequestObject) (gen.GetQueueResponseObject, error) {
	p := req.Params
	m, err := s.loadActive(ctx)
	if errors.Is(err, errNoActiveModel) {
		return gen.GetQueue404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noActiveModel())}, nil
	}
	if errors.Is(err, errNoRanks) {
		return gen.GetQueue404JSONResponse{NotFoundJSONResponse: gen.NotFoundJSONResponse(noRanks())}, nil
	}
	if err != nil {
		return nil, err
	}
	if p.Mode != nil && !p.Mode.Valid() {
		return queueBadRequest("mode must be normal or blind"), nil
	}
	if p.ReviewStatus != nil && !p.ReviewStatus.Valid() {
		return queueBadRequest("review_status must be all, pending or reviewed"), nil
	}
	day, err := m.dayFor(p.AsOfDay)
	if err != nil {
		return queueBadRequest(err.Error()), nil
	}
	bands, err := parseBands(p.Band)
	if err != nil {
		return queueBadRequest(err.Error()), nil
	}
	capacity := m.Card.DefaultCapacity
	if p.Capacity != nil {
		if *p.Capacity < 1 || *p.Capacity > maxCapacity {
			return queueBadRequest(fmt.Sprintf("capacity must be between 1 and %d", maxCapacity)), nil
		}
		capacity = int32(*p.Capacity)
	}
	if p.Mode != nil && *p.Mode == gen.GetQueueParamsModeBlind {
		return gen.GetQueue501JSONResponse{NotImplementedJSONResponse: gen.NotImplementedJSONResponse(
			gen.Error{Code: "not_implemented", Message: "Blind mode is not implemented yet"})}, nil
	}

	q := store.New(s.DB)
	c, err := q.QueueCounts(ctx, store.QueueCountsParams{ModelVersion: m.Version, Day: day})
	if err != nil {
		return nil, err
	}
	items := []gen.QueueItem{}
	if p.ReviewStatus == nil || *p.ReviewStatus != gen.GetQueueParamsReviewStatusReviewed {
		var op pgtype.Text
		if p.Operator != nil {
			op = pgtype.Text{String: *p.Operator, Valid: true}
		}
		rows, err := q.ListQueue(ctx, store.ListQueueParams{
			ModelVersion: m.Version, Day: day, Bands: bands, Operator: op, RowLimit: capacity,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			it, err := queueItem(r, day)
			if err != nil {
				return nil, err
			}
			items = append(items, it)
		}
	}

	out := gen.Queue{AsOfDay: int(day), Items: items, Meta: m.meta(capacity), Mode: gen.QueueMode(gen.GetQueueParamsModeNormal)}
	high, elevated, low, insuff, pending, newHigh := int(c.High), int(c.Elevated), int(c.Low), int(c.InsufficientData), int(c.High+c.Elevated), int(c.NewHigh)
	out.Counts.High, out.Counts.Elevated, out.Counts.Low = &high, &elevated, &low
	out.Counts.InsufficientData, out.Counts.PendingReview, out.Counts.NewHighSincePreviousDay = &insuff, &pending, &newHigh
	return gen.GetQueue200JSONResponse(out), nil
}

func queueItem(r store.ListQueueRow, day int32) (gen.QueueItem, error) {
	eff := min(day, r.LastDay)
	band := gen.Band(r.Band)
	it := gen.QueueItem{
		PlayerId:       r.PlayerID,
		OperatorCode:   r.OperatorCode,
		EffectiveDay:   int(eff),
		DaysObserved:   int(eff) + 1,
		Band:           &band,
		ReviewStatus:   gen.QueueItemReviewStatusPending,
		FlaggedByRules: &r.Flagged,
	}
	if day > r.LastDay {
		last := int(r.LastDay)
		it.HistoryEndedOnDay = &last
	}
	if r.Rank.Valid {
		rank := int(r.Rank.Int32)
		it.Rank = &rank
	}
	if r.Score.Valid {
		it.Score = &r.Score.Float32
	}
	var trend []*float32
	if err := json.Unmarshal(r.Trend, &trend); err != nil {
		return it, fmt.Errorf("decode trend: %w", err)
	}
	it.ScoreTrend30d = &trend
	if r.Reasons != nil {
		var reasons []gen.Reason
		if err := json.Unmarshal(r.Reasons, &reasons); err != nil {
			return it, fmt.Errorf("decode reasons: %w", err)
		}
		best := -1
		for i, rs := range reasons {
			if best < 0 || rs.Weight > reasons[best].Weight {
				best = i
			}
		}
		if best >= 0 {
			it.MainReason = &reasons[best].Text
		}
	}
	return it, nil
}
