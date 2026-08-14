//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
)

func TestAuditStoreIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	audit := NewAuditStore(sqlc.New(pool))
	requestID := uuid.NewString()
	if err := audit.Record(ctx, store.AuditEvent{Action: "post /apps", Method: "POST", Path: "/apps", Status: 201, RequestID: requestID}); err != nil {
		t.Fatalf("record audit event: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM audit_events WHERE request_id = $1", requestID); err != nil {
			t.Errorf("delete audit event: %v", err)
		}
	})

	events, err := audit.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	for _, event := range events {
		if event.RequestID == requestID {
			if event.Action != "post /apps" || event.Status != 201 {
				t.Fatalf("unexpected event: %+v", event)
			}
			return
		}
	}
	t.Fatalf("audit event %q was not returned", requestID)
}
