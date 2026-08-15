package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
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
	params := sqlc.UpsertGitHubInstallationParams{
		InstallationID:      installation.InstallationID,
		AccountLogin:        installation.AccountLogin,
		AccountType:         installation.AccountType,
		RepositorySelection: installation.RepositorySelection,
	}
	if ownerID, ok := authctx.UserID(ctx); ok {
		params.OwnerUserID = uuidToPG(ownerID)
	}
	row, err := s.q.UpsertGitHubInstallation(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.GitHubInstallation{}, domain.ErrGitHubInstallationOwned
		}
		return domain.GitHubInstallation{}, fmt.Errorf("store.UpsertGitHubInstallation: %w", err)
	}
	return toDomainGitHubInstallation(row), nil
}

func (s *GitHubInstallationStore) Get(ctx context.Context, installationID int64) (domain.GitHubInstallation, error) {
	var row sqlc.GithubInstallation
	var err error
	if ownerID, ok := authctx.UserID(ctx); ok {
		row, err = s.q.GetGitHubInstallationForUser(ctx, sqlc.GetGitHubInstallationForUserParams{InstallationID: installationID, OwnerUserID: uuidToPG(ownerID)})
	} else {
		row, err = s.q.GetGitHubInstallation(ctx, installationID)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.GitHubInstallation{}, domain.ErrGitHubInstallationNotFound
		}
		return domain.GitHubInstallation{}, fmt.Errorf("store.GetGitHubInstallation: %w", err)
	}
	return toDomainGitHubInstallation(row), nil
}

func (s *GitHubInstallationStore) List(ctx context.Context) ([]domain.GitHubInstallation, error) {
	var rows []sqlc.GithubInstallation
	var err error
	if ownerID, ok := authctx.UserID(ctx); ok {
		rows, err = s.q.ListGitHubInstallationsForUser(ctx, uuidToPG(ownerID))
	} else {
		rows, err = s.q.ListGitHubInstallations(ctx)
	}
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
