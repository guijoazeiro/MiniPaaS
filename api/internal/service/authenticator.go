package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

// APITokenAuthenticator is the narrow contract the authentication layer needs
// from token persistence. Keeping it separate from token management prevents
// middleware from gaining access to token creation or revocation operations.
type APITokenAuthenticator interface {
	Authenticate(context.Context, string) (domain.APIToken, error)
}

// Authenticator combines the existing JWT session authenticator with opaque
// API-token authentication behind one principal-producing contract.
type Authenticator struct {
	sessions *AuthService
	tokens   APITokenAuthenticator
}

func NewAuthenticator(sessions *AuthService, tokens APITokenAuthenticator) *Authenticator {
	return &Authenticator{sessions: sessions, tokens: tokens}
}

func (a *Authenticator) Authenticate(ctx context.Context, raw string) (authctx.Identity, error) {
	if strings.HasPrefix(raw, domain.APITokenPrefix) {
		if a.tokens == nil {
			return authctx.Identity{}, domain.ErrUnauthorized
		}
		token, err := a.tokens.Authenticate(ctx, raw)
		if err != nil {
			if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrAPITokenNotFound) {
				return authctx.Identity{}, domain.ErrUnauthorized
			}
			return authctx.Identity{}, err
		}
		scopes := make(map[string]struct{}, len(token.Scopes))
		for _, scope := range token.Scopes {
			scopes[scope] = struct{}{}
		}
		return authctx.Identity{
			UserID:  token.UserID,
			Method:  authctx.AuthMethodAPIToken,
			TokenID: token.ID,
			Scopes:  scopes,
		}, nil
	}
	if a.sessions == nil {
		return authctx.Identity{}, domain.ErrUnauthorized
	}
	id, username, err := a.sessions.ParseToken(raw)
	if err != nil {
		return authctx.Identity{}, err
	}
	return authctx.Identity{UserID: id, Username: username, Method: authctx.AuthMethodSession}, nil
}

// ParseToken keeps Authenticator compatible with the middleware's original
// TokenParser contract and with callers that only need JWT-style fields.
func (a *Authenticator) ParseToken(raw string) (id uuid.UUID, username string, err error) {
	identity, err := a.Authenticate(context.Background(), raw)
	if err != nil {
		return uuid.Nil, "", err
	}
	// This method is only retained for source compatibility. Middleware uses
	// Authenticate so API-token scopes are never discarded.
	return identity.UserID, identity.Username, nil
}
