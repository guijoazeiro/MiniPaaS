package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

// Audit records mutating HTTP requests without copying request bodies. This
// keeps credentials and environment values out of the audit trail while still
// providing a durable record of who changed platform state and when.
func Audit(recorder store.AuditStore, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if recorder == nil || !isMutation(c.Request.Method) || strings.HasPrefix(c.Request.URL.Path, "/health") || strings.HasPrefix(c.Request.URL.Path, "/ready") {
			return
		}

		event := store.AuditEvent{
			Action:    strings.ToLower(c.Request.Method) + " " + c.FullPath(),
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Status:    c.Writer.Status(),
			RequestID: RequestIDValue(c),
		}
		if value, ok := c.Get(CtxUserID); ok {
			if id, ok := value.(uuid.UUID); ok {
				event.UserID = id
			}
		}

		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 2*time.Second)
		defer cancel()
		if err := recorder.Record(ctx, event); err != nil && log != nil {
			log.Warn("persist audit event", "request_id", event.RequestID, "action", event.Action, "err", err)
		}
	}
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
