package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type apiTokenAuthenticatorStub struct {
	token domain.APIToken
}

func (s apiTokenAuthenticatorStub) Authenticate(context.Context, string) (domain.APIToken, error) {
	return s.token, nil
}

func TestAuthenticatorBuildsAPIIdentityWithScopes(t *testing.T) {
	userID := uuid.New()
	tokenID := uuid.New()
	auth := NewAuthenticator(nil, apiTokenAuthenticatorStub{token: domain.APIToken{
		ID: tokenID, UserID: userID, Scopes: []string{domain.APITokenScopeRead, domain.APITokenScopeDeploy},
	}})

	identity, err := auth.Authenticate(context.Background(), domain.APITokenPrefix+"opaque")
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != userID || identity.TokenID != tokenID || identity.Method != authctx.AuthMethodAPIToken {
		t.Fatalf("identity = %+v", identity)
	}
	if !identity.HasScope(domain.APITokenScopeRead) || !identity.HasScope(domain.APITokenScopeDeploy) || identity.HasScope(domain.APITokenScopeManage) {
		t.Fatalf("unexpected scopes = %#v", identity.Scopes)
	}
}
