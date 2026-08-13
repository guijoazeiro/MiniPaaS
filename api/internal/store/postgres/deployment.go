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

func (s *DeploymentStore) CreateGit(ctx context.Context, appID uuid.UUID, imageTag, repository, branch string) (domain.Deployment, error) {
	row, err := s.q.CreateGitDeployment(ctx, sqlc.CreateGitDeploymentParams{
		AppID: uuidToPG(appID), ImageTag: imageTag, Repository: pgtype.Text{String: repository, Valid: true}, Branch: pgtype.Text{String: branch, Valid: true},
	})
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("store.CreateGitDeployment: %w", err)
	}
	return toDomainDeployment(row), nil
}

func (s *DeploymentStore) CreateGitTriggered(ctx context.Context, appID uuid.UUID, imageTag, repository, branch, triggerType, deliveryID string) (domain.Deployment, error) {
	row, err := s.q.CreateTriggeredGitDeployment(ctx, sqlc.CreateTriggeredGitDeploymentParams{
		AppID: uuidToPG(appID), ImageTag: imageTag,
		Repository: pgtype.Text{String: repository, Valid: true}, Branch: pgtype.Text{String: branch, Valid: true},
		TriggerType: triggerType, GithubDeliveryID: pgtype.Text{String: deliveryID, Valid: deliveryID != ""},
	})
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("store.CreateTriggeredGitDeployment: %w", err)
	}
	return toDomainDeployment(row), nil
}

func (s *DeploymentStore) CreateGitRetry(ctx context.Context, appID uuid.UUID, imageTag, repository, branch string, retryOf uuid.UUID, attempt int) (domain.Deployment, error) {
	row, err := s.q.CreateRetryGitDeployment(ctx, sqlc.CreateRetryGitDeploymentParams{
		AppID: uuidToPG(appID), ImageTag: imageTag,
		Repository: pgtype.Text{String: repository, Valid: true}, Branch: pgtype.Text{String: branch, Valid: true},
		Attempt: int32(attempt), RetryOf: uuidToPG(retryOf),
	})
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("store.CreateGitRetryDeployment: %w", err)
	}
	return toDomainDeployment(row), nil
}

