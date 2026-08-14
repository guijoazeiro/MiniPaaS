//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	"golang.org/x/crypto/bcrypt"
)

func TestAppStoreScopesApplicationsToAuthenticatedUser(t *testing.T) {
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
	users := NewUserStore(queries)
	apps := NewAppStore(queries)
	hash, err := bcrypt.GenerateFromPassword([]byte("ownership-test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	userA, err := users.Create(ctx, "owner-a-"+uuid.NewString()[:8], string(hash))
	if err != nil {
		t.Fatal(err)
	}
	userB, err := users.Create(ctx, "owner-b-"+uuid.NewString()[:8], string(hash))
	if err != nil {
		t.Fatal(err)
	}
	appA, err := apps.Create(authctx.WithUserID(ctx, userA.ID), "owned-a-"+uuid.NewString()[:8])
	if err != nil {
		t.Fatal(err)
	}
	appB, err := apps.Create(authctx.WithUserID(ctx, userB.ID), "owned-b-"+uuid.NewString()[:8])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = apps.Delete(ctx, appA.ID)
		_ = apps.Delete(ctx, appB.ID)
	})

	owned, err := apps.List(authctx.WithUserID(ctx, userA.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(owned) != 1 || owned[0].ID != appA.ID {
		t.Fatalf("user A applications = %+v, want only %s", owned, appA.ID)
	}
	if _, err := apps.GetByName(authctx.WithUserID(ctx, userA.ID), appB.Name); !errors.Is(err, domain.ErrAppNotFound) {
		t.Fatalf("cross-owner lookup error = %v, want app not found", err)
	}
}
