package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const CtxRequestID = "request_id"

// RequestID assigns a canonical UUID to every request. A valid incoming UUID
// is preserved for cross-service correlation; malformed values are replaced so
// they cannot inject arbitrary data into logs.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New()
		if incoming, err := uuid.Parse(c.GetHeader("X-Request-ID")); err == nil {
			requestID = incoming
		}
		value := requestID.String()
		c.Set(CtxRequestID, value)
		c.Header("X-Request-ID", value)
		c.Next()
	}
}

func RequestIDValue(c *gin.Context) string {
	if value, ok := c.Get(CtxRequestID); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}
