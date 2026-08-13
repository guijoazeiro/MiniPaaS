package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
)

type DeploymentLogStore struct{ q *sqlc.Queries }

func NewDeploymentLogStore(q *sqlc.Queries) *DeploymentLogStore { return &DeploymentLogStore{q: q} }

func (s *DeploymentLogStore) Append(ctx context.Context, deploymentID uuid.UUID, stage, stream, message string) (domain.DeploymentLog, error) {
	row, err := s.q.CreateDeploymentLog(ctx, sqlc.CreateDeploymentLogParams{
		DeploymentID: uuidToPG(deploymentID), Stage: stage, Stream: stream, Message: message,
	})
	if err != nil {
		return domain.DeploymentLog{}, fmt.Errorf("store.CreateDeploymentLog: %w", err)
	}
	return toDomainDeploymentLog(row), nil
}

func (s *DeploymentLogStore) List(ctx context.Context, deploymentID uuid.UUID, afterID int64, limit int) ([]domain.DeploymentLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	rows, err := s.q.ListDeploymentLogs(ctx, sqlc.ListDeploymentLogsParams{
		DeploymentID: uuidToPG(deploymentID), AfterID: afterID, Lim: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store.ListDeploymentLogs: %w", err)
	}
	out := make([]domain.DeploymentLog, len(rows))
	for i, row := range rows {
		out[i] = toDomainDeploymentLog(row)
	}
	return out, nil
}

func (s *DeploymentLogStore) ListHealthCheckFailures(ctx context.Context, appID uuid.UUID, limit int) ([]domain.DeploymentLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 5
	}
	rows, err := s.q.ListHealthCheckLogsByApp(ctx, sqlc.ListHealthCheckLogsByAppParams{AppID: uuidToPG(appID), Lim: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("store.ListHealthCheckFailures: %w", err)
	}
	out := make([]domain.DeploymentLog, len(rows))
	for i, row := range rows {
		out[i] = toDomainDeploymentLog(row)
	}
	return out, nil
}

func toDomainDeploymentLog(row sqlc.DeploymentLog) domain.DeploymentLog {
	return domain.DeploymentLog{ID: row.ID, DeploymentID: pgToUUID(row.DeploymentID), Stage: row.Stage, Stream: row.Stream, Message: row.Message, CreatedAt: row.CreatedAt.Time}
}
