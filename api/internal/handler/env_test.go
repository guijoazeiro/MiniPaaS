package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type envHandlerService struct {
	called bool
	value  string
}

func (s *envHandlerService) Set(_ context.Context, _ uuid.UUID, _, value string) error {
	s.called = true
	s.value = value
	return nil
}
func (*envHandlerService) List(context.Context, uuid.UUID) ([]domain.EnvVarKey, error) {
	return nil, nil
}
func (*envHandlerService) Delete(context.Context, uuid.UUID, string) error { return nil }

type envHandlerApps struct{ app domain.App }

func (a *envHandlerApps) GetByName(context.Context, string) (domain.App, error) { return a.app, nil }

func newEnvRouter(svc *envHandlerService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewEnvHandler(svc, &envHandlerApps{app: domain.App{ID: uuid.New(), Name: "app"}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.PUT("/apps/:name/env/:key", h.Set)
	return r
}

func TestSetEnvRejectsMissingValueField(t *testing.T) {
	svc := &envHandlerService{}
	req := httptest.NewRequest(http.MethodPut, "/apps/app/env/EMPTY", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	newEnvRouter(svc).ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
	if svc.called {
		t.Fatal("service called for body without value field")
	}
}

func TestSetEnvAcceptsExplicitEmptyValue(t *testing.T) {
	svc := &envHandlerService{}
	req := httptest.NewRequest(http.MethodPut, "/apps/app/env/EMPTY", strings.NewReader(`{"value":""}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	newEnvRouter(svc).ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNoContent)
	}
	if !svc.called || svc.value != "" {
		t.Fatalf("service call = %v, value = %q", svc.called, svc.value)
	}
}
