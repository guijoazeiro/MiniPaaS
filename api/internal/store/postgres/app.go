package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type AppStore struct {
	q *sqlc.Queries
}

func NewAppStore(q *sqlc.Queries) *AppStore {
	return &AppStore{q: q}
}

func (s *AppStore) Create(ctx context.Context, name string) (domain.App, error) {
	row, err := s.q.CreateApp(ctx, name)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.App{}, domain.ErrAppNameTaken
		}
		return domain.App{}, fmt.Errorf("store.CreateApp: %w", err)
	}
	return toDomainApp(row), nil
}

func (s *AppStore) GetByName(ctx context.Context, name string) (domain.App, error) {
	row, err := s.q.GetAppByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.App{}, domain.ErrAppNotFound
		}
		return domain.App{}, fmt.Errorf("store.GetAppByName: %w", err)
	}
	return toDomainApp(row), nil
}

func (s *AppStore) GetByID(ctx context.Context, id uuid.UUID) (domain.App, error) {
	row, err := s.q.GetAppByID(ctx, uuidToPG(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.App{}, domain.ErrAppNotFound
		}
		return domain.App{}, fmt.Errorf("store.GetAppByID: %w", err)
	}
	return toDomainApp(row), nil
}

func (s *AppStore) List(ctx context.Context) ([]domain.App, error) {
	rows, err := s.q.ListApps(ctx)
	if err != nil {
		return nil, fmt.Errorf("store.ListApps: %w", err)
	}
	out := make([]domain.App, len(rows))
	for i, r := range rows {
		out[i] = toDomainApp(r)
	}
	return out, nil
}

func (s *AppStore) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.AppStatus) error {
	err := s.q.UpdateAppStatus(ctx, sqlc.UpdateAppStatusParams{
		ID:     uuidToPG(id),
		Status: string(status),
	})
	if err != nil {
		return fmt.Errorf("store.UpdateAppStatus: %w", err)
	}
	return nil
}

func (s *AppStore) UpdatePublicURL(ctx context.Context, id uuid.UUID, url string) error {
	err := s.q.UpdateAppPublicURL(ctx, sqlc.UpdateAppPublicURLParams{
		ID:        uuidToPG(id),
		PublicUrl: pgtype.Text{String: url, Valid: url != ""},
	})
	if err != nil {
		return fmt.Errorf("store.UpdateAppPublicURL: %w", err)
	}
	return nil
}

func (s *AppStore) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteApp(ctx, uuidToPG(id)); err != nil {
		return fmt.Errorf("store.DeleteApp: %w", err)
	}
	return nil
}

func toDomainApp(row sqlc.App) domain.App {
	return domain.App{
		ID:        pgToUUID(row.ID),
		Name:      row.Name,
		Status:    domain.AppStatus(row.Status),
		PublicURL: pgText(row.PublicUrl),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(err.Error(), "SQLSTATE 23505")
}
