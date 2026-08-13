package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type MetricsService interface {
	Get(ctx context.Context, appName string) (domain.AppMetrics, error)
}

type MetricsHandler struct {
	service MetricsService
	log     *slog.Logger
}

func NewMetricsHandler(service MetricsService, log *slog.Logger) *MetricsHandler {
	return &MetricsHandler{service: service, log: log}
}

func (h *MetricsHandler) Get(c *gin.Context) {
	metrics, err := h.service.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, metrics)
}
