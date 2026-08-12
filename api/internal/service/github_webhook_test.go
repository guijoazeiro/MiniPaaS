package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type webhookSourceStore struct{ source domain.GitSource }

func (s *webhookSourceStore) Upsert(context.Context, domain.GitSource) (domain.GitSource, error) {
	return domain.GitSource{}, nil
}
func (s *webhookSourceStore) Get(context.Context, uuid.UUID) (domain.GitSource, error) {
	return s.source, nil
}
func (s *webhookSourceStore) Delete(context.Context, uuid.UUID) error { return nil }
func (s *webhookSourceStore) SetAutoDeploy(context.Context, uuid.UUID, bool) (domain.GitSource, error) {
	return s.source, nil
}
func (s *webhookSourceStore) ListAutoDeployByRepository(context.Context, int64) ([]domain.GitSource, error) {
	return []domain.GitSource{s.source}, nil
}

type webhookAppStore struct{ app domain.App }

func (s *webhookAppStore) Create(context.Context, string) (domain.App, error)     { return s.app, nil }
func (s *webhookAppStore) GetByName(context.Context, string) (domain.App, error)  { return s.app, nil }
func (s *webhookAppStore) GetByID(context.Context, uuid.UUID) (domain.App, error) { return s.app, nil }
func (s *webhookAppStore) List(context.Context) ([]domain.App, error) {
	return []domain.App{s.app}, nil
}
func (s *webhookAppStore) UpdateStatus(context.Context, uuid.UUID, domain.AppStatus) error {
	return nil
}
func (s *webhookAppStore) UpdatePublicURL(context.Context, uuid.UUID, string) error { return nil }
func (s *webhookAppStore) Delete(context.Context, uuid.UUID) error                  { return nil }

type webhookDeliveryStore struct {
	claimed bool
	status  string
}

func (s *webhookDeliveryStore) Claim(context.Context, string, string, int64, string) (bool, error) {
	return s.claimed, nil
}
func (s *webhookDeliveryStore) Complete(_ context.Context, _ string, status, _ string) error {
	s.status = status
	return nil
}

type webhookDeployments struct{ created int }

func (s *webhookDeployments) CreateTriggered(_ context.Context, _ string, _ domain.GitSource, _, _ string) (domain.Deployment, domain.App, error) {
	s.created++
	return domain.Deployment{ID: uuid.New()}, domain.App{Name: "api"}, nil
}
func (s *webhookDeployments) Run(context.Context, domain.Deployment, domain.App, domain.GitSource, string) error {
	return nil
}

func TestGitHubWebhookStartsMatchingAutoDeployOnce(t *testing.T) {
	appID := uuid.New()
	installationID, repositoryID := int64(42), int64(99)
	sources := &webhookSourceStore{source: domain.GitSource{
		AppID: appID, Repository: "acme/api", Branch: "main", AccessMode: domain.GitAccessGitHubApp,
		GitHubInstallationID: &installationID, GitHubRepositoryID: &repositoryID, AutoDeploy: true,
	}}
	apps := &webhookAppStore{app: domain.App{ID: appID, Name: "api"}}
	deliveries := &webhookDeliveryStore{claimed: true}
	deployments := &webhookDeployments{}
	service := NewGitHubWebhookService(sources, apps, deliveries, deployments, slog.New(slog.NewTextHandler(io.Discard, nil)))
	payload := []byte(`{"ref":"refs/heads/main","after":"0123456789012345678901234567890123456789","repository":{"id":99,"full_name":"acme/api"},"installation":{"id":42}}`)

	result, err := service.Handle(context.Background(), "push", "delivery-1", payload)
	if err != nil {
		t.Fatal(err)
	}
	if result.Deployments != 1 || deployments.created != 1 || deliveries.status != "accepted" {
		t.Fatalf("result=%+v created=%d status=%q", result, deployments.created, deliveries.status)
	}
}

func TestGitHubWebhookIgnoresDuplicateAndOtherBranch(t *testing.T) {
	appID := uuid.New()
	installationID, repositoryID := int64(42), int64(99)
	sources := &webhookSourceStore{source: domain.GitSource{AppID: appID, Repository: "acme/api", Branch: "main", GitHubInstallationID: &installationID, GitHubRepositoryID: &repositoryID}}
	apps := &webhookAppStore{app: domain.App{ID: appID, Name: "api"}}
	deployments := &webhookDeployments{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	payload := []byte(`{"ref":"refs/heads/release","after":"0123456789012345678901234567890123456789","repository":{"id":99,"full_name":"acme/api"},"installation":{"id":42}}`)

	duplicate := NewGitHubWebhookService(sources, apps, &webhookDeliveryStore{claimed: false}, deployments, log)
	result, err := duplicate.Handle(context.Background(), "push", "delivery-1", payload)
	if err != nil || !result.Duplicate {
		t.Fatalf("duplicate result=%+v err=%v", result, err)
	}

	deliveries := &webhookDeliveryStore{claimed: true}
	service := NewGitHubWebhookService(sources, apps, deliveries, deployments, log)
	result, err = service.Handle(context.Background(), "push", "delivery-2", payload)
	if err != nil || !result.Ignored || deliveries.status != "ignored" || deployments.created != 0 {
		t.Fatalf("branch result=%+v status=%q created=%d err=%v", result, deliveries.status, deployments.created, err)
	}
}
