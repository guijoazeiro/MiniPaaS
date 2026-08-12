package service

import (
	"context"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type GitHubAppClient interface {
	InstallURL(state string) string
	Installation(ctx context.Context, installationID int64) (domain.GitHubInstallation, error)
	ListRepositories(ctx context.Context, installationID int64) ([]domain.GitHubRepository, error)
}

type GitHubStateSigner interface {
	Sign(appName string) (string, error)
	Verify(raw string) (string, error)
}

type GitHubAppService struct {
	apps          store.AppStore
	installations store.GitHubInstallationStore
	client        GitHubAppClient
	states        GitHubStateSigner
}

func NewGitHubAppService(apps store.AppStore, installations store.GitHubInstallationStore, client GitHubAppClient, states GitHubStateSigner) *GitHubAppService {
	return &GitHubAppService{apps: apps, installations: installations, client: client, states: states}
}

func (s *GitHubAppService) Enabled() bool {
	return s != nil && s.client != nil && s.states != nil
}

func (s *GitHubAppService) InstallURL(ctx context.Context, appName string) (string, error) {
	if !s.Enabled() {
		return "", domain.ErrGitHubAppNotConfigured
	}
	if _, err := s.apps.GetByName(ctx, appName); err != nil {
		return "", err
	}
	state, err := s.states.Sign(appName)
	if err != nil {
		return "", err
	}
	return s.client.InstallURL(state), nil
}

func (s *GitHubAppService) Connect(ctx context.Context, installationID int64, state string) (string, domain.GitHubInstallation, error) {
	if !s.Enabled() || installationID <= 0 {
		return "", domain.GitHubInstallation{}, domain.ErrGitHubInstallationInvalid
	}
	appName, err := s.states.Verify(state)
	if err != nil {
		return "", domain.GitHubInstallation{}, err
	}
	if _, err := s.apps.GetByName(ctx, appName); err != nil {
		return "", domain.GitHubInstallation{}, err
	}
	installation, err := s.client.Installation(ctx, installationID)
	if err != nil {
		return "", domain.GitHubInstallation{}, err
	}
	installation, err = s.installations.Upsert(ctx, installation)
	return appName, installation, err
}

func (s *GitHubAppService) ListInstallations(ctx context.Context) ([]domain.GitHubInstallation, error) {
	if !s.Enabled() {
		return nil, domain.ErrGitHubAppNotConfigured
	}
	return s.installations.List(ctx)
}

func (s *GitHubAppService) ListRepositories(ctx context.Context, installationID int64) ([]domain.GitHubRepository, error) {
	if !s.Enabled() {
		return nil, domain.ErrGitHubAppNotConfigured
	}
	if _, err := s.installations.Get(ctx, installationID); err != nil {
		return nil, err
	}
	return s.client.ListRepositories(ctx, installationID)
}

func (s *GitHubAppService) Repository(ctx context.Context, installationID, repositoryID int64) (domain.GitHubRepository, error) {
	repositories, err := s.ListRepositories(ctx, installationID)
	if err != nil {
		return domain.GitHubRepository{}, err
	}
	for _, repository := range repositories {
		if repository.ID == repositoryID {
			return repository, nil
		}
	}
	return domain.GitHubRepository{}, domain.ErrGitHubRepositoryNotAccessible
}
