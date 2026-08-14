package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type auditRecorder struct {
	events []store.AuditEvent
}

func (r *auditRecorder) Record(_ context.Context, event store.AuditEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (*auditRecorder) List(context.Context, int, int) ([]store.AuditEvent, error) { return nil, nil }

func TestAuditRecordsMutationsWithoutRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &auditRecorder{}
	r := gin.New()
	r.Use(RequestID(), Audit(recorder, slog.Default()))
	r.POST("/apps/:name/env/:key", func(c *gin.Context) {
		c.Set(CtxUserID, uuid.MustParse("11111111-1111-1111-1111-111111111111"))
		c.Status(204)
	})

	req := httptest.NewRequest(http.MethodPost, "/apps/demo/env/TOKEN", nil)
	req.Header.Set("X-Request-ID", "22222222-2222-2222-2222-222222222222")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != 204 {
		t.Fatalf("status = %d, want 204", res.Code)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("events = %d, want 1", len(recorder.events))
	}
	event := recorder.events[0]
	if event.Action != "post /apps/:name/env/:key" || event.Path != "/apps/demo/env/TOKEN" || event.Status != 204 {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.UserID == uuid.Nil || event.RequestID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("missing correlation fields: %+v", event)
	}
}

func TestAuditSkipsReadsAndHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &auditRecorder{}
	r := gin.New()
	r.Use(Audit(recorder, nil))
	r.GET("/apps", func(c *gin.Context) { c.Status(200) })
	r.POST("/health", func(c *gin.Context) { c.Status(200) })
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/apps", nil),
		httptest.NewRequest(http.MethodPost, "/health", nil),
	} {
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("events = %d, want 0", len(recorder.events))
	}
}
