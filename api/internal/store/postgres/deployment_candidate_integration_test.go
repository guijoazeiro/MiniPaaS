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

func TestDeploymentCandidateLifecycleIntegration(t *testing.T) {
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
	deployments := NewDeploymentStore(queries)
	app, err := apps.Create(ctx, "candidate-integration-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = apps.Delete(context.Background(), app.ID) })

	deployment, err := deployments.Create(ctx, app.ID, "candidate:test")
	if err != nil {
		t.Fatal(err)
	}
	if err := deployments.UpdateCandidate(ctx, deployment.ID, "candidate-container", 4321); err != nil {
		t.Fatal(err)
	}
	stored, err := deployments.GetByID(ctx, deployment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CandidateContainerID != "candidate-container" || stored.CandidatePort != 4321 {
		t.Fatalf("candidate metadata = %#v", stored)
	}

	candidates, err := deployments.ListCandidates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ID != deployment.ID {
		t.Fatalf("candidates = %#v", candidates)
	}

	if err := deployments.PromoteCandidate(ctx, deployment.ID, "promoted-container", 4322, "candidate:test", 42); err != nil {
		t.Fatal(err)
	}
	active, err := deployments.GetActive(ctx, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active.Status != domain.DeploymentStatusRunning || active.ContainerID != "promoted-container" || active.Port != 4322 || active.CandidateContainerID != "" {
		t.Fatalf("promoted deployment = %#v", active)
	}
}
