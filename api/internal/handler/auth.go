package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type AuthService interface {
	Login(ctx context.Context, username, password string) (token string, expiresAt time.Time, err error)
}

type AuthHandler struct {
	svc AuthService
	log *slog.Logger
}

func NewAuthHandler(svc AuthService, log *slog.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log}
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResp struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	tok, exp, err := h.svc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, loginResp{Token: tok, ExpiresAt: exp})
}
