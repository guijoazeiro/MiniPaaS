package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type CapacityReader interface {
	Get(ctx context.Context) (domain.CapacitySnapshot, error)
}

type CapacityHandler struct {
	svc CapacityReader
	log *slog.Logger
}

func NewCapacityHandler(svc CapacityReader, log *slog.Logger) *CapacityHandler {
	return &CapacityHandler{svc: svc, log: log}
}

func (h *CapacityHandler) Get(c *gin.Context) {
	snapshot, err := h.svc.Get(c.Request.Context())
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, snapshot)
}
