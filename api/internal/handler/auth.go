package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/handler/middleware"
)

type AuthService interface {
	Login(ctx context.Context, username, password string) (token string, expiresAt time.Time, err error)
	Register(ctx context.Context, username, password string) (domain.User, error)
	Profile(ctx context.Context, userID uuid.UUID) (domain.User, error)
	UpdateUsername(ctx context.Context, userID uuid.UUID, username string) (domain.User, error)
	UpdatePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
	RefreshToken(ctx context.Context, userID uuid.UUID) (token string, expiresAt time.Time, err error)
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

type registerReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type updateUsernameReq struct {
	Username string `json:"username" binding:"required"`
}

type updatePasswordReq struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
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
	setSessionCookie(c, tok, exp)
	if includeToken {
		c.JSON(http.StatusOK, loginResp{Token: tok, ExpiresAt: exp})
		return
	}
	c.JSON(http.StatusOK, webLoginResp{ExpiresAt: exp})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	user, err := h.svc.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := requestUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	user, err := h.svc.Profile(c.Request.Context(), userID)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) UpdateMe(c *gin.Context) {
	userID, ok := requestUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	var req updateUsernameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	user, err := h.svc.UpdateUsername(c.Request.Context(), userID, req.Username)
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	if err := h.refreshCookie(c, userID); err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) UpdatePassword(c *gin.Context) {
	userID, ok := requestUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	var req updatePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := h.svc.UpdatePassword(c.Request.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		respondError(c, h.log, err)
		return
	}
	if err := h.refreshCookie(c, userID); err != nil {
		respondError(c, h.log, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) refreshCookie(c *gin.Context, userID uuid.UUID) error {
	token, expiresAt, err := h.svc.RefreshToken(c.Request.Context(), userID)
	if err != nil {
		return err
	}
	setSessionCookie(c, token, expiresAt)
	return nil
}

func requestUserID(c *gin.Context) (uuid.UUID, bool) {
	value, ok := c.Get(middleware.CtxUserID)
	id, valid := value.(uuid.UUID)
	return id, ok && valid && id != uuid.Nil
}

func setSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "minipaas_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
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
