package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type stopHandlerApps struct{ app domain.App }

func (*stopHandlerApps) Create(context.Context, string) (domain.App, error)      { return domain.App{}, nil }
func (s *stopHandlerApps) GetByName(context.Context, string) (domain.App, error) { return s.app, nil }
func (*stopHandlerApps) List(context.Context) ([]domain.App, error)              { return nil, nil }
func (*stopHandlerApps) Delete(context.Context, uuid.UUID) error                 { return nil }

type stopHandlerStopper struct{ calls int }

func (s *stopHandlerStopper) StopApp(context.Context, domain.App) error { s.calls++; return nil }

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
