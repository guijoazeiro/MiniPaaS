package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

const (
	apiTokenEntropyBytes = 32
	apiTokenNameMax      = 64
)

type APITokenService struct {
	tokens store.APITokenStore
}

func NewAPITokenService(tokens store.APITokenStore) *APITokenService {
	return &APITokenService{tokens: tokens}
}

// Create generates an opaque token with 256 bits of entropy. Only the returned
// raw value is suitable for authentication; the store receives its SHA-256
// digest and a display-only prefix.
func (s *APITokenService) Create(ctx context.Context, userID uuid.UUID, name string, scopes []string, expiresAt *time.Time) (domain.APITokenCreated, error) {
	if userID == uuid.Nil {
		return domain.APITokenCreated{}, domain.ErrUnauthorized
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > apiTokenNameMax {
		return domain.APITokenCreated{}, domain.ErrAPITokenNameInvalid
	}
	normalizedScopes, err := normalizeAPITokenScopes(scopes)
	if err != nil {
		return domain.APITokenCreated{}, err
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return domain.APITokenCreated{}, domain.ErrAPITokenExpiryInvalid
	}

	randomBytes := make([]byte, apiTokenEntropyBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return domain.APITokenCreated{}, fmt.Errorf("tokens.Create: random secret: %w", err)
	}
	raw := domain.APITokenPrefix + base64.RawURLEncoding.EncodeToString(randomBytes)
	hash := HashAPIToken(raw)
	// The prefix is only an operator-facing identifier. Authentication always
	// hashes and compares the complete secret.
	prefix := raw[:16]
	token, err := s.tokens.CreateAPIToken(ctx, userID, name, hash, prefix, normalizedScopes, expiresAt)
	if err != nil {
		return domain.APITokenCreated{}, fmt.Errorf("tokens.Create: persist: %w", err)
	}
	return domain.APITokenCreated{APIToken: token, Token: raw}, nil
}

func (s *APITokenService) List(ctx context.Context, userID uuid.UUID) ([]domain.APIToken, error) {
	if userID == uuid.Nil {
		return nil, domain.ErrUnauthorized
	}
	return s.tokens.ListAPITokens(ctx, userID)
}

func (s *APITokenService) Revoke(ctx context.Context, userID, tokenID uuid.UUID) error {
	if userID == uuid.Nil {
		return domain.ErrUnauthorized
	}
	if tokenID == uuid.Nil {
		return domain.ErrAPITokenNotFound
	}
	return s.tokens.RevokeAPIToken(ctx, userID, tokenID)
}

// Authenticate validates an API token without exposing its secret to the
// caller or to errors. It is used by the authentication middleware.
func (s *APITokenService) Authenticate(ctx context.Context, raw string) (domain.APIToken, error) {
	if !validRawAPIToken(raw) {
		return domain.APIToken{}, domain.ErrUnauthorized
	}
	token, err := s.tokens.GetAPITokenByHash(ctx, HashAPIToken(raw))
	if err != nil {
		if errors.Is(err, domain.ErrAPITokenNotFound) {
			return domain.APIToken{}, domain.ErrUnauthorized
		}
		return domain.APIToken{}, fmt.Errorf("tokens.Authenticate: lookup: %w", err)
	}
	if token.RevokedAt != nil || (token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now())) {
		return domain.APIToken{}, domain.ErrUnauthorized
	}
	// Usage metadata must not turn a valid credential into an outage. The
	// write is conditional in SQL, so revocation remains immediate.
	_ = s.tokens.TouchAPIToken(ctx, token.ID)
	return token, nil
}

func HashAPIToken(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

func validRawAPIToken(raw string) bool {
	if !strings.HasPrefix(raw, domain.APITokenPrefix) {
		return false
	}
	encoded := strings.TrimPrefix(raw, domain.APITokenPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	return err == nil && len(decoded) == apiTokenEntropyBytes
}

func normalizeAPITokenScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return []string{domain.APITokenScopeRead}, nil
	}
	seen := make(map[string]struct{}, len(scopes))
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if !domain.ValidAPITokenScope(scope) {
			return nil, domain.ErrAPITokenScopeInvalid
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		normalized = append(normalized, scope)
	}
	if len(normalized) == 0 {
		return nil, domain.ErrAPITokenScopeInvalid
	}
	return normalized, nil
}
