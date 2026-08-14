package authctx

import (
	"context"

	"github.com/google/uuid"
)

type userIDKey struct{}

// WithUserID associates the authenticated user with a request context. The
// value is intentionally kept in a small shared package so middleware and
// persistence code can enforce ownership without importing one another.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}
