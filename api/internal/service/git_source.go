package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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

func (s *GitSourceService) SetAutoDeploy(ctx context.Context, appName string, enabled bool) (domain.GitSource, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.GitSource{}, err
	}
	source, err := s.sources.Get(ctx, app.ID)
	if err != nil {
		return domain.GitSource{}, err
	}
	if enabled && source.AccessMode != domain.GitAccessGitHubApp {
		return domain.GitSource{}, domain.ErrGitHubAutoDeployRequiresApp
	}
	return s.sources.SetAutoDeploy(ctx, app.ID, enabled)
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

func (s *GitDeploymentService) CreateTriggered(ctx context.Context, appName string, source domain.GitSource, branch, deliveryID string) (domain.Deployment, domain.App, error) {
	if branch == "" {
		branch = source.Branch
	}
	branch, err := sourcegit.NormalizeBranch(branch)
	if err != nil {
		return domain.Deployment{}, domain.App{}, err
	}
	return s.deployments.CreateGitTriggered(ctx, appName, source.Repository, branch, domain.DeploymentTriggerWebhook, deliveryID)
}

func (s *GitDeploymentService) Retry(ctx context.Context, appName string, targetID uuid.UUID) (domain.Deployment, domain.App, domain.GitSource, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, err
	}
	target, err := s.deployments.Get(ctx, targetID)
	if err != nil {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, err
	}
	if target.AppID != app.ID {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, domain.ErrDeploymentNotFound
	}
	if target.SourceType != "git" {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, domain.ErrDeploymentRetryUnavailable
	}
	if target.Status != domain.DeploymentStatusFailed && target.Status != domain.DeploymentStatusCancelled {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, domain.ErrDeploymentNotRetryable
	}
	source, err := s.sources.Get(ctx, app.ID)
	if err != nil {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, err
	}
	branch := target.Branch
	if branch == "" {
		branch = source.Branch
	}
	attempt := target.Attempt + 1
	if attempt < 2 {
		attempt = 2
	}
	tag := fmt.Sprintf("%s:ts-%d", app.Name, time.Now().UnixNano())
	dep, err := s.deployments.deps.CreateGitRetry(ctx, app.ID, tag, source.Repository, branch, target.ID, attempt)
	if err != nil {
		return domain.Deployment{}, domain.App{}, domain.GitSource{}, fmt.Errorf("service.Retry: %w", err)
	}
	s.deployments.event(ctx, dep.ID, "queued", "system", fmt.Sprintf("Retry da tentativa %d solicitado", target.Attempt))
	return dep, app, source, nil
}

func (s *GitDeploymentService) Run(ctx context.Context, dep domain.Deployment, app domain.App, source domain.GitSource, branch string) error {
	runCtx, done := s.deployments.beginExecution(ctx, dep.ID)
	defer done()
	return s.run(runCtx, dep, app, source, branch)
}

func (s *GitDeploymentService) run(ctx context.Context, dep domain.Deployment, app domain.App, source domain.GitSource, branch string) error {
	if branch == "" {
		branch = source.Branch
	}
	if err := s.deployments.checkCancelled(ctx, dep, app.ID); err != nil {
		return err
	}
	cloneCtx, cancel := context.WithTimeout(ctx, s.cloneTimeout)
	s.deployments.event(ctx, dep.ID, "cloning", "system", fmt.Sprintf("Clonando %s (%s)", source.Repository, branch))
	snapshot, err := s.preparer.Prepare(cloneCtx, source, branch)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return s.deployments.finishCancelled(ctx, dep, app.ID, "clone interrompido")
		}
		s.deployments.event(ctx, dep.ID, "error", "stderr", err.Error())
		s.deployments.MarkFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunGitBuild: prepare source: %w", err)
	}
	defer snapshot.Source.Close()
	s.deployments.event(ctx, dep.ID, "cloning", "system", fmt.Sprintf("Commit %s preparado", snapshot.CommitSHA))
	if err := s.deployments.UpdateGitMetadata(ctx, dep.ID, snapshot.CommitSHA, snapshot.CommitAuthor, snapshot.CommitMessage, snapshot.Branch); err != nil {
		if ctx.Err() != nil {
			return s.deployments.finishCancelled(ctx, dep, app.ID, "metadados interrompidos")
		}
		s.deployments.MarkFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunGitBuild: metadata: %w", err)
	}
	return s.deployments.runBuildWithDockerfile(ctx, dep, app, snapshot.Source, snapshot.DockerfilePath)
}
