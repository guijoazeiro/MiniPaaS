package postgres

import (
	"context"
	"fmt"

	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
)

type AuditStore struct {
	q *sqlc.Queries
}

func NewAuditStore(q *sqlc.Queries) *AuditStore { return &AuditStore{q: q} }

func (s *AuditStore) Record(ctx context.Context, event store.AuditEvent) error {
	if err := s.q.CreateAuditEvent(ctx, sqlc.CreateAuditEventParams{
		UserID:     nullableUUID(event.UserID),
		Action:     event.Action,
		Method:     event.Method,
		Path:       event.Path,
		StatusCode: int32(event.Status),
		RequestID:  event.RequestID,
	}); err != nil {
		return fmt.Errorf("store.RecordAuditEvent: %w", err)
	}
	return nil
}

func (s *AuditStore) List(ctx context.Context, limit, offset int) ([]store.AuditEvent, error) {
	params := sqlc.ListAuditEventsParams{Limit: int32(limit), Offset: int32(offset)}
	if userID, ok := authctx.UserID(ctx); ok {
		params.UserID = nullableUUID(userID)
	}
	rows, err := s.q.ListAuditEvents(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("store.ListAuditEvents: %w", err)
	}
	events := make([]store.AuditEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, store.AuditEvent{
			ID: row.ID, UserID: pgToUUID(row.UserID), Action: row.Action,
			Method: row.Method, Path: row.Path, Status: int(row.StatusCode),
			RequestID: row.RequestID, CreatedAt: row.CreatedAt.Time,
		})
	}
	return events, nil
}

var _ store.AuditStore = (*AuditStore)(nil)
