package service

import (
	"context"
	"fmt"
	"time"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/sourcegit"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type GitSourceService struct {
	apps    store.AppStore
	sources store.GitSourceStore
	github  GitHubRepositoryProvider
}

type GitHubRepositoryProvider interface {
	Repository(ctx context.Context, installationID, repositoryID int64) (domain.GitHubRepository, error)
}

func NewGitSourceService(apps store.AppStore, sources store.GitSourceStore, github ...GitHubRepositoryProvider) *GitSourceService {
	service := &GitSourceService{apps: apps, sources: sources}
	if len(github) > 0 {
		service.github = github[0]
	}
	return service
}

func (s *GitSourceService) Configure(ctx context.Context, appName string, source domain.GitSource) (domain.GitSource, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.GitSource{}, err
	}
	source.AppID = app.ID
	source.AccessMode = domain.GitAccessPublic
	source.GitHubInstallationID = nil
	source.GitHubRepositoryID = nil
	source.Private = false
	source.Repository, err = sourcegit.NormalizeRepository(source.Repository)
	if err != nil {
		return domain.GitSource{}, err
	}
	source.Branch, err = sourcegit.NormalizeBranch(source.Branch)
	if err != nil {
		return domain.GitSource{}, err
	}
	source.BuildContext, err = sourcegit.NormalizeRelativePath(source.BuildContext, ".")
	if err != nil {
		return domain.GitSource{}, err
	}
	source.DockerfilePath, err = sourcegit.NormalizeRelativePath(source.DockerfilePath, "Dockerfile")
	if err != nil {
		return domain.GitSource{}, err
	}
	return s.sources.Upsert(ctx, source)
}

func (s *GitSourceService) ConfigureGitHubApp(ctx context.Context, appName string, installationID, repositoryID int64, branch, buildContext, dockerfilePath string) (domain.GitSource, error) {
	if s.github == nil {
		return domain.GitSource{}, domain.ErrGitHubAppNotConfigured
	}
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.GitSource{}, err
	}
	repository, err := s.github.Repository(ctx, installationID, repositoryID)
	if err != nil {
		return domain.GitSource{}, err
	}
	if branch == "" {
		branch = repository.DefaultBranch
	}
	source := domain.GitSource{
		AppID:                app.ID,
		Repository:           repository.FullName,
		Branch:               branch,
		BuildContext:         buildContext,
		DockerfilePath:       dockerfilePath,
		AccessMode:           domain.GitAccessGitHubApp,
		GitHubInstallationID: &installationID,
		GitHubRepositoryID:   &repositoryID,
		Private:              repository.Private,
	}
	source.Repository, err = sourcegit.NormalizeRepository(source.Repository)
	if err != nil {
		return domain.GitSource{}, err
	}
	source.Branch, err = sourcegit.NormalizeBranch(source.Branch)
	if err != nil {
		return domain.GitSource{}, err
	}
	source.BuildContext, err = sourcegit.NormalizeRelativePath(source.BuildContext, ".")
	if err != nil {
		return domain.GitSource{}, err
	}
	source.DockerfilePath, err = sourcegit.NormalizeRelativePath(source.DockerfilePath, "Dockerfile")
	if err != nil {
		return domain.GitSource{}, err
	}
	return s.sources.Upsert(ctx, source)
}

func (s *GitSourceService) Get(ctx context.Context, appName string) (domain.GitSource, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.GitSource{}, err
	}
	return s.sources.Get(ctx, app.ID)
}

func (s *GitSourceService) Delete(ctx context.Context, appName string) error {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return err
	}
	if _, err := s.sources.Get(ctx, app.ID); err != nil {
		return err
	}
	if err := s.sources.Delete(ctx, app.ID); err != nil {
		return fmt.Errorf("service.DeleteGitSource: %w", err)
	}
	return nil
}

type GitDeploymentService struct {
	sources      store.GitSourceStore
	apps         store.AppStore
	deployments  *DeploymentService
	preparer     sourcegit.Preparer
	cloneTimeout time.Duration
}

func NewGitDeploymentService(sources store.GitSourceStore, apps store.AppStore, deployments *DeploymentService, preparer sourcegit.Preparer, cloneTimeout time.Duration) *GitDeploymentService {
	if cloneTimeout <= 0 {
		cloneTimeout = 10 * time.Minute
	}
	return &GitDeploymentService{sources: sources, apps: apps, deployments: deployments, preparer: preparer, cloneTimeout: cloneTimeout}
}

func (s *GitDeploymentService) Create(ctx context.Context, appName, branch string) (domain.Deployment, domain.App, domain.GitSource, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, err
	}
	source, err := s.sources.Get(ctx, app.ID)
	if err != nil {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, err
	}
	if branch == "" {
		branch = source.Branch
	}
	branch, err = sourcegit.NormalizeBranch(branch)
	if err != nil {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, err
	}
	dep, app, err := s.deployments.CreateGit(ctx, appName, source.Repository, branch)
	return dep, app, source, err
}

func (s *GitDeploymentService) Run(ctx context.Context, dep domain.Deployment, app domain.App, source domain.GitSource, branch string) error {
	if branch == "" {
		branch = source.Branch
	}
	cloneCtx, cancel := context.WithTimeout(ctx, s.cloneTimeout)
	snapshot, err := s.preparer.Prepare(cloneCtx, source, branch)
	cancel()
	if err != nil {
		s.deployments.MarkFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunGitBuild: prepare source: %w", err)
	}
	defer snapshot.Source.Close()
	if err := s.deployments.UpdateGitMetadata(ctx, dep.ID, snapshot.CommitSHA, snapshot.CommitAuthor, snapshot.CommitMessage, snapshot.Branch); err != nil {
		s.deployments.MarkFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunGitBuild: metadata: %w", err)
	}
	return s.deployments.RunBuildWithDockerfile(context.WithoutCancel(ctx), dep, app, snapshot.Source, snapshot.DockerfilePath)
}
