package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeAuthService struct{}

func (fakeAuthService) Login(context.Context, string, string) (string, time.Time, error) {
	return "test-token", time.Now().Add(time.Hour), nil
}

func TestWebLoginSetsSecureCookieWithoutReturningToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewAuthHandler(fakeAuthService{}, slog.Default())
	router.POST("/auth/web-login", h.WebLogin)

	req := httptest.NewRequest(http.MethodPost, "/auth/web-login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if strings.Contains(resp.Body.String(), "token") {
		t.Fatalf("web login must not expose token: %s", resp.Body.String())
	}
	cookie := resp.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "Secure") {
		t.Fatalf("expected secure HTTP-only cookie, got %q", cookie)
	}
}

func TestLogoutExpiresSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewAuthHandler(fakeAuthService{}, slog.Default())
	router.POST("/auth/logout", h.Logout)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNoContent)
	}
	if !strings.Contains(resp.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("logout did not expire cookie: %q", resp.Header().Get("Set-Cookie"))
	}
}