func (s *DeploymentStore) UpdateGitMetadata(ctx context.Context, id uuid.UUID, commitSHA, author, message, branch string) error {
	err := s.q.UpdateDeploymentGitMetadata(ctx, sqlc.UpdateDeploymentGitMetadataParams{
		ID: uuidToPG(id), CommitSha: pgtype.Text{String: commitSHA, Valid: true},
		CommitAuthor:  pgtype.Text{String: author, Valid: author != ""},
		CommitMessage: pgtype.Text{String: message, Valid: message != ""},
		Branch:        pgtype.Text{String: branch, Valid: branch != ""},
	})
	if err != nil {
		return fmt.Errorf("store.UpdateDeploymentGitMetadata: %w", err)
	}
	return nil
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

func (s *DeploymentStore) ListRunning(ctx context.Context) ([]domain.Deployment, error) {
	rows, err := s.q.ListRunningDeployments(ctx)
	if err != nil {
		return nil, fmt.Errorf("store.ListRunning: %w", err)
	}
	out := make([]domain.Deployment, len(rows))
	for i, row := range rows {
		out[i] = toDomainDeployment(row)
	}
	return out, nil
}

func (s *DeploymentStore) ListByApp(ctx context.Context, appID uuid.UUID, limit int) ([]domain.Deployment, error) {
	rows, err := s.q.ListDeploymentsByApp(ctx, sqlc.ListDeploymentsByAppParams{
		AppID: uuidToPG(appID),
		Lim:   int32(limit),
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

func (s *DeploymentStore) ListAll(ctx context.Context, appName, status string, limit, offset int) ([]domain.DeploymentListItem, error) {
	rows, err := s.q.ListDeployments(ctx, sqlc.ListDeploymentsParams{
		AppName: pgtype.Text{String: appName, Valid: appName != ""},
		Status:  pgtype.Text{String: status, Valid: status != ""},
		Lim:     int32(limit),
		Off:     int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("store.ListDeployments: %w", err)
	}
	out := make([]domain.DeploymentListItem, len(rows))
	for i, row := range rows {
		deployment := toDomainDeployment(sqlc.Deployment{
			ID: row.ID, AppID: row.AppID, ImageTag: row.ImageTag, Status: row.Status,
			ContainerID: row.ContainerID, Port: row.Port, CommitSha: row.CommitSha,
			DurationMs: row.DurationMs, CreatedAt: row.CreatedAt, FinishedAt: row.FinishedAt,
			SourceType: row.SourceType, Repository: row.Repository, Branch: row.Branch,
			CommitAuthor: row.CommitAuthor, CommitMessage: row.CommitMessage,
			TriggerType: row.TriggerType, GithubDeliveryID: row.GithubDeliveryID,
			Attempt: row.Attempt, RetryOf: row.RetryOf, CancelRequested: row.CancelRequested,
		})
		out[i] = domain.DeploymentListItem{Deployment: deployment, AppName: row.AppName}
	}
	return out, nil
}

func (s *DeploymentStore) CountAll(ctx context.Context, appName, status string) (int64, error) {
	total, err := s.q.CountDeployments(ctx, sqlc.CountDeploymentsParams{
		AppName: pgtype.Text{String: appName, Valid: appName != ""},
		Status:  pgtype.Text{String: status, Valid: status != ""},
	})
	if err != nil {
		return 0, fmt.Errorf("store.CountDeployments: %w", err)
	}
	return total, nil
}

func (s *DeploymentStore) ListForRetention(ctx context.Context, appID uuid.UUID, keep int) ([]domain.Deployment, error) {
	rows, err := s.q.ListDeploymentsForRetention(ctx, sqlc.ListDeploymentsForRetentionParams{
		AppID: uuidToPG(appID),
		Keep:  int32(keep),
	})
	if err != nil {
		return nil, fmt.Errorf("store.ListDeploymentsForRetention: %w", err)
	}
	out := make([]domain.Deployment, len(rows))
	for i, r := range rows {
		out[i] = toDomainDeployment(r)
	}
	return out, nil
}

func (s *DeploymentStore) UpdateRunning(ctx context.Context, id uuid.UUID, containerID string, port int, imageTag string, durationMs int) error {
	err := s.q.UpdateDeploymentRunning(ctx, sqlc.UpdateDeploymentRunningParams{
		ID:          uuidToPG(id),
		ContainerID: pgtype.Text{String: containerID, Valid: true},
		Port:        pgtype.Int4{Int32: int32(port), Valid: true},
		ImageTag:    imageTag,
		DurationMs:  pgtype.Int4{Int32: int32(durationMs), Valid: true},
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

func (s *DeploymentStore) RequestCancel(ctx context.Context, id uuid.UUID) (domain.Deployment, error) {
	row, err := s.q.RequestDeploymentCancel(ctx, uuidToPG(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return domain.Deployment{}, domain.ErrDeploymentNotCancellable }
		return domain.Deployment{}, fmt.Errorf("store.RequestDeploymentCancel: %w", err)
	}
	return toDomainDeployment(row), nil
}

func (s *DeploymentStore) MarkCancelled(ctx context.Context, id uuid.UUID) error {
	if err := s.q.MarkDeploymentCancelled(ctx, uuidToPG(id)); err != nil {
		return fmt.Errorf("store.MarkDeploymentCancelled: %w", err)
	}
	return nil
}

func toDomainDeployment(row sqlc.Deployment) domain.Deployment {
	d := domain.Deployment{
		ID:               pgToUUID(row.ID),
		AppID:            pgToUUID(row.AppID),
		ImageTag:         row.ImageTag,
		Status:           domain.DeploymentStatus(row.Status),
		ContainerID:      pgText(row.ContainerID),
		Port:             pgInt4(row.Port),
		CommitSHA:        pgText(row.CommitSha),
		SourceType:       row.SourceType,
		Repository:       pgText(row.Repository),
		Branch:           pgText(row.Branch),
		CommitAuthor:     pgText(row.CommitAuthor),
		CommitMessage:    pgText(row.CommitMessage),
		TriggerType:      row.TriggerType,
		GitHubDeliveryID: pgText(row.GithubDeliveryID),
		Attempt:          int(row.Attempt),
		CancelRequested:  row.CancelRequested,
		DurationMs:       pgInt4(row.DurationMs),
		CreatedAt:        row.CreatedAt.Time,
	}
	if row.RetryOf.Valid { retry := pgToUUID(row.RetryOf); d.RetryOf = &retry }
	if row.FinishedAt.Valid {
		t := row.FinishedAt.Time
		d.FinishedAt = &t
	}
	return d
}
