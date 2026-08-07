package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
)

type EnvStore struct {
	q *sqlc.Queries
}

func NewEnvStore(q *sqlc.Queries) *EnvStore { return &EnvStore{q: q} }

func (s *EnvStore) Upsert(ctx context.Context, appID uuid.UUID, key string, value, nonce []byte) error {
	err := s.q.UpsertEnvVar(ctx, sqlc.UpsertEnvVarParams{
		AppID: uuidToPG(appID),
		Key:   key,
		Value: value,
		Nonce: nonce,
	})
	if err != nil {
		return fmt.Errorf("store.UpsertEnvVar: %w", err)
	}
	return nil
}

func (s *EnvStore) ListKeys(ctx context.Context, appID uuid.UUID) ([]domain.EnvVarKey, error) {
	rows, err := s.q.ListEnvVarsByApp(ctx, uuidToPG(appID))
	if err != nil {
		return nil, fmt.Errorf("store.ListEnvVarsByApp: %w", err)
	}
	out := make([]domain.EnvVarKey, len(rows))
	for i, r := range rows {
		out[i] = domain.EnvVarKey{Key: r.Key, UpdatedAt: r.UpdatedAt.Time}
	}
	return out, nil
}

func (s *EnvStore) ListRecords(ctx context.Context, appID uuid.UUID) ([]store.EnvVarRecord, error) {
	rows, err := s.q.GetEnvVarsByApp(ctx, uuidToPG(appID))
	if err != nil {
		return nil, fmt.Errorf("store.GetEnvVarsByApp: %w", err)
	}
	out := make([]store.EnvVarRecord, len(rows))
	for i, r := range rows {
		out[i] = store.EnvVarRecord{
			Key:       r.Key,
			Value:     r.Value,
			Nonce:     r.Nonce,
			UpdatedAt: r.UpdatedAt.Time,
		}
	}
	return out, nil
}

func (s *EnvStore) Delete(ctx context.Context, appID uuid.UUID, key string) error {
	err := s.q.DeleteEnvVar(ctx, sqlc.DeleteEnvVarParams{
		AppID: uuidToPG(appID),
		Key:   key,
	})
	if err != nil {
		return fmt.Errorf("store.DeleteEnvVar: %w", err)
	}
	return nil
}
