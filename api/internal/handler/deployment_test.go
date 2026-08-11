package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type uploadHandlerService struct {
	createCalled bool
	listPage     domain.DeploymentPage
	listApp      string
	listStatus   string
	page         int
	perPage      int
}

func (s *uploadHandlerService) Create(context.Context, string) (domain.Deployment, domain.App, error) {
	s.createCalled = true
	return domain.Deployment{}, domain.App{}, nil
}
func (*uploadHandlerService) Get(context.Context, uuid.UUID) (domain.Deployment, error) {
	return domain.Deployment{}, nil
}
func (*uploadHandlerService) ListByApp(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return nil, nil
}
func (s *uploadHandlerService) ListAll(_ context.Context, app, status string, page, perPage int) (domain.DeploymentPage, error) {
	s.listApp, s.listStatus, s.page, s.perPage = app, status, page, perPage
	return s.listPage, nil
}

func TestDeploymentListAllUsesFiltersAndPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &uploadHandlerService{listPage: domain.DeploymentPage{
		Items: []domain.DeploymentListItem{{Deployment: domain.Deployment{ID: uuid.New()}, AppName: "api"}},
		Page:  2, PerPage: 25, Total: 30,
	}}
	h := NewDeploymentHandler(svc, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), 128)
	r := gin.New()
	r.GET("/deployments", h.ListAll)
	req := httptest.NewRequest(http.MethodGet, "/deployments?app=api&status=failed&page=2&per_page=25", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if svc.listApp != "api" || svc.listStatus != "failed" || svc.page != 2 || svc.perPage != 25 {
		t.Fatalf("list args = %q %q %d %d", svc.listApp, svc.listStatus, svc.page, svc.perPage)
	}
	var body domain.DeploymentPage
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 30 || len(body.Items) != 1 || body.Items[0].AppName != "api" {
		t.Fatalf("body = %+v", body)
	}
}
func (*uploadHandlerService) RunBuild(context.Context, domain.Deployment, domain.App, io.Reader) error {
	return nil
}
func (*uploadHandlerService) Rollback(context.Context, string, uuid.UUID, string) (domain.Deployment, error) {
	return domain.Deployment{}, nil
}

func TestDeploymentUploadRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("source", "source.tar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("x"), 256)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	svc := &uploadHandlerService{}
	h := NewDeploymentHandler(svc, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), 128)
	r := gin.New()
	r.POST("/apps/:name/deployments", h.Create)
	req := httptest.NewRequest(http.MethodPost, "/apps/app/deployments", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusRequestEntityTooLarge, resp.Body.String())
	}
	if svc.createCalled {
		t.Fatal("deployment created for oversized request")
	}
}
