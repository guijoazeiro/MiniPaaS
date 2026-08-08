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
	ListByApp(ctx context.Context, appID uuid.UUID, limit int) ([]domain.Deployment, error)
	ListForRetention(ctx context.Context, appID uuid.UUID, keep int) ([]domain.Deployment, error)
	UpdateRunning(ctx context.Context, id uuid.UUID, containerID string, port int, imageTag string, durationMs int) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.DeploymentStatus) error
}

type RollbackStore interface {
	Record(ctx context.Context, appID, fromDep, toDep uuid.UUID, triggeredBy string) error
}
