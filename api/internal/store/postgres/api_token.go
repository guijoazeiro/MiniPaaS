package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type APITokenStore struct {
	q *sqlc.Queries
}

var _ store.APITokenStore = (*APITokenStore)(nil)

func NewAPITokenStore(q *sqlc.Queries) *APITokenStore { return &APITokenStore{q: q} }

func (s *APITokenStore) CreateAPIToken(ctx context.Context, userID uuid.UUID, name, tokenHash, tokenPrefix string, scopes []string, expiresAt *time.Time) (domain.APIToken, error) {
	row, err := s.q.CreateAPIToken(ctx, sqlc.CreateAPITokenParams{
		UserID: uuidToPG(userID), Name: name, TokenHash: tokenHash,
		TokenPrefix: tokenPrefix, Scopes: scopes, ExpiresAt: nullableTime(expiresAt),
	})
	if err != nil {
		return domain.APIToken{}, fmt.Errorf("store.CreateAPIToken: %w", err)
	}
	return toDomainAPIToken(row), nil
}

func (s *APITokenStore) ListAPITokens(ctx context.Context, userID uuid.UUID) ([]domain.APIToken, error) {
	rows, err := s.q.ListAPITokensForUser(ctx, uuidToPG(userID))
	if err != nil {
		return nil, fmt.Errorf("store.ListAPITokens: %w", err)
	}
	items := make([]domain.APIToken, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainAPIToken(row))
	}
	return items, nil
}

func (s *APITokenStore) GetAPITokenByHash(ctx context.Context, tokenHash string) (domain.APIToken, error) {
	row, err := s.q.GetAPITokenByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.APIToken{}, domain.ErrAPITokenNotFound
		}
		return domain.APIToken{}, fmt.Errorf("store.GetAPITokenByHash: %w", err)
	}
	return toDomainAPIToken(row), nil
}

func (s *APITokenStore) RevokeAPIToken(ctx context.Context, userID, tokenID uuid.UUID) error {
	rows, err := s.q.RevokeAPIToken(ctx, sqlc.RevokeAPITokenParams{ID: uuidToPG(tokenID), UserID: uuidToPG(userID)})
	if err != nil {
		return fmt.Errorf("store.RevokeAPIToken: %w", err)
	}
	if rows == 0 {
		return domain.ErrAPITokenNotFound
	}
	return nil
}

func (s *APITokenStore) TouchAPIToken(ctx context.Context, tokenID uuid.UUID) error {
	if err := s.q.TouchAPIToken(ctx, uuidToPG(tokenID)); err != nil {
		return fmt.Errorf("store.TouchAPIToken: %w", err)
	}
	return nil
}

func toDomainAPIToken(row sqlc.ApiToken) domain.APIToken {
	return domain.APIToken{
		ID: pgToUUID(row.ID), UserID: pgToUUID(row.UserID), Name: row.Name, TokenPrefix: row.TokenPrefix,
		Scopes: append([]string(nil), row.Scopes...), ExpiresAt: nullableTimeValue(row.ExpiresAt),
		RevokedAt: nullableTimeValue(row.RevokedAt), LastUsedAt: nullableTimeValue(row.LastUsedAt),
		CreatedAt: row.CreatedAt.Time,
	}
}

func nullableTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func nullableTimeValue(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
