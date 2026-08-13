package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CustomDomainStore struct{ q *sqlc.Queries }

func NewCustomDomainStore(q *sqlc.Queries) *CustomDomainStore { return &CustomDomainStore{q: q} }

func (s *CustomDomainStore) Create(ctx context.Context, appID uuid.UUID, hostname string) (domain.CustomDomain, error) {
	row, err := s.q.CreateCustomDomain(ctx, sqlc.CreateCustomDomainParams{AppID: uuidToPG(appID), Hostname: hostname})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.CustomDomain{}, domain.ErrCustomDomainTaken
		}
		return domain.CustomDomain{}, fmt.Errorf("store.CreateCustomDomain: %w", err)
	}
	return toDomainCustomDomain(row), nil
}

func (s *CustomDomainStore) Get(ctx context.Context, id uuid.UUID) (domain.CustomDomain, error) {
	row, err := s.q.GetCustomDomain(ctx, uuidToPG(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CustomDomain{}, domain.ErrCustomDomainNotFound
	}
	if err != nil {
		return domain.CustomDomain{}, fmt.Errorf("store.GetCustomDomain: %w", err)
	}
	return toDomainCustomDomain(row), nil
}

func (s *CustomDomainStore) ListByApp(ctx context.Context, appID uuid.UUID) ([]domain.CustomDomain, error) {
	rows, err := s.q.ListCustomDomainsByApp(ctx, uuidToPG(appID))
	if err != nil {
		return nil, fmt.Errorf("store.ListCustomDomainsByApp: %w", err)
	}
	out := make([]domain.CustomDomain, len(rows))
	for i, row := range rows {
		out[i] = toDomainCustomDomain(row)
	}
	return out, nil
}

func (s *CustomDomainStore) UpdateVerification(ctx context.Context, id uuid.UUID, status domain.CustomDomainStatus, lastError string, verifiedAt *time.Time) error {
	verified := pgtype.Timestamptz{}
	if verifiedAt != nil {
		verified = pgtype.Timestamptz{Time: *verifiedAt, Valid: true}
	}
	if err := s.q.UpdateCustomDomainVerification(ctx, sqlc.UpdateCustomDomainVerificationParams{
		ID: uuidToPG(id), Status: string(status), LastError: pgtype.Text{String: lastError, Valid: lastError != ""}, VerifiedAt: verified,
	}); err != nil {
		return fmt.Errorf("store.UpdateCustomDomainVerification: %w", err)
	}
	return nil
}

func (s *CustomDomainStore) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteCustomDomain(ctx, uuidToPG(id)); err != nil {
		return fmt.Errorf("store.DeleteCustomDomain: %w", err)
	}
	return nil
}

func toDomainCustomDomain(row sqlc.CustomDomain) domain.CustomDomain {
	d := domain.CustomDomain{
		ID: row.ID.Bytes, AppID: row.AppID.Bytes, Hostname: row.Hostname,
		Status: domain.CustomDomainStatus(row.Status), LastError: pgText(row.LastError),
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
	if row.VerifiedAt.Valid {
		verifiedAt := row.VerifiedAt.Time
		d.VerifiedAt = &verifiedAt
	}
	return d
}
