package store

import (
	"context"

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

type DeploymentStore interface {
	Create(ctx context.Context, appID uuid.UUID, imageTag string) (domain.Deployment, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Deployment, error)
	GetActive(ctx context.Context, appID uuid.UUID) (domain.Deployment, error)
	ListByApp(ctx context.Context, appID uuid.UUID, limit int) ([]domain.Deployment, error)
	UpdateRunning(ctx context.Context, id uuid.UUID, containerID string, port int, imageTag string, durationMs int) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.DeploymentStatus) error
}
