package domain

import (
	"time"

	"github.com/google/uuid"
)

type AppStatus string

const (
	AppStatusIdle    AppStatus = "idle"
	AppStatusRunning AppStatus = "running"
	AppStatusFailed  AppStatus = "failed"
)

type App struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Status    AppStatus `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusBuilding   DeploymentStatus = "building"
	DeploymentStatusRunning    DeploymentStatus = "running"
	DeploymentStatusFailed     DeploymentStatus = "failed"
	DeploymentStatusRolledBack DeploymentStatus = "rolled_back"
)

type Deployment struct {
	ID          uuid.UUID        `json:"id"`
	AppID       uuid.UUID        `json:"app_id"`
	ImageTag    string           `json:"image_tag"`
	Status      DeploymentStatus `json:"status"`
	ContainerID string           `json:"container_id,omitempty"`
	Port        int              `json:"port,omitempty"`
	CommitSHA   string           `json:"commit_sha,omitempty"`
	DurationMs  int              `json:"duration_ms,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	FinishedAt  *time.Time       `json:"finished_at,omitempty"`
}
