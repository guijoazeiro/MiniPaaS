package authctx

import (
	"context"

	"github.com/google/uuid"
)

type userIDKey struct{}
type identityKey struct{}

// AuthMethod identifies how a request was authenticated.
type AuthMethod string

const (
	AuthMethodSession  AuthMethod = "session"
	AuthMethodAPIToken AuthMethod = "api_token"
)

// Identity is the authenticated principal attached to a request. Scopes are
// only meaningful for API tokens; session identities retain the existing full
// dashboard/CLI permissions for backwards compatibility.
type Identity struct {
	UserID   uuid.UUID
	Username string
	Method   AuthMethod
	TokenID  uuid.UUID
	Scopes   map[string]struct{}
}

func (i Identity) HasScope(scope string) bool {
	if i.Method == AuthMethodSession {
		return true
	}
	_, ok := i.Scopes[scope]
	return ok
}

// WithIdentity associates a copy of an authenticated identity with ctx. The
// scope map is copied so request handlers cannot mutate the principal owned by
// authentication middleware.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	if identity.Scopes != nil {
		copyScopes := make(map[string]struct{}, len(identity.Scopes))
		for scope := range identity.Scopes {
			copyScopes[scope] = struct{}{}
		}
		identity.Scopes = copyScopes
	}
	return context.WithValue(ctx, identityKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	if !ok || identity.UserID == uuid.Nil {
		return Identity{}, false
	}
	return identity, true
}

// WithUserID associates the authenticated user with a request context. The
// value is intentionally kept in a small shared package so middleware and
// persistence code can enforce ownership without importing one another.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

func UserID(ctx context.Context) (uuid.UUID, bool) {
	if identity, ok := IdentityFromContext(ctx); ok {
		return identity.UserID, true
	}
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}
