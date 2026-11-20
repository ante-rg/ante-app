package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ante-rg/ante-app/api/internal/gen"
	"github.com/ante-rg/ante-app/api/internal/store"
)

const maxIngestEvents = 5000

var playerIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func validateBatch(b *gen.EventBatch) string {
	if !playerIDPattern.MatchString(b.PlayerId) {
		return "player_id must be 1 to 64 letters, digits, - or _"
	}
	if len(b.Events) < 1 || len(b.Events) > maxIngestEvents {
		return fmt.Sprintf("events must hold 1 to %d events", maxIngestEvents)
	}
	for i, e := range b.Events {
		switch {
		case e.EventOrder < 1:
			return fmt.Sprintf("events[%d].event_order must be at least 1", i)
		case !e.EventType.Valid():
			return fmt.Sprintf("events[%d].event_type is not a known type", i)
		case e.SecondsFromFirstEvent == nil:
			return fmt.Sprintf("events[%d].seconds_from_first_event is required", i)
		case *e.SecondsFromFirstEvent < 0:
			return fmt.Sprintf("events[%d].seconds_from_first_event must be at least 0", i)
		case e.Product != nil && (!e.Product.Valid() || *e.Product == gen.LessThannil):
			return fmt.Sprintf("events[%d].product is not a known product", i)
		case e.LocalHour != nil && (*e.LocalHour < 0 || *e.LocalHour > 23):
			return fmt.Sprintf("events[%d].local_hour must be 0 to 23", i)
		case e.Weekday != nil && (*e.Weekday < 1 || *e.Weekday > 7):
			return fmt.Sprintf("events[%d].weekday must be 1 to 7", i)
		}
	}
	return ""
}

func (s *Server) IngestEvents(ctx context.Context, req gen.IngestEventsRequestObject) (gen.IngestEventsResponseObject, error) {
	code, ok := operatorFrom(ctx)
	if !ok {
		return nil, errors.New("ingest: no operator in context")
	}
	b := req.Body
	if msg := validateBatch(b); msg != "" {
		return gen.IngestEvents422JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "invalid", Message: msg})}, nil
	}

	seen := make(map[int]bool, len(b.Events))
	orders := make([]int64, 0, len(b.Events))
	payloads := make([][]byte, 0, len(b.Events))
	for _, e := range b.Events {
		if seen[e.EventOrder] {
			continue
		}
		seen[e.EventOrder] = true
		p, err := json.Marshal(e)
		if err != nil {
			return nil, errors.New("ingest: encode event")
		}
		orders = append(orders, int64(e.EventOrder))
		payloads = append(payloads, p)
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ingest: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(s.DB).WithTx(tx)

	if err := q.LockPlayer(ctx, b.PlayerId); err != nil {
		return nil, fmt.Errorf("ingest: lock player: %w", err)
	}
	owner, err := q.PlayerOwner(ctx, b.PlayerId)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("ingest: player owner: %w", err)
	}
	if err == nil && owner != code {
		return gen.IngestEvents403JSONResponse(gen.Error{Code: "forbidden", Message: "The key belongs to another operator"}), nil
	}
	accepted, err := q.InsertEvents(ctx, store.InsertEventsParams{OperatorCode: code, PlayerID: b.PlayerId, Orders: orders, Payloads: payloads})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P05" {
		return gen.IngestEvents422JSONResponse{InvalidJSONResponse: gen.InvalidJSONResponse(gen.Error{Code: "invalid", Message: "events contain a NUL character"})}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ingest: insert events: %w", err)
	}
	last, err := q.LastEventOrder(ctx, store.LastEventOrderParams{OperatorCode: code, PlayerID: b.PlayerId})
	if err != nil {
		return nil, fmt.Errorf("ingest: last event order: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("ingest: commit: %w", err)
	}

	lastOrder := int(last)
	return gen.IngestEvents503JSONResponse{
		PlayerId:       b.PlayerId,
		Accepted:       int(accepted),
		Duplicates:     len(b.Events) - int(accepted),
		LastEventOrder: &lastOrder,
	}, nil
}
