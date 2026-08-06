package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type DeploymentService interface {
	Create(ctx context.Context, appName string) (domain.Deployment, domain.App, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Deployment, error)
	ListByApp(ctx context.Context, appID uuid.UUID, limit int32) ([]domain.Deployment, error)
	RunBuild(ctx context.Context, dep domain.Deployment, app domain.App, src io.Reader) error
}

type AppLookup interface {
	GetByName(ctx context.Context, name string) (domain.App, error)
}

type DeploymentHandler struct {
	svc  DeploymentService
	apps AppLookup
	log  *slog.Logger
}

func NewDeploymentHandler(svc DeploymentService, apps AppLookup, log *slog.Logger) *DeploymentHandler {
	return &DeploymentHandler{svc: svc, apps: apps, log: log}
}

func (h *DeploymentHandler) Create(c *gin.Context) {
	file, _, err := c.Request.FormFile("source")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart field 'source' required"})
		return
	}

	body, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read source: " + err.Error()})
		return
	}

	dep, app, err := h.svc.Create(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}

	go func() {
		buildCtx := context.WithoutCancel(c.Request.Context())
		if err := h.svc.RunBuild(buildCtx, dep, app, bytesReader(body)); err != nil {
			h.log.Error("build failed", "app", app.Name, "deployment", dep.ID, "err", err)
		}
	}()

	c.JSON(http.StatusAccepted, dep)
}

func (h *DeploymentHandler) List(c *gin.Context) {
	app, err := h.apps.GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	limit := int32(50)
	if q := c.Query("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 200 {
			limit = int32(n)
		}
	}
	deps, err := h.svc.ListByApp(c.Request.Context(), app.ID, limit)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if deps == nil {
		deps = []domain.Deployment{}
	}
	c.JSON(http.StatusOK, deps)
}

func (h *DeploymentHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}
	dep, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrDeploymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dep)
}
