package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type DeploymentService interface {
	Create(ctx context.Context, appName string) (domain.Deployment, domain.App, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Deployment, error)
	ListByApp(ctx context.Context, appID uuid.UUID, limit int) ([]domain.Deployment, error)
	ListAll(ctx context.Context, appName, status string, page, perPage int) (domain.DeploymentPage, error)
	RunBuild(ctx context.Context, dep domain.Deployment, app domain.App, src io.Reader) error
	Rollback(ctx context.Context, appName string, targetID uuid.UUID, triggeredBy string) (domain.Deployment, error)
}

type DeploymentLogReader interface {
	List(ctx context.Context, deploymentID uuid.UUID, afterID int64, limit int) ([]domain.DeploymentLog, error)
}

func (h *DeploymentHandler) ListAll(c *gin.Context) {
	page := queryInt(c, "page", 1, 1, 100000)
	perPage := queryInt(c, "per_page", 50, 1, 200)
	result, err := h.svc.ListAll(c.Request.Context(), c.Query("app"), c.Query("status"), page, perPage)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func queryInt(c *gin.Context, key string, fallback, min, max int) int {
	value := fallback
	if raw := c.Query(key); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= min && parsed <= max {
			value = parsed
		}
	}
	return value
}

type AppLookup interface {
	GetByName(ctx context.Context, name string) (domain.App, error)
}

type DeploymentHandler struct {
	svc           DeploymentService
	apps          AppLookup
	log           *slog.Logger
	maxDeploySize int64
	logs          DeploymentLogReader
}

func NewDeploymentHandler(svc DeploymentService, apps AppLookup, log *slog.Logger, maxDeploySize int64, logs ...DeploymentLogReader) *DeploymentHandler {
	var reader DeploymentLogReader
	if len(logs) > 0 {
		reader = logs[0]
	}
	return &DeploymentHandler{svc: svc, apps: apps, log: log, maxDeploySize: maxDeploySize, logs: reader}
}

func (h *DeploymentHandler) Create(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxDeploySize)
	file, _, err := c.Request.FormFile("source")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "deployment source exceeds the configured size limit"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart field 'source' required"})
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "minipaas-build-*.tar")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "spool source: " + err.Error()})
		return
	}
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}
	if _, err := io.Copy(tmp, file); err != nil {
		cleanup()
		c.JSON(http.StatusBadRequest, gin.H{"error": "read source: " + err.Error()})
		return
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		cleanup()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	dep, app, err := h.svc.Create(c.Request.Context(), c.Param("name"))
	if err != nil {
		cleanup()
		respondError(c, h.log, err)
		return
	}

	go func() {
		defer cleanup()
		buildCtx := context.WithoutCancel(c.Request.Context())
		if err := h.svc.RunBuild(buildCtx, dep, app, tmp); err != nil {
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
	limit := 50
	if q := c.Query("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 200 {
			limit = n
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

type rollbackReq struct {
	DeploymentID string `json:"deployment_id" binding:"required"`
	TriggeredBy  string `json:"triggered_by"`
}

func (h *DeploymentHandler) Rollback(c *gin.Context) {
	var req rollbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	id, err := uuid.Parse(req.DeploymentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment_id"})
		return
	}
	triggeredBy := req.TriggeredBy
	if triggeredBy != "cli" && triggeredBy != "dashboard" {
		triggeredBy = "api"
	}
	restored, err := h.svc.Rollback(c.Request.Context(), c.Param("name"), id, triggeredBy)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, restored)
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

func (h *DeploymentHandler) Logs(c *gin.Context) {
	if h.logs == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "deployment logs unavailable"})
		return
	}
	app, err := h.apps.GetByName(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	depID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}
	dep, err := h.svc.Get(c.Request.Context(), depID)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if dep.AppID != app.ID {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}
	after := int64(0)
	if raw := c.Query("after"); raw != "" {
		if parsed, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil && parsed >= 0 {
			after = parsed
		}
	}
	limit := queryInt(c, "limit", 500, 1, 1000)
	logs, err := h.logs.List(c.Request.Context(), depID, after, limit)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if logs == nil {
		logs = []domain.DeploymentLog{}
	}
	c.JSON(http.StatusOK, logs)
}
