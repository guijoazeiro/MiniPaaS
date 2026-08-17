package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type githubAppTestApps struct{ app domain.App }

func (f githubAppTestApps) Create(context.Context, string) (domain.App, error) {
	return f.app, nil
}
func (f githubAppTestApps) GetByName(_ context.Context, name string) (domain.App, error) {
	if name != f.app.Name {
		return domain.App{}, domain.ErrAppNotFound
	}
	return f.app, nil
}
func (f githubAppTestApps) GetByID(context.Context, uuid.UUID) (domain.App, error) {
	return f.app, nil
}
func (f githubAppTestApps) List(context.Context) ([]domain.App, error) {
	return []domain.App{f.app}, nil
}
func (f githubAppTestApps) UpdateStatus(context.Context, uuid.UUID, domain.AppStatus) error {
	return nil
}
func (f githubAppTestApps) UpdatePublicURL(context.Context, uuid.UUID, string) error { return nil }
func (f githubAppTestApps) Delete(context.Context, uuid.UUID) error                  { return nil }

type githubAppTestInstallations struct {
	upserted bool
}

func (f *githubAppTestInstallations) Upsert(context.Context, domain.GitHubInstallation) (domain.GitHubInstallation, error) {
	f.upserted = true
	return domain.GitHubInstallation{}, nil
}
func (f *githubAppTestInstallations) Get(context.Context, int64) (domain.GitHubInstallation, error) {
	return domain.GitHubInstallation{}, nil
}
func (f *githubAppTestInstallations) List(context.Context) ([]domain.GitHubInstallation, error) {
	return nil, nil
}

type githubAppTestClient struct{ called bool }

func (f *githubAppTestClient) InstallURL(string) string { return "https://github.com/install" }
func (f *githubAppTestClient) Installation(context.Context, int64) (domain.GitHubInstallation, error) {
	f.called = true
	return domain.GitHubInstallation{InstallationID: 42}, nil
}
func (f *githubAppTestClient) ListRepositories(context.Context, int64) ([]domain.GitHubRepository, error) {
	return nil, nil
}

type githubAppTestState struct {
	userID uuid.UUID
}

func (f *githubAppTestState) Sign(string, uuid.UUID) (string, error) { return "state", nil }
func (f *githubAppTestState) SignAccount(uuid.UUID) (string, error)  { return "account-state", nil }
func (f *githubAppTestState) Verify(string) (string, uuid.UUID, error) {
	return "private-api", f.userID, nil
}
func (f *githubAppTestState) VerifyTarget(string) (string, uuid.UUID, string, error) {
	return "private-api", f.userID, "app", nil
}

func TestGitHubAppConnectBindsInstallationToStateUser(t *testing.T) {
	stateUser := uuid.New()
	requestUser := uuid.New()
	installations := &githubAppTestInstallations{}
	client := &githubAppTestClient{}
	service := NewGitHubAppService(
		githubAppTestApps{app: domain.App{ID: uuid.New(), Name: "private-api"}},
		installations,
		client,
		&githubAppTestState{userID: stateUser},
	)

	_, _, err := service.Connect(authctx.WithUserID(context.Background(), requestUser), 42, "state")
	if !errors.Is(err, domain.ErrGitHubInstallationInvalid) {
		t.Fatalf("Connect() error = %v, want invalid installation state", err)
	}
	if client.called || installations.upserted {
		t.Fatal("Connect() touched GitHub or persisted an installation for a different user")
	}

	_, _, err = service.Connect(authctx.WithUserID(context.Background(), stateUser), 42, "state")
	if err != nil {
		t.Fatalf("Connect() with matching user error = %v", err)
	}
	if !client.called || !installations.upserted {
		t.Fatal("Connect() with matching user did not complete installation")
	}
}

func TestGitHubAppAccountInstallURLRequiresAuthenticatedUser(t *testing.T) {
	client := &githubAppTestClient{}
	service := NewGitHubAppService(
		githubAppTestApps{app: domain.App{ID: uuid.New(), Name: "private-api"}},
		&githubAppTestInstallations{},
		client,
		&githubAppTestState{},
	)
	if _, err := service.AccountInstallURL(context.Background()); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("AccountInstallURL() error = %v, want unauthorized", err)
	}
	userID := uuid.New()
	url, err := service.AccountInstallURL(authctx.WithUserID(context.Background(), userID))
	if err != nil {
		t.Fatalf("AccountInstallURL() error = %v", err)
	}
	if url != "https://github.com/install" {
		t.Fatalf("AccountInstallURL() = %q", url)
	}
}
