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

type GitSourceStore struct {
	q *sqlc.Queries
}

func NewGitSourceStore(q *sqlc.Queries) *GitSourceStore {
	return &GitSourceStore{q: q}
}

func (s *GitSourceStore) Upsert(ctx context.Context, source domain.GitSource) (domain.GitSource, error) {
	row, err := s.q.UpsertGitSource(ctx, sqlc.UpsertGitSourceParams{
		AppID:                uuidToPG(source.AppID),
		Repository:           source.Repository,
		Branch:               source.Branch,
		BuildContext:         source.BuildContext,
		DockerfilePath:       source.DockerfilePath,
		AccessMode:           source.AccessMode,
		GithubInstallationID: optionalInt8(source.GitHubInstallationID),
		GithubRepositoryID:   optionalInt8(source.GitHubRepositoryID),
		Private:              source.Private,
	})
	if err != nil {
		return domain.GitSource{}, fmt.Errorf("store.UpsertGitSource: %w", err)
	}
	return toDomainGitSource(row), nil
}

func (s *GitSourceStore) Get(ctx context.Context, appID uuid.UUID) (domain.GitSource, error) {
	row, err := s.q.GetGitSource(ctx, uuidToPG(appID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.GitSource{}, domain.ErrGitSourceNotFound
		}
		return domain.GitSource{}, fmt.Errorf("store.GetGitSource: %w", err)
	}
	return toDomainGitSource(row), nil
}

func (s *GitSourceStore) Delete(ctx context.Context, appID uuid.UUID) error {
	if err := s.q.DeleteGitSource(ctx, uuidToPG(appID)); err != nil {
		return fmt.Errorf("store.DeleteGitSource: %w", err)
	}
	return nil
}

func (s *GitSourceStore) SetAutoDeploy(ctx context.Context, appID uuid.UUID, enabled bool) (domain.GitSource, error) {
	row, err := s.q.SetGitSourceAutoDeploy(ctx, sqlc.SetGitSourceAutoDeployParams{AppID: uuidToPG(appID), AutoDeploy: enabled})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.GitSource{}, domain.ErrGitSourceNotFound
		}
		return domain.GitSource{}, fmt.Errorf("store.SetGitSourceAutoDeploy: %w", err)
	}
	return toDomainGitSource(row), nil
}

func (s *GitSourceStore) ListAutoDeployByRepository(ctx context.Context, repositoryID int64) ([]domain.GitSource, error) {
	rows, err := s.q.ListAutoDeployGitSourcesByRepository(ctx, pgtype.Int8{Int64: repositoryID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("store.ListAutoDeployGitSourcesByRepository: %w", err)
	}
	result := make([]domain.GitSource, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainGitSource(row))
	}
	return result, nil
}

func toDomainGitSource(row sqlc.AppGitSource) domain.GitSource {
	return domain.GitSource{
		AppID:                pgToUUID(row.AppID),
		Repository:           row.Repository,
		Branch:               row.Branch,
		BuildContext:         row.BuildContext,
		DockerfilePath:       row.DockerfilePath,
		AccessMode:           row.AccessMode,
		GitHubInstallationID: int8Pointer(row.GithubInstallationID),
		GitHubRepositoryID:   int8Pointer(row.GithubRepositoryID),
		Private:              row.Private,
		AutoDeploy:           row.AutoDeploy,
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
	}
}

func optionalInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

func int8Pointer(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}
