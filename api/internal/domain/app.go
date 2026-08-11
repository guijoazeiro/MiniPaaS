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
	AppStatusStopped AppStatus = "stopped"
)

type App struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Status         AppStatus `json:"status"`
	ContainerState string    `json:"container_state,omitempty"`
	PublicURL      string    `json:"public_url,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusBuilding   DeploymentStatus = "building"
	DeploymentStatusRunning    DeploymentStatus = "running"
	DeploymentStatusFailed     DeploymentStatus = "failed"
	DeploymentStatusSuperseded DeploymentStatus = "superseded"
	DeploymentStatusRolledBack DeploymentStatus = "rolled_back"
	DeploymentStatusStopped    DeploymentStatus = "stopped"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type EnvVarKey struct {
	Key       string    `json:"key"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Deployment struct {
	ID            uuid.UUID        `json:"id"`
	AppID         uuid.UUID        `json:"app_id"`
	ImageTag      string           `json:"image_tag"`
	Status        DeploymentStatus `json:"status"`
	ContainerID   string           `json:"container_id,omitempty"`
	Port          int              `json:"port,omitempty"`
	CommitSHA     string           `json:"commit_sha,omitempty"`
	SourceType    string           `json:"source_type"`
	Repository    string           `json:"repository,omitempty"`
	Branch        string           `json:"branch,omitempty"`
	CommitAuthor  string           `json:"commit_author,omitempty"`
	CommitMessage string           `json:"commit_message,omitempty"`
	DurationMs    int              `json:"duration_ms,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	FinishedAt    *time.Time       `json:"finished_at,omitempty"`
}

type GitSource struct {
	AppID          uuid.UUID `json:"app_id"`
	Repository     string    `json:"repository"`
	Branch         string    `json:"branch"`
	BuildContext   string    `json:"build_context"`
	DockerfilePath string    `json:"dockerfile_path"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
