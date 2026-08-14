package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"github.com/jackc/pgx/v5"
)

type UserStore struct {
	q *sqlc.Queries
}

var _ store.UserStore = (*UserStore)(nil)

func NewUserStore(q *sqlc.Queries) *UserStore { return &UserStore{q: q} }

func (s *UserStore) Create(ctx context.Context, username, passwordHash string) (domain.User, error) {
	row, err := s.q.CreateUser(ctx, sqlc.CreateUserParams{
		Username:     username,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, domain.ErrUsernameTaken
		}
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

func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := s.q.GetUserByID(ctx, uuidToPG(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("store.GetUserByID: %w", err)
	}
	return toDomainUser(row), nil
}

func (s *UserStore) UpdateUsername(ctx context.Context, id uuid.UUID, username string) (domain.User, error) {
	row, err := s.q.UpdateUsername(ctx, sqlc.UpdateUsernameParams{ID: uuidToPG(id), Username: username})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, domain.ErrUsernameTaken
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("store.UpdateUsername: %w", err)
	}
	return toDomainUser(row), nil
}

func (s *UserStore) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	if err := s.q.UpdatePassword(ctx, sqlc.UpdatePasswordParams{ID: uuidToPG(id), PasswordHash: passwordHash}); err != nil {
		return fmt.Errorf("store.UpdatePassword: %w", err)
	}
	return nil
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
