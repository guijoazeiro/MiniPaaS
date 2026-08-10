package handler

import (
	"bytes"
	"context"
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

type uploadHandlerService struct{ createCalled bool }

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
