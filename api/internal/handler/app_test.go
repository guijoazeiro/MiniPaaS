package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type stopHandlerApps struct {
	app     domain.App
	deleted int
}

func (*stopHandlerApps) Create(context.Context, string) (domain.App, error)      { return domain.App{}, nil }
func (s *stopHandlerApps) GetByName(context.Context, string) (domain.App, error) { return s.app, nil }
func (*stopHandlerApps) List(context.Context) ([]domain.App, error)              { return nil, nil }
func (s *stopHandlerApps) Delete(context.Context, uuid.UUID) error               { s.deleted++; return nil }

type stopHandlerStopper struct {
	calls int
	err   error
}

func (s *stopHandlerStopper) StopApp(context.Context, domain.App) error { s.calls++; return s.err }

func TestAppHandlerStopPreservesApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apps := &stopHandlerApps{app: domain.App{ID: uuid.New(), Name: "app", Status: domain.AppStatusRunning}}
	stopper := &stopHandlerStopper{}
	h := NewAppHandler(apps, stopper, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := gin.New()
	r.POST("/apps/:name/stop", h.Stop)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/apps/app/stop", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if stopper.calls != 1 {
		t.Fatalf("StopApp calls = %d", stopper.calls)
	}
}

func TestAppHandlerDeleteDoesNotRemoveRecordWhenRuntimeCleanupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apps := &stopHandlerApps{app: domain.App{ID: uuid.New(), Name: "app", Status: domain.AppStatusRunning}}
	stopper := &stopHandlerStopper{err: errors.New("container cleanup failed")}
	h := NewAppHandler(apps, stopper, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := gin.New()
	r.DELETE("/apps/:name", h.Delete)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/apps/app", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if apps.deleted != 0 {
		t.Fatal("application was deleted after runtime cleanup failed")
	}
}
