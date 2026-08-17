package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type APITokenService interface {
	Create(ctx context.Context, userID uuid.UUID, name string, scopes []string, expiresAt *time.Time) (domain.APITokenCreated, error)
	List(ctx context.Context, userID uuid.UUID) ([]domain.APIToken, error)
	Revoke(ctx context.Context, userID, tokenID uuid.UUID) error
}

type APITokenHandler struct {
	svc APITokenService
	log *slog.Logger
}

func NewAPITokenHandler(svc APITokenService, log *slog.Logger) *APITokenHandler {
	return &APITokenHandler{svc: svc, log: log}
}

type createAPITokenRequest struct {
	Name      string     `json:"name" binding:"required"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (h *APITokenHandler) Create(c *gin.Context) {
	userID, ok := requestUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	var req createAPITokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	created, err := h.svc.Create(c.Request.Context(), userID, req.Name, req.Scopes, req.ExpiresAt)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	// This is the only endpoint that returns Token. The list endpoint below
	// serializes domain.APIToken, which has no raw secret field.
	c.JSON(http.StatusCreated, created)
}

func (h *APITokenHandler) List(c *gin.Context) {
	userID, ok := requestUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	tokens, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if tokens == nil {
		tokens = []domain.APIToken{}
	}
	c.JSON(http.StatusOK, tokens)
}

func (h *APITokenHandler) Revoke(c *gin.Context) {
	userID, ok := requestUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	tokenID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": domain.ErrAPITokenNotFound.Error()})
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), userID, tokenID); err != nil {
		respondError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}
