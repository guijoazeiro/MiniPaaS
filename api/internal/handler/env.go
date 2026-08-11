package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type EnvService interface {
	Set(ctx context.Context, appID uuid.UUID, key, value string) error
	List(ctx context.Context, appID uuid.UUID) ([]domain.EnvVarKey, error)
	Delete(ctx context.Context, appID uuid.UUID, key string) error
}

type EnvHandler struct {
	svc  EnvService
	apps AppLookup
	log  *slog.Logger
}

func NewEnvHandler(svc EnvService, apps AppLookup, log *slog.Logger) *EnvHandler {
	return &EnvHandler{svc: svc, apps: apps, log: log}
}

func (h *EnvHandler) List(c *gin.Context) {
	app, err := h.apps.GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	keys, err := h.svc.List(c.Request.Context(), app.ID)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if keys == nil {
		keys = []domain.EnvVarKey{}
	}
	c.JSON(http.StatusOK, keys)
}

type setEnvReq struct {
	Value *string `json:"value" binding:"required"`
}

func (h *EnvHandler) Set(c *gin.Context) {
	app, err := h.apps.GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	var req setEnvReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := h.svc.Set(c.Request.Context(), app.ID, c.Param("key"), *req.Value); err != nil {
		respondError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *EnvHandler) Delete(c *gin.Context) {
	app, err := h.apps.GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), app.ID, c.Param("key")); err != nil {
		respondError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}
