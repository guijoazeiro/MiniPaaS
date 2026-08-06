package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type DeploymentStore struct {
	q *sqlc.Queries
}

func NewDeploymentStore(q *sqlc.Queries) *DeploymentStore {
	return &DeploymentStore{q: q}
}

func (s *DeploymentStore) Create(ctx context.Context, appID uuid.UUID, imageTag string) (domain.Deployment, error) {
	row, err := s.q.CreateDeployment(ctx, sqlc.CreateDeploymentParams{
		AppID:    uuidToPG(appID),
		ImageTag: imageTag,
	})
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("store.CreateDeployment: %w", err)
	}
	return toDomainDeployment(row), nil
}

func (s *DeploymentStore) GetByID(ctx context.Context, id uuid.UUID) (domain.Deployment, error) {
	row, err := s.q.GetDeploymentByID(ctx, uuidToPG(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Deployment{}, domain.ErrDeploymentNotFound
		}
		return domain.Deployment{}, fmt.Errorf("store.GetDeploymentByID: %w", err)
	}
	return toDomainDeployment(row), nil
}

func (s *DeploymentStore) GetActive(ctx context.Context, appID uuid.UUID) (domain.Deployment, error) {
	row, err := s.q.GetActiveDeployment(ctx, uuidToPG(appID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Deployment{}, domain.ErrDeploymentNotFound
		}
		return domain.Deployment{}, fmt.Errorf("store.GetActiveDeployment: %w", err)
	}
	return toDomainDeployment(row), nil
}

func (s *DeploymentStore) ListByApp(ctx context.Context, appID uuid.UUID, limit int32) ([]domain.Deployment, error) {
	rows, err := s.q.ListDeploymentsByApp(ctx, sqlc.ListDeploymentsByAppParams{
		AppID: uuidToPG(appID),
		Lim:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("store.ListDeploymentsByApp: %w", err)
	}
	out := make([]domain.Deployment, len(rows))
	for i, r := range rows {
		out[i] = toDomainDeployment(r)
	}
	return out, nil
}

func (s *DeploymentStore) UpdateRunning(ctx context.Context, id uuid.UUID, containerID string, port int, imageTag string) error {
	err := s.q.UpdateDeploymentRunning(ctx, sqlc.UpdateDeploymentRunningParams{
		ID:          uuidToPG(id),
		ContainerID: pgtype.Text{String: containerID, Valid: true},
		Port:        pgtype.Int4{Int32: int32(port), Valid: true},
		ImageTag:    imageTag,
	})
	if err != nil {
		return fmt.Errorf("store.UpdateDeploymentRunning: %w", err)
	}
	return nil
}

func (s *DeploymentStore) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.DeploymentStatus) error {
	err := s.q.UpdateDeploymentStatus(ctx, sqlc.UpdateDeploymentStatusParams{
		ID:     uuidToPG(id),
		Status: string(status),
	})
	if err != nil {
		return fmt.Errorf("store.UpdateDeploymentStatus: %w", err)
	}
	return nil
}

func toDomainDeployment(row sqlc.Deployment) domain.Deployment {
	d := domain.Deployment{
		ID:          pgToUUID(row.ID),
		AppID:       pgToUUID(row.AppID),
		ImageTag:    row.ImageTag,
		Status:      domain.DeploymentStatus(row.Status),
		ContainerID: pgText(row.ContainerID),
		Port:        pgInt4(row.Port),
		CommitSHA:   pgText(row.CommitSha),
		DurationMs:  pgInt4(row.DurationMs),
		CreatedAt:   row.CreatedAt.Time,
	}
	if row.FinishedAt.Valid {
		t := row.FinishedAt.Time
		d.FinishedAt = &t
	}
	return d
}
