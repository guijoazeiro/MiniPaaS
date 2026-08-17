package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
)

type parserStub struct {
	identity authctx.Identity
	err      error
	calls    int
}

func (p *parserStub) ParseToken(string) (uuid.UUID, string, error) {
	p.calls++
	return p.identity.UserID, p.identity.Username, p.err
}

func (p *parserStub) Authenticate(context.Context, string) (authctx.Identity, error) {
	p.calls++
	return p.identity, p.err
}

func TestAuthRejectsInvalidAuthorizationWithoutCookieFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	parser := &parserStub{identity: authctx.Identity{UserID: uuid.New(), Method: authctx.AuthMethodSession}}
	router := gin.New()
	router.GET("/", Auth(parser), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic credentials")
	req.AddCookie(&http.Cookie{Name: "minipaas_token", Value: "valid-cookie"})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
	if parser.calls != 0 {
		t.Fatalf("parser calls = %d, want 0", parser.calls)
	}
}

func TestAuthAttachesAPITokenIdentityAndScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	parser := &parserStub{identity: authctx.Identity{
		UserID: userID, Method: authctx.AuthMethodAPIToken,
		Scopes: map[string]struct{}{"read": {}},
	}}
	router := gin.New()
	router.GET("/read", Auth(parser), RequireScope("read"), func(c *gin.Context) {
		identity, ok := authctx.IdentityFromContext(c.Request.Context())
		if !ok || identity.UserID != userID || identity.Method != authctx.AuthMethodAPIToken {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.POST("/manage", Auth(parser), RequireScope("manage"), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	readReq := httptest.NewRequest(http.MethodGet, "/read", nil)
	readReq.Header.Set("Authorization", "Bearer mpat_test")
	readResp := httptest.NewRecorder()
	router.ServeHTTP(readResp, readReq)
	if readResp.Code != http.StatusNoContent {
		t.Fatalf("read status = %d, want %d", readResp.Code, http.StatusNoContent)
	}

	manageReq := httptest.NewRequest(http.MethodPost, "/manage", nil)
	manageReq.Header.Set("Authorization", "Bearer mpat_test")
	manageResp := httptest.NewRecorder()
	router.ServeHTTP(manageResp, manageReq)
	if manageResp.Code != http.StatusForbidden {
		t.Fatalf("manage status = %d, want %d", manageResp.Code, http.StatusForbidden)
	}
}

func TestRequireSessionRejectsAPIToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	parser := &parserStub{identity: authctx.Identity{UserID: uuid.New(), Method: authctx.AuthMethodAPIToken}}
	router := gin.New()
	router.GET("/", Auth(parser), RequireSession(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer mpat_test")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusForbidden)
	}
}
