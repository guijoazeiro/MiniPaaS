package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type rollbackDeploymentStore struct {
	target      domain.Deployment
	active      domain.Deployment
	activeCalls int
	statuses    []domain.DeploymentStatus
}

func (s *rollbackDeploymentStore) Create(context.Context, uuid.UUID, string) (domain.Deployment, error) {
	return domain.Deployment{}, nil
}
func (s *rollbackDeploymentStore) GetByID(context.Context, uuid.UUID) (domain.Deployment, error) {
	return s.target, nil
}
func (s *rollbackDeploymentStore) GetActive(context.Context, uuid.UUID) (domain.Deployment, error) {
	s.activeCalls++
	return s.active, nil
}
func (s *rollbackDeploymentStore) ListRunning(context.Context) ([]domain.Deployment, error) {
	return nil, nil
}
func (s *rollbackDeploymentStore) ListByApp(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return nil, nil
}
func (s *rollbackDeploymentStore) ListForRetention(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return nil, nil
}
func (s *rollbackDeploymentStore) UpdateRunning(context.Context, uuid.UUID, string, int, string, int) error {
	return nil
}
func (s *rollbackDeploymentStore) UpdateStatus(_ context.Context, _ uuid.UUID, status domain.DeploymentStatus) error {
	s.statuses = append(s.statuses, status)
	return nil
}

type rollbackAppStore struct{ app domain.App }

func (s *rollbackAppStore) Create(context.Context, string) (domain.App, error) {
	return domain.App{}, nil
}
func (s *rollbackAppStore) GetByName(context.Context, string) (domain.App, error)  { return s.app, nil }
func (s *rollbackAppStore) GetByID(context.Context, uuid.UUID) (domain.App, error) { return s.app, nil }
func (s *rollbackAppStore) List(context.Context) ([]domain.App, error)             { return nil, nil }
func (s *rollbackAppStore) UpdateStatus(context.Context, uuid.UUID, domain.AppStatus) error {
	return nil
}
func (s *rollbackAppStore) UpdatePublicURL(context.Context, uuid.UUID, string) error { return nil }
func (s *rollbackAppStore) Delete(context.Context, uuid.UUID) error                  { return nil }

type rollbackDocker struct{ mutations int }

func (d *rollbackDocker) BuildImage(context.Context, io.Reader, string) (io.ReadCloser, error) {
	return nil, nil
}
func (d *rollbackDocker) RunContainer(context.Context, docker.RunOptions) (docker.ContainerInfo, error) {
	d.mutations++
	return docker.ContainerInfo{}, nil
}
func (d *rollbackDocker) StopContainer(context.Context, string) error   { d.mutations++; return nil }
func (d *rollbackDocker) RemoveContainer(context.Context, string) error { d.mutations++; return nil }
func (d *rollbackDocker) RemoveImage(context.Context, string) error     { return nil }

type rollbackCaddy struct{}

func (*rollbackCaddy) UpsertRoute(context.Context, string, int) (string, error) { return "", nil }
func (*rollbackCaddy) RemoveRoute(context.Context, string) error                { return nil }

type rollbackEnv struct {
	err   error
	calls int
}

func (e *rollbackEnv) Decrypted(context.Context, uuid.UUID) (map[string]string, error) {
	e.calls++
	return nil, e.err
}

type rollbackRecorder struct{}

func (*rollbackRecorder) Record(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) error {
	return nil
}

func newRollbackService(deps *rollbackDeploymentStore, apps *rollbackAppStore, dk *rollbackDocker, env *rollbackEnv) *DeploymentService {
	return NewDeploymentService(deps, apps, &rollbackRecorder{}, dk, &rollbackCaddy{}, env, 5, "on-failure", 3, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestRollbackRejectsInvalidTargetBeforeSideEffects(t *testing.T) {
	appID := uuid.New()
	targetID := uuid.New()
	deps := &rollbackDeploymentStore{
		target: domain.Deployment{ID: targetID, AppID: appID, ImageTag: "app:failed", Status: domain.DeploymentStatusFailed},
		active: domain.Deployment{ID: uuid.New(), AppID: appID, ContainerID: "active-container", Status: domain.DeploymentStatusRunning},
	}
	dk := &rollbackDocker{}
	env := &rollbackEnv{}
	svc := newRollbackService(deps, &rollbackAppStore{app: domain.App{ID: appID, Name: "app"}}, dk, env)

	_, err := svc.Rollback(context.Background(), "app", targetID, "test")
	if !errors.Is(err, domain.ErrDeploymentNotRollbackable) {
		t.Fatalf("Rollback() error = %v, want %v", err, domain.ErrDeploymentNotRollbackable)
	}
	if deps.activeCalls != 0 || env.calls != 0 || dk.mutations != 0 || len(deps.statuses) != 0 {
		t.Fatalf("invalid rollback caused side effects: active_calls=%d env_calls=%d docker_mutations=%d statuses=%v", deps.activeCalls, env.calls, dk.mutations, deps.statuses)
	}
}

func TestRollbackResolvesEnvironmentBeforeStoppingActiveContainer(t *testing.T) {
	appID := uuid.New()
	targetID := uuid.New()
	deps := &rollbackDeploymentStore{
		target: domain.Deployment{ID: targetID, AppID: appID, ImageTag: "app:previous", Status: domain.DeploymentStatusSuperseded},
		active: domain.Deployment{ID: uuid.New(), AppID: appID, ContainerID: "active-container", Status: domain.DeploymentStatusRunning},
	}
	dk := &rollbackDocker{}
	envErr := errors.New("decrypt failed")
	env := &rollbackEnv{err: envErr}
	svc := newRollbackService(deps, &rollbackAppStore{app: domain.App{ID: appID, Name: "app"}}, dk, env)

	_, err := svc.Rollback(context.Background(), "app", targetID, "test")
	if !errors.Is(err, envErr) {
		t.Fatalf("Rollback() error = %v, want wrapped %v", err, envErr)
	}
	if deps.activeCalls != 0 || dk.mutations != 0 || len(deps.statuses) != 0 {
		t.Fatalf("failed prerequisite caused side effects: active_calls=%d docker_mutations=%d statuses=%v", deps.activeCalls, dk.mutations, deps.statuses)
	}
}
