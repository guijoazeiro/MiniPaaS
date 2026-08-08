package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type RollbackStore struct {
	q *sqlc.Queries
}

func NewRollbackStore(q *sqlc.Queries) *RollbackStore { return &RollbackStore{q: q} }

func (s *RollbackStore) Record(ctx context.Context, appID, fromDep, toDep uuid.UUID, triggeredBy string) error {
	err := s.q.RecordRollback(ctx, sqlc.RecordRollbackParams{
		AppID:           uuidToPG(appID),
		FromDeployment:  uuidToPG(fromDep),
		ToDeployment:    uuidToPG(toDep),
		TriggeredBy:     pgtype.Text{String: triggeredBy, Valid: triggeredBy != ""},
	})
	if err != nil {
		return fmt.Errorf("store.RecordRollback: %w", err)
	}
	return nil
}
