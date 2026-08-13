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

func TestGitSourceAndDeploymentMetadataIntegration(t *testing.T) {
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
	sources := NewGitSourceStore(queries)
	deployments := NewDeploymentStore(queries)
	app, err := apps.Create(ctx, "git-integration-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = apps.Delete(context.Background(), app.ID) })

	source, err := sources.Upsert(ctx, domain.GitSource{AppID: app.ID, Repository: "owner/repository", Branch: "main", BuildContext: "services/api", DockerfilePath: "Dockerfile", AccessMode: domain.GitAccessPublic})
	if err != nil {
		t.Fatal(err)
	}
	if source.Repository != "owner/repository" || source.BuildContext != "services/api" {
		t.Fatalf("source = %#v", source)
	}
	found, err := sources.Get(ctx, app.ID)
	if err != nil || found.Repository != source.Repository {
		t.Fatalf("Get() = %#v, %v", found, err)
	}

	deployment, err := deployments.CreateGit(ctx, app.ID, "test:git", source.Repository, source.Branch)
	if err != nil {
		t.Fatal(err)
	}
	if err := deployments.UpdateGitMetadata(ctx, deployment.ID, "0123456789abcdef", "Codex", "phase 9", "main"); err != nil {
		t.Fatal(err)
	}
	deployment, err = deployments.GetByID(ctx, deployment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deployment.SourceType != "git" || deployment.CommitSHA != "0123456789abcdef" || deployment.CommitAuthor != "Codex" {
		t.Fatalf("deployment = %#v", deployment)
	}
}
