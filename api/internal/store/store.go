package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type AppStore interface {
	Create(ctx context.Context, name string) (domain.App, error)
	GetByName(ctx context.Context, name string) (domain.App, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.App, error)
	List(ctx context.Context) ([]domain.App, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.AppStatus) error
	UpdatePublicURL(ctx context.Context, id uuid.UUID, url string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type UserStore interface {
	Create(ctx context.Context, username, passwordHash string) (domain.User, error)
	GetByUsername(ctx context.Context, username string) (domain.User, error)
	Count(ctx context.Context) (int64, error)
}

type EnvVarRecord struct {
	Key       string
	Value     []byte
	Nonce     []byte
	UpdatedAt time.Time
}

type EnvStore interface {
	Upsert(ctx context.Context, appID uuid.UUID, key string, value, nonce []byte) error
	ListKeys(ctx context.Context, appID uuid.UUID) ([]domain.EnvVarKey, error)
	ListRecords(ctx context.Context, appID uuid.UUID) ([]EnvVarRecord, error)
	Delete(ctx context.Context, appID uuid.UUID, key string) error
}

type DeploymentStore interface {
	Create(ctx context.Context, appID uuid.UUID, imageTag string) (domain.Deployment, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Deployment, error)
	GetActive(ctx context.Context, appID uuid.UUID) (domain.Deployment, error)
	ListRunning(ctx context.Context) ([]domain.Deployment, error)
	ListByApp(ctx context.Context, appID uuid.UUID, limit int) ([]domain.Deployment, error)
	ListAll(ctx context.Context, appName, status string, limit, offset int) ([]domain.DeploymentListItem, error)
	CountAll(ctx context.Context, appName, status string) (int64, error)
	ListForRetention(ctx context.Context, appID uuid.UUID, keep int) ([]domain.Deployment, error)
	UpdateRunning(ctx context.Context, id uuid.UUID, containerID string, port int, imageTag string, durationMs int) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.DeploymentStatus) error
}

type GitDeploymentStore interface {
	CreateGit(ctx context.Context, appID uuid.UUID, imageTag, repository, branch string) (domain.Deployment, error)
	UpdateGitMetadata(ctx context.Context, id uuid.UUID, commitSHA, author, message, branch string) error
}

type TriggeredGitDeploymentStore interface {
	CreateGitTriggered(ctx context.Context, appID uuid.UUID, imageTag, repository, branch, triggerType, deliveryID string) (domain.Deployment, error)
}

type GitSourceStore interface {
	Upsert(ctx context.Context, source domain.GitSource) (domain.GitSource, error)
	Get(ctx context.Context, appID uuid.UUID) (domain.GitSource, error)
	Delete(ctx context.Context, appID uuid.UUID) error
	SetAutoDeploy(ctx context.Context, appID uuid.UUID, enabled bool) (domain.GitSource, error)
	ListAutoDeployByRepository(ctx context.Context, repositoryID int64) ([]domain.GitSource, error)
}

type GitHubWebhookDeliveryStore interface {
	Claim(ctx context.Context, deliveryID, event string, repositoryID int64, commitSHA string) (bool, error)
	Complete(ctx context.Context, deliveryID, status, errorMessage string) error
}

type GitHubInstallationStore interface {
	Upsert(ctx context.Context, installation domain.GitHubInstallation) (domain.GitHubInstallation, error)
	Get(ctx context.Context, installationID int64) (domain.GitHubInstallation, error)
	List(ctx context.Context) ([]domain.GitHubInstallation, error)
}

type RollbackStore interface {
	Record(ctx context.Context, appID, fromDep, toDep uuid.UUID, triggeredBy string) error
}
