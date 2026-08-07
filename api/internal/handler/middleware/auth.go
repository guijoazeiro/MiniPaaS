package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		raw := strings.TrimPrefix(h, "Bearer ")
		id, name, err := p.ParseToken(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set(CtxUserID, id)
		c.Set(CtxUsername, name)
		c.Next()
	}
}
