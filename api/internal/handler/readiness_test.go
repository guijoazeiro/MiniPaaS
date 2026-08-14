package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestReadinessHandlerReportsAllDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewReadinessHandler(time.Second, map[string]ReadinessProbe{
		"database": func(context.Context) error { return nil },
		"docker":   func(context.Context) error { return nil },
	}, slog.Default())
	router := gin.New()
	router.GET("/ready", h.Serve)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"status":"ready"`) || !strings.Contains(body, `"database":"ok"`) {
		t.Fatalf("body = %s", body)
	}
}

func TestReadinessHandlerReturns503OnProbeFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewReadinessHandler(time.Second, map[string]ReadinessProbe{
		"database": func(context.Context) error { return errors.New("database down") },
	}, slog.Default())
	router := gin.New()
	router.GET("/ready", h.Serve)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(resp.Body.String(), `"database":"error"`) {
		t.Fatalf("body = %s", resp.Body.String())
	}
}
