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

type webLoginResp struct {
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	h.login(c, true)
}

// WebLogin starts a browser session without exposing the bearer token to JavaScript.
func (h *AuthHandler) WebLogin(c *gin.Context) {
	h.login(c, false)
}

func (h *AuthHandler) login(c *gin.Context, includeToken bool) {
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
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "minipaas_token",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
	})
	if includeToken {
		c.JSON(http.StatusOK, loginResp{Token: tok, ExpiresAt: exp})
		return
	}
	c.JSON(http.StatusOK, webLoginResp{ExpiresAt: exp})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "minipaas_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
	c.Status(http.StatusNoContent)
}
