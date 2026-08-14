package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
)

type TokenParser interface {
	ParseToken(raw string) (uuid.UUID, string, error)
}

const (
	CtxUserID   = "user_id"
	CtxUsername = "username"
)

func Auth(p TokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		raw := strings.TrimPrefix(h, "Bearer ")
		if !strings.HasPrefix(h, "Bearer ") {
			var err error
			raw, err = c.Cookie("minipaas_token")
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
				return
			}
		}
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		id, name, err := p.ParseToken(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set(CtxUserID, id)
		c.Set(CtxUsername, name)
		c.Request = c.Request.WithContext(authctx.WithUserID(c.Request.Context(), id))
		c.Next()
	}
}
