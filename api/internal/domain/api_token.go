package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	APITokenPrefix      = "mpat_"
	APITokenScopeRead   = "read"
	APITokenScopeDeploy = "deploy"
	APITokenScopeManage = "manage"
)

// APIToken is the safe, persistable representation of an automation token.
// The raw secret and its hash are intentionally not part of this type.
type APIToken struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"-"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	Scopes      []string   `json:"scopes"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// APITokenCreated is returned only by the create operation. Token contains
// the raw secret and must never be persisted or returned by another endpoint.
type APITokenCreated struct {
	APIToken
	Token string `json:"token"`
}

func ValidAPITokenScope(scope string) bool {
	switch scope {
	case APITokenScopeRead, APITokenScopeDeploy, APITokenScopeManage:
		return true
	default:
		return false
	}
}
