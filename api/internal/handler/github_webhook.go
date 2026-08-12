package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/githubapp"
	"github.com/guijoazeiro/MiniPaaS/api/internal/service"
)

type GitHubWebhookProcessor interface {
	Handle(ctx context.Context, event, deliveryID string, payload []byte) (service.GitHubWebhookResult, error)
}

type GitHubWebhookHandler struct {
	secret    string
	processor GitHubWebhookProcessor
	log       *slog.Logger
}

func NewGitHubWebhookHandler(secret string, processor GitHubWebhookProcessor, log *slog.Logger) *GitHubWebhookHandler {
	return &GitHubWebhookHandler{secret: secret, processor: processor, log: log}
}

func (h *GitHubWebhookHandler) Enabled() bool { return h != nil && h.secret != "" }

func (h *GitHubWebhookHandler) Handle(c *gin.Context) {
	if !h.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": domain.ErrGitHubWebhookNotConfigured.Error()})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook body"})
		return
	}
	if err := githubapp.VerifyWebhookSignature(h.secret, payload, c.GetHeader("X-Hub-Signature-256")); err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, domain.ErrGitHubWebhookNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	result, err := h.processor.Handle(c.Request.Context(), c.GetHeader("X-GitHub-Event"), c.GetHeader("X-GitHub-Delivery"), payload)
	if err != nil {
		h.log.Error("process GitHub webhook", "delivery", c.GetHeader("X-GitHub-Delivery"), "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "webhook processing failed"})
		return
	}
	c.JSON(http.StatusAccepted, result)
}
