package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type GitHubAppService interface {
	Enabled() bool
	InstallURL(ctx context.Context, appName string) (string, error)
	AccountInstallURL(ctx context.Context) (string, error)
	Connect(ctx context.Context, installationID int64, state string) (string, domain.GitHubInstallation, error)
	ListInstallations(ctx context.Context) ([]domain.GitHubInstallation, error)
	ListRepositories(ctx context.Context, installationID int64) ([]domain.GitHubRepository, error)
}

type GitHubAppHandler struct {
	service         GitHubAppService
	dashboardOrigin string
	log             *slog.Logger
	webhooksEnabled bool
}

func NewGitHubAppHandler(service GitHubAppService, dashboardOrigin string, log *slog.Logger, webhooksEnabled ...bool) *GitHubAppHandler {
	handler := &GitHubAppHandler{service: service, dashboardOrigin: strings.TrimRight(dashboardOrigin, "/"), log: log}
	if len(webhooksEnabled) > 0 {
		handler.webhooksEnabled = webhooksEnabled[0]
	}
	return handler
}

func (h *GitHubAppHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": h.service != nil && h.service.Enabled(), "webhooks_enabled": h.webhooksEnabled})
}

func (h *GitHubAppHandler) InstallURL(c *gin.Context) {
	appName := strings.TrimSpace(c.Query("app"))
	if appName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "app is required"})
		return
	}
	installURL, err := h.service.InstallURL(c.Request.Context(), appName)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": installURL})
}

func (h *GitHubAppHandler) AccountInstallURL(c *gin.Context) {
	installURL, err := h.service.AccountInstallURL(c.Request.Context())
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": installURL})
}

func (h *GitHubAppHandler) Callback(c *gin.Context) {
	installationID, err := strconv.ParseInt(c.Query("installation_id"), 10, 64)
	if err != nil || installationID <= 0 || c.Query("state") == "" {
		respondError(c, h.log, domain.ErrGitHubInstallationInvalid)
		return
	}
	appName, _, err := h.service.Connect(c.Request.Context(), installationID, c.Query("state"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	destination := h.dashboardOrigin + "/dashboard/account?github=connected"
	if appName != "" {
		destination = h.dashboardOrigin + "/dashboard/projects/" + url.PathEscape(appName) + "?tab=settings&github=connected"
	}
	c.Redirect(http.StatusFound, destination)
}

func (h *GitHubAppHandler) ListInstallations(c *gin.Context) {
	installations, err := h.service.ListInstallations(c.Request.Context())
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, installations)
}

func (h *GitHubAppHandler) ListRepositories(c *gin.Context) {
	installationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || installationID <= 0 {
		respondError(c, h.log, domain.ErrGitHubInstallationInvalid)
		return
	}
	repositories, err := h.service.ListRepositories(c.Request.Context(), installationID)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, repositories)
}
