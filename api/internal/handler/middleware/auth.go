package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
)

type TokenParser interface {
	ParseToken(raw string) (uuid.UUID, string, error)
}

type IdentityParser interface {
	Authenticate(context.Context, string) (authctx.Identity, error)
}

const (
	CtxUserID   = "user_id"
	CtxUsername = "username"
	CtxIdentity = "identity"
)

func Auth(p TokenParser) gin.HandlerFunc {
	identityParser, hasIdentityParser := p.(IdentityParser)
	return func(c *gin.Context) {
		raw, ok := credential(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		var identity authctx.Identity
		var err error
		if hasIdentityParser {
			identity, err = identityParser.Authenticate(c.Request.Context(), raw)
		} else {
			var id uuid.UUID
			var name string
			id, name, err = p.ParseToken(raw)
			identity = authctx.Identity{UserID: id, Username: name, Method: authctx.AuthMethodSession}
		}
		if err != nil || identity.UserID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set(CtxUserID, identity.UserID)
		c.Set(CtxUsername, identity.Username)
		c.Set(CtxIdentity, identity)
		c.Request = c.Request.WithContext(authctx.WithIdentity(c.Request.Context(), identity))
		c.Next()
	}
}

func credential(c *gin.Context) (string, bool) {
	if header := strings.TrimSpace(c.GetHeader("Authorization")); header != "" {
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			return "", false
		}
		return parts[1], true
	}
	raw, err := c.Cookie("minipaas_token")
	raw = strings.TrimSpace(raw)
	return raw, err == nil && raw != ""
}

// RequireScope applies an explicit authorization policy to a route. Session
// identities retain full access for backwards compatibility; API tokens must
// carry the requested scope.
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := authctx.IdentityFromContext(c.Request.Context())
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if identity.HasScope(scope) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient scope"})
	}
}

// RequireSession protects account-management and OAuth callback flows that
// must not be performed with an automation credential.
func RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := authctx.IdentityFromContext(c.Request.Context())
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if identity.Method != authctx.AuthMethodSession {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "session authentication required"})
			return
		}
		c.Next()
	}
}
