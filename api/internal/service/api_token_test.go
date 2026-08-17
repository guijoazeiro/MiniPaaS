package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type fakeAPITokenStore struct {
	token       domain.APIToken
	createHash  string
	create      domain.APIToken
	touchCount  int
	revokeUser  uuid.UUID
	revokeToken uuid.UUID
}

func (f *fakeAPITokenStore) CreateAPIToken(_ context.Context, _ uuid.UUID, name, tokenHash, tokenPrefix string, scopes []string, expiresAt *time.Time) (domain.APIToken, error) {
	f.createHash = tokenHash
	f.create = domain.APIToken{ID: uuid.New(), Name: name, TokenPrefix: tokenPrefix, Scopes: scopes, ExpiresAt: expiresAt, CreatedAt: time.Now()}
	f.token = f.create
	return f.create, nil
}
func (f *fakeAPITokenStore) ListAPITokens(context.Context, uuid.UUID) ([]domain.APIToken, error) {
	if f.token.ID == uuid.Nil {
		return []domain.APIToken{}, nil
	}
	return []domain.APIToken{f.token}, nil
}
func (f *fakeAPITokenStore) GetAPITokenByHash(_ context.Context, hash string) (domain.APIToken, error) {
	if hash != f.createHash || f.token.ID == uuid.Nil {
		return domain.APIToken{}, domain.ErrAPITokenNotFound
	}
	return f.token, nil
}
func (f *fakeAPITokenStore) RevokeAPIToken(_ context.Context, userID, tokenID uuid.UUID) error {
	f.revokeUser, f.revokeToken = userID, tokenID
	return nil
}
func (f *fakeAPITokenStore) TouchAPIToken(_ context.Context, _ uuid.UUID) error {
	f.touchCount++
	return nil
}

func TestAPITokenCreateUsesOpaque256BitSecret(t *testing.T) {
	store := &fakeAPITokenStore{}
	svc := NewAPITokenService(store)
	userID := uuid.New()
	created, err := svc.Create(context.Background(), userID, "CI", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Token, domain.APITokenPrefix) {
		t.Fatalf("token %q does not use mpat_ prefix", created.Token)
	}
	if len(created.Token) != len(domain.APITokenPrefix)+43 {
		t.Fatalf("token length = %d, want %d", len(created.Token), len(domain.APITokenPrefix)+43)
	}
	if created.APIToken.TokenPrefix != created.Token[:16] {
		t.Fatalf("token prefix = %q, want %q", created.APIToken.TokenPrefix, created.Token[:16])
	}
	if store.createHash == created.Token || len(store.createHash) != 64 {
		t.Fatal("store received raw token or an unexpected hash")
	}
	if created.Scopes == nil || len(created.Scopes) != 1 || created.Scopes[0] != domain.APITokenScopeRead {
		t.Fatalf("default scopes = %#v, want [read]", created.Scopes)
	}
}

func TestAPITokenAuthenticateUpdatesLastUsed(t *testing.T) {
	store := &fakeAPITokenStore{}
	svc := NewAPITokenService(store)
	created, err := svc.Create(context.Background(), uuid.New(), "automation", []string{domain.APITokenScopeDeploy, domain.APITokenScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Authenticate(context.Background(), created.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || store.touchCount != 1 {
		t.Fatalf("authenticated token = %v, touches = %d", got.ID, store.touchCount)
	}
}

func TestAPITokenAuthenticateRejectsInvalidExpiredAndRevoked(t *testing.T) {
	store := &fakeAPITokenStore{}
	svc := NewAPITokenService(store)
	created, err := svc.Create(context.Background(), uuid.New(), "automation", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(context.Background(), created.Token+"x"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("invalid token error = %v", err)
	}
	expired := time.Now().Add(-time.Minute)
	store.token.ExpiresAt = &expired
	if _, err := svc.Authenticate(context.Background(), created.Token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expired token error = %v", err)
	}
	store.token.ExpiresAt = nil
	revoked := time.Now()
	store.token.RevokedAt = &revoked
	if _, err := svc.Authenticate(context.Background(), created.Token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("revoked token error = %v", err)
	}
}

func TestAPITokenCreateValidatesNameScopesAndExpiry(t *testing.T) {
	svc := NewAPITokenService(&fakeAPITokenStore{})
	userID := uuid.New()
	if _, err := svc.Create(context.Background(), userID, "", nil, nil); !errors.Is(err, domain.ErrAPITokenNameInvalid) {
		t.Fatalf("empty name error = %v", err)
	}
	if _, err := svc.Create(context.Background(), userID, "ci", []string{"admin"}, nil); !errors.Is(err, domain.ErrAPITokenScopeInvalid) {
		t.Fatalf("invalid scope error = %v", err)
	}
	expired := time.Now().Add(-time.Second)
	if _, err := svc.Create(context.Background(), userID, "ci", nil, &expired); !errors.Is(err, domain.ErrAPITokenExpiryInvalid) {
		t.Fatalf("expired-at creation error = %v", err)
	}
}
