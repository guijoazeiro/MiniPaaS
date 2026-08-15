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

func TestGitHubInstallationStoreScopesInstallationsToAuthenticatedUser(t *testing.T) {
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
	installations := NewGitHubInstallationStore(queries)
	hash, err := bcrypt.GenerateFromPassword([]byte("ownership-test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	userA, err := users.Create(ctx, "github-owner-a-"+uuid.NewString()[:8], string(hash))
	if err != nil {
		t.Fatal(err)
	}
	userB, err := users.Create(ctx, "github-owner-b-"+uuid.NewString()[:8], string(hash))
	if err != nil {
		t.Fatal(err)
	}
	installationID := time.Now().UnixNano()
	installation := domain.GitHubInstallation{
		InstallationID:      installationID,
		AccountLogin:        "github-owner-a",
		AccountType:         "User",
		RepositorySelection: "selected",
	}
	ownerCtxA := authctx.WithUserID(ctx, userA.ID)
	ownerCtxB := authctx.WithUserID(ctx, userB.ID)
	if _, err := installations.Upsert(ownerCtxA, installation); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM github_installations WHERE installation_id = $1", installationID)
	})
	if _, err := installations.Get(ownerCtxA, installationID); err != nil {
		t.Fatalf("owner lookup error = %v", err)
	}
	if _, err := installations.Get(ownerCtxB, installationID); !errors.Is(err, domain.ErrGitHubInstallationNotFound) {
		t.Fatalf("cross-owner lookup error = %v, want installation not found", err)
	}
	listA, err := installations.List(ownerCtxA)
	if err != nil {
		t.Fatal(err)
	}
	foundA := false
	for _, item := range listA {
		if item.InstallationID == installationID {
			foundA = true
			break
		}
	}
	if !foundA {
		t.Fatalf("owner list does not contain installation %d", installationID)
	}
	listB, err := installations.List(ownerCtxB)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range listB {
		if item.InstallationID == installationID {
			t.Fatalf("cross-owner list contains installation %d", installationID)
		}
	}
	if _, err := installations.Upsert(ownerCtxB, installation); !errors.Is(err, domain.ErrGitHubInstallationOwned) {
		t.Fatalf("cross-owner upsert error = %v, want installation owned", err)
	}
}
