package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type GitHubWebhookDeliveryStore struct{ q *sqlc.Queries }

func NewGitHubWebhookDeliveryStore(q *sqlc.Queries) *GitHubWebhookDeliveryStore {
	return &GitHubWebhookDeliveryStore{q: q}
}

func (s *GitHubWebhookDeliveryStore) Claim(ctx context.Context, deliveryID, event string, repositoryID int64, commitSHA string) (bool, error) {
	_, err := s.q.ClaimGitHubWebhookDelivery(ctx, sqlc.ClaimGitHubWebhookDeliveryParams{
		DeliveryID: deliveryID, Event: event,
		RepositoryID: pgtype.Int8{Int64: repositoryID, Valid: repositoryID > 0},
		CommitSha:    pgtype.Text{String: commitSHA, Valid: commitSHA != ""},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store.ClaimGitHubWebhookDelivery: %w", err)
	}
	return true, nil
}

func (s *GitHubWebhookDeliveryStore) Complete(ctx context.Context, deliveryID, status, errorMessage string) error {
	if err := s.q.CompleteGitHubWebhookDelivery(ctx, sqlc.CompleteGitHubWebhookDeliveryParams{
		DeliveryID: deliveryID, Status: status,
		ErrorMessage: pgtype.Text{String: errorMessage, Valid: errorMessage != ""},
	}); err != nil {
		return fmt.Errorf("store.CompleteGitHubWebhookDelivery: %w", err)
	}
	return nil
}
