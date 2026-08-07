package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type AppService interface {
	Create(ctx context.Context, name string) (domain.App, error)
	GetByName(ctx context.Context, name string) (domain.App, error)
	List(ctx context.Context) ([]domain.App, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AppStopper interface {
	StopApp(ctx context.Context, app domain.App) error
}

type AppHandler struct {
	svc     AppService
	stopper AppStopper
	log     *slog.Logger
}

func NewAppHandler(svc AppService, stopper AppStopper, log *slog.Logger) *AppHandler {
	return &AppHandler{svc: svc, stopper: stopper, log: log}
}

type createAppReq struct {
	Name string `json:"name" binding:"required"`
}

func (h *AppHandler) Create(c *gin.Context) {
	var req createAppReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	app, err := h.svc.Create(c.Request.Context(), req.Name)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, app)
}

func (h *AppHandler) List(c *gin.Context) {
	apps, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if apps == nil {
		apps = []domain.App{}
	}
	c.JSON(http.StatusOK, apps)
}

func (h *AppHandler) Get(c *gin.Context) {
	app, err := h.svc.GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, app)
}

func (h *AppHandler) Delete(c *gin.Context) {
	app, err := h.svc.GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if err := h.stopper.StopApp(c.Request.Context(), app); err != nil {
		h.log.Warn("stop app", "app", app.Name, "err", err)
	}
	if err := h.svc.Delete(c.Request.Context(), app.ID); err != nil {
		respondError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}
