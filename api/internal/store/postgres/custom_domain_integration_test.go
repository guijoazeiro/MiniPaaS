//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
)

func TestCustomDomainLifecycleIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	queries := sqlc.New(pool)
	apps := NewAppStore(queries)
	domains := NewCustomDomainStore(queries)
	app, err := apps.Create(ctx, "domain-integration-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = apps.Delete(context.Background(), app.ID) })

	item, err := domains.Create(ctx, app.ID, "api-"+uuid.NewString()+".example.com")
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != domain.CustomDomainStatusPending {
		t.Fatalf("created domain = %#v", item)
	}
	verifiedAt := time.Now().UTC()
	if err := domains.UpdateVerification(ctx, item.ID, domain.CustomDomainStatusActive, "", &verifiedAt); err != nil {
		t.Fatal(err)
	}
	items, err := domains.ListByApp(ctx, app.ID)
	if err != nil || len(items) != 1 || items[0].Status != domain.CustomDomainStatusActive || items[0].VerifiedAt == nil {
		t.Fatalf("domains = %#v, err = %v", items, err)
	}
	if err := domains.Delete(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := domains.Get(ctx, item.ID); err != domain.ErrCustomDomainNotFound {
		t.Fatalf("Get after delete = %v", err)
	}
}
