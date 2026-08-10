//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
)

func TestAppStoreIntegration(t *testing.T) {
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
	t.Cleanup(pool.Close)

	store := NewAppStore(sqlc.New(pool))
	name := "integration-" + uuid.NewString()

	created, err := store.Create(ctx, name)
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Delete(context.Background(), created.ID); err != nil {
			t.Errorf("delete integration app: %v", err)
		}
	})

	if created.Name != name || created.Status != domain.AppStatusIdle {
		t.Fatalf("unexpected created app: %#v", created)
	}

	if _, err := store.Create(ctx, name); !errors.Is(err, domain.ErrAppNameTaken) {
		t.Fatalf("duplicate app error = %v, want %v", err, domain.ErrAppNameTaken)
	}

	if err := store.UpdateStatus(ctx, created.ID, domain.AppStatusRunning); err != nil {
		t.Fatalf("update app status: %v", err)
	}
	if err := store.UpdatePublicURL(ctx, created.ID, "https://"+name+".minipaas.local"); err != nil {
		t.Fatalf("update app URL: %v", err)
	}

	found, err := store.GetByName(ctx, name)
	if err != nil {
		t.Fatalf("get app by name: %v", err)
	}
	if found.ID != created.ID || found.Status != domain.AppStatusRunning {
		t.Fatalf("unexpected stored app: %#v", found)
	}
	if found.PublicURL != "https://"+name+".minipaas.local" {
		t.Fatalf("public URL = %q", found.PublicURL)
	}
}
