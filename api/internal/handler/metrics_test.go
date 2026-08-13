package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type metricsHandlerService struct{}

func (metricsHandlerService) Get(context.Context, string) (domain.AppMetrics, error) {
	return domain.AppMetrics{AppName: "api", HealthCheckFailures: []domain.HealthCheckFailure{}}, nil
}

func TestMetricsHandlerReturnsSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewMetricsHandler(metricsHandlerService{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.GET("/apps/:name/metrics", h.Get)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/apps/api/metrics", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Body.String() == "" {
		t.Fatal("expected metrics response body")
	}
}
