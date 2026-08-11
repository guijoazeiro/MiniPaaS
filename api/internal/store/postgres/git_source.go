package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5"
)

type GitSourceStore struct {
	q *sqlc.Queries
}

func NewGitSourceStore(q *sqlc.Queries) *GitSourceStore {
	return &GitSourceStore{q: q}
}

func (s *GitSourceStore) Upsert(ctx context.Context, source domain.GitSource) (domain.GitSource, error) {
	row, err := s.q.UpsertGitSource(ctx, sqlc.UpsertGitSourceParams{
		AppID:          uuidToPG(source.AppID),
		Repository:     source.Repository,
		Branch:         source.Branch,
		BuildContext:   source.BuildContext,
		DockerfilePath: source.DockerfilePath,
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

func toDomainGitSource(row sqlc.AppGitSource) domain.GitSource {
	return domain.GitSource{
		AppID:          pgToUUID(row.AppID),
		Repository:     row.Repository,
		Branch:         row.Branch,
		BuildContext:   row.BuildContext,
		DockerfilePath: row.DockerfilePath,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}
