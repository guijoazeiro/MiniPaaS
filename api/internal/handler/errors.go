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
	case errors.Is(err, domain.ErrAppNotFound), errors.Is(err, domain.ErrDeploymentNotFound), errors.Is(err, domain.ErrGitSourceNotFound), errors.Is(err, domain.ErrGitHubInstallationNotFound), errors.Is(err, domain.ErrCustomDomainNotFound), errors.Is(err, domain.ErrUserNotFound), errors.Is(err, domain.ErrAPITokenNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrAppNameTaken), errors.Is(err, domain.ErrCustomDomainTaken), errors.Is(err, domain.ErrUsernameTaken), errors.Is(err, domain.ErrGitHubInstallationOwned):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrAppCapacityExceeded):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrDeploymentActive), errors.Is(err, domain.ErrDeploymentNotRollbackable), errors.Is(err, domain.ErrDeploymentNotCancellable), errors.Is(err, domain.ErrDeploymentNotRetryable), errors.Is(err, domain.ErrDeploymentRetryUnavailable):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrAppNameInvalid), errors.Is(err, domain.ErrUsernameInvalid), errors.Is(err, domain.ErrPasswordWeak), errors.Is(err, domain.ErrEnvKeyInvalid), errors.Is(err, domain.ErrGitRepositoryInvalid), errors.Is(err, domain.ErrGitRefInvalid), errors.Is(err, domain.ErrGitPathInvalid), errors.Is(err, domain.ErrDockerfileNotFound), errors.Is(err, domain.ErrGitHubInstallationInvalid), errors.Is(err, domain.ErrGitHubRepositoryNotAccessible), errors.Is(err, domain.ErrCustomDomainInvalid), errors.Is(err, domain.ErrAPITokenNameInvalid), errors.Is(err, domain.ErrAPITokenScopeInvalid), errors.Is(err, domain.ErrAPITokenExpiryInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrCustomDomainDNSNotConfigured), errors.Is(err, domain.ErrCustomDomainNotVerified):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
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
