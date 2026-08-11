package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type gitHandlerSources struct{ source domain.GitSource }

func (s *gitHandlerSources) Configure(_ context.Context, _ string, source domain.GitSource) (domain.GitSource, error) {
	s.source = source
	return source, nil
}
func (s *gitHandlerSources) Get(context.Context, string) (domain.GitSource, error) {
	return s.source, nil
}
func (s *gitHandlerSources) Delete(context.Context, string) error { return nil }

type gitHandlerDeployments struct{ ran chan struct{} }

func (s *gitHandlerDeployments) Create(_ context.Context, _ string, _ string) (domain.Deployment, domain.App, domain.GitSource, error) {
	return domain.Deployment{ID: uuid.New(), Status: domain.DeploymentStatusPending, SourceType: "git"}, domain.App{ID: uuid.New(), Name: "app"}, domain.GitSource{Repository: "owner/repo", Branch: "main"}, nil
}
func (s *gitHandlerDeployments) Run(context.Context, domain.Deployment, domain.App, domain.GitSource, string) error {
	close(s.ran)
	return nil
}

func TestGitSourceHandlerConfigure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sources := &gitHandlerSources{}
	h := NewGitSourceHandler(sources, &gitHandlerDeployments{ran: make(chan struct{})}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := gin.New()
	r.PUT("/apps/:name/source/git", h.Configure)
	req := httptest.NewRequest(http.MethodPut, "/apps/app/source/git", strings.NewReader(`{"repository":"owner/repo","branch":"main"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if sources.source.Repository != "owner/repo" {
		t.Fatalf("source = %#v", sources.source)
	}
}

func TestGitSourceHandlerDeployAcceptsEmptyBody(t *testing.T) {
	deployments := &gitHandlerDeployments{ran: make(chan struct{})}
	h := NewGitSourceHandler(&gitHandlerSources{}, deployments, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := gin.New()
	r.POST("/apps/:name/deployments/git", h.Deploy)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/apps/app/deployments/git", nil))
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	select {
	case <-deployments.ran:
	case <-time.After(time.Second):
		t.Fatal("background Git deployment did not start")
	}
}
