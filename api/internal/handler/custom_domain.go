package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type CustomDomainService interface {
	List(ctx context.Context, appName string) ([]domain.CustomDomain, error)
	Create(ctx context.Context, appName, hostname string) (domain.CustomDomain, error)
	Verify(ctx context.Context, appName string, id uuid.UUID) (domain.CustomDomain, error)
	Delete(ctx context.Context, appName string, id uuid.UUID) error
}

type CustomDomainHandler struct {
	service CustomDomainService
	log     *slog.Logger
}

func NewCustomDomainHandler(service CustomDomainService, log *slog.Logger) *CustomDomainHandler {
	return &CustomDomainHandler{service: service, log: log}
}

type createCustomDomainRequest struct {
	Hostname string `json:"hostname" binding:"required"`
}

func (h *CustomDomainHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if items == nil {
		items = []domain.CustomDomain{}
	}
	c.JSON(http.StatusOK, items)
}

func (h *CustomDomainHandler) Create(c *gin.Context) {
	var req createCustomDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hostname is required"})
		return
	}
	item, err := h.service.Create(c.Request.Context(), c.Param("name"), req.Hostname)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *CustomDomainHandler) Verify(c *gin.Context) {
	id, err := uuid.Parse(c.Param("domainID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
		return
	}
	item, err := h.service.Verify(c.Request.Context(), c.Param("name"), id)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *CustomDomainHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("domainID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.Param("name"), id); err != nil {
		respondError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}
