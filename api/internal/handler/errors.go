package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

func respondError(c *gin.Context, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, domain.ErrAppNotFound), errors.Is(err, domain.ErrDeploymentNotFound), errors.Is(err, domain.ErrGitSourceNotFound), errors.Is(err, domain.ErrGitHubInstallationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrAppNameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrDeploymentActive), errors.Is(err, domain.ErrDeploymentNotRollbackable):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrAppNameInvalid), errors.Is(err, domain.ErrEnvKeyInvalid), errors.Is(err, domain.ErrGitRepositoryInvalid), errors.Is(err, domain.ErrGitRefInvalid), errors.Is(err, domain.ErrGitPathInvalid), errors.Is(err, domain.ErrDockerfileNotFound), errors.Is(err, domain.ErrGitHubInstallationInvalid), errors.Is(err, domain.ErrGitHubRepositoryNotAccessible):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrGitHubAppNotConfigured), errors.Is(err, domain.ErrGitHubWebhookNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrGitHubAutoDeployRequiresApp):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	default:
		log.Error("http handler", "path", c.Request.URL.Path, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
