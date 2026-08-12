package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type GitSourceService interface {
	Configure(ctx context.Context, appName string, source domain.GitSource) (domain.GitSource, error)
	Get(ctx context.Context, appName string) (domain.GitSource, error)
	Delete(ctx context.Context, appName string) error
}

type GitHubAppSourceService interface {
	ConfigureGitHubApp(ctx context.Context, appName string, installationID, repositoryID int64, branch, buildContext, dockerfilePath string) (domain.GitSource, error)
}

type gitHubAppSourceRequest struct {
	InstallationID int64  `json:"installation_id" binding:"required"`
	RepositoryID   int64  `json:"repository_id" binding:"required"`
	Branch         string `json:"branch"`
	BuildContext   string `json:"build_context"`
	DockerfilePath string `json:"dockerfile_path"`
}

func (h *GitSourceHandler) ConfigureGitHubApp(c *gin.Context) {
	var req gitHubAppSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.InstallationID <= 0 || req.RepositoryID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "installation_id and repository_id are required"})
		return
	}
	service, ok := h.sources.(GitHubAppSourceService)
	if !ok {
		respondError(c, h.log, domain.ErrGitHubAppNotConfigured)
		return
	}
	source, err := service.ConfigureGitHubApp(c.Request.Context(), c.Param("name"), req.InstallationID, req.RepositoryID, req.Branch, req.BuildContext, req.DockerfilePath)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, source)
}

type GitDeploymentService interface {
	Create(ctx context.Context, appName, branch string) (domain.Deployment, domain.App, domain.GitSource, error)
	Run(ctx context.Context, dep domain.Deployment, app domain.App, source domain.GitSource, branch string) error
}

type GitSourceHandler struct {
	sources     GitSourceService
	deployments GitDeploymentService
	log         *slog.Logger
}

func NewGitSourceHandler(sources GitSourceService, deployments GitDeploymentService, log *slog.Logger) *GitSourceHandler {
	return &GitSourceHandler{sources: sources, deployments: deployments, log: log}
}

type gitSourceRequest struct {
	Repository     string `json:"repository" binding:"required"`
	Branch         string `json:"branch"`
	BuildContext   string `json:"build_context"`
	DockerfilePath string `json:"dockerfile_path"`
}

func (h *GitSourceHandler) Configure(c *gin.Context) {
	var req gitSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repository is required"})
		return
	}
	source, err := h.sources.Configure(c.Request.Context(), c.Param("name"), domain.GitSource{
		Repository: req.Repository, Branch: req.Branch, BuildContext: req.BuildContext, DockerfilePath: req.DockerfilePath,
	})
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, source)
}

func (h *GitSourceHandler) Get(c *gin.Context) {
	source, err := h.sources.Get(c.Request.Context(), c.Param("name"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, source)
}

func (h *GitSourceHandler) Delete(c *gin.Context) {
	if err := h.sources.Delete(c.Request.Context(), c.Param("name")); err != nil {
		respondError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type gitDeployRequest struct {
	Branch string `json:"branch"`
}

func (h *GitSourceHandler) Deploy(c *gin.Context) {
	var req gitDeployRequest
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
			return
		}
	}
	dep, app, source, err := h.deployments.Create(c.Request.Context(), c.Param("name"), req.Branch)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	go func() {
		buildCtx := context.WithoutCancel(c.Request.Context())
		if err := h.deployments.Run(buildCtx, dep, app, source, req.Branch); err != nil {
			h.log.Error("git build failed", "app", app.Name, "deployment", dep.ID, "repository", source.Repository, "err", err)
		}
	}()
	c.JSON(http.StatusAccepted, dep)
}
