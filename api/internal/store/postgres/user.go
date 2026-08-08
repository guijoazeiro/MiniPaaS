package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5"
)

type UserStore struct {
	q *sqlc.Queries
}

func NewUserStore(q *sqlc.Queries) *UserStore { return &UserStore{q: q} }

func (s *UserStore) Create(ctx context.Context, username, passwordHash string) (domain.User, error) {
	row, err := s.q.CreateUser(ctx, sqlc.CreateUserParams{
		Username:     username,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return domain.User{}, fmt.Errorf("store.CreateUser: %w", err)
	}
	return toDomainUser(row), nil
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	row, err := s.q.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrInvalidCredentials
		}
		return domain.User{}, fmt.Errorf("store.GetUserByUsername: %w", err)
	}
	return toDomainUser(row), nil
}

func (s *UserStore) Count(ctx context.Context) (int64, error) {
	n, err := s.q.CountUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("store.CountUsers: %w", err)
	}
	return n, nil
}

func toDomainUser(row sqlc.User) domain.User {
	return domain.User{
		ID:           pgToUUID(row.ID),
		Username:     row.Username,
		PasswordHash: row.PasswordHash,
		CreatedAt:    row.CreatedAt.Time,
	}
}
