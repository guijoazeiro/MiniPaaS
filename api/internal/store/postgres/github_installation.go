package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5"
)

type GitHubInstallationStore struct {
	q *sqlc.Queries
}

func NewGitHubInstallationStore(q *sqlc.Queries) *GitHubInstallationStore {
	return &GitHubInstallationStore{q: q}
}

func (s *GitHubInstallationStore) Upsert(ctx context.Context, installation domain.GitHubInstallation) (domain.GitHubInstallation, error) {
	row, err := s.q.UpsertGitHubInstallation(ctx, sqlc.UpsertGitHubInstallationParams{
		InstallationID:      installation.InstallationID,
		AccountLogin:        installation.AccountLogin,
		AccountType:         installation.AccountType,
		RepositorySelection: installation.RepositorySelection,
	})
	if err != nil {
		return domain.GitHubInstallation{}, fmt.Errorf("store.UpsertGitHubInstallation: %w", err)
	}
	return toDomainGitHubInstallation(row), nil
}

func (s *GitHubInstallationStore) Get(ctx context.Context, installationID int64) (domain.GitHubInstallation, error) {
	row, err := s.q.GetGitHubInstallation(ctx, installationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.GitHubInstallation{}, domain.ErrGitHubInstallationNotFound
		}
		return domain.GitHubInstallation{}, fmt.Errorf("store.GetGitHubInstallation: %w", err)
	}
	return toDomainGitHubInstallation(row), nil
}

func (s *GitHubInstallationStore) List(ctx context.Context) ([]domain.GitHubInstallation, error) {
	rows, err := s.q.ListGitHubInstallations(ctx)
	if err != nil {
		return nil, fmt.Errorf("store.ListGitHubInstallations: %w", err)
	}
	result := make([]domain.GitHubInstallation, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainGitHubInstallation(row))
	}
	return result, nil
}

func toDomainGitHubInstallation(row sqlc.GithubInstallation) domain.GitHubInstallation {
	return domain.GitHubInstallation{
		InstallationID:      row.InstallationID,
		AccountLogin:        row.AccountLogin,
		AccountType:         row.AccountType,
		RepositorySelection: row.RepositorySelection,
		CreatedAt:           row.CreatedAt.Time,
		UpdatedAt:           row.UpdatedAt.Time,
	}
}
