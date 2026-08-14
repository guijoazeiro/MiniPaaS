package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type AuditHandler struct {
	store store.AuditStore
	log   *slog.Logger
}

func NewAuditHandler(auditStore store.AuditStore, log *slog.Logger) *AuditHandler {
	return &AuditHandler{store: auditStore, log: log}
}

func (h *AuditHandler) List(c *gin.Context) {
	limit := queryInt(c, "limit", 50, 1, 200)
	offset := queryInt(c, "offset", 0, 0, 1_000_000)
	events, err := h.store.List(c.Request.Context(), limit, offset)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(200, gin.H{"items": events, "limit": limit, "offset": offset})
}
