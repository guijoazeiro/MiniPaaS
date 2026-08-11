package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type rollbackDeploymentStore struct {
	target      domain.Deployment
	active      domain.Deployment
	activeErr   error
	recent      []domain.Deployment
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
	return s.active, s.activeErr
}
func (s *rollbackDeploymentStore) ListRunning(context.Context) ([]domain.Deployment, error) {
	return nil, nil
}
func (s *rollbackDeploymentStore) ListByApp(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return s.recent, nil
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

type rollbackAppStore struct {
	app      domain.App
	statuses []domain.AppStatus
	urls     []string
}

func (s *rollbackAppStore) Create(context.Context, string) (domain.App, error) {
	return domain.App{}, nil
}
func (s *rollbackAppStore) GetByName(context.Context, string) (domain.App, error)  { return s.app, nil }
func (s *rollbackAppStore) GetByID(context.Context, uuid.UUID) (domain.App, error) { return s.app, nil }
func (s *rollbackAppStore) List(context.Context) ([]domain.App, error)             { return nil, nil }

func (s *rollbackAppStore) UpdateStatus(_ context.Context, _ uuid.UUID, status domain.AppStatus) error {
	s.statuses = append(s.statuses, status)
	return nil
}
func (s *rollbackAppStore) UpdatePublicURL(_ context.Context, _ uuid.UUID, url string) error {
	s.urls = append(s.urls, url)
	return nil
}
func (s *rollbackAppStore) Delete(context.Context, uuid.UUID) error { return nil }

type rollbackDocker struct {
	mutations int
	buildErr  error
}

func (d *rollbackDocker) BuildImage(context.Context, io.Reader, string) (io.ReadCloser, error) {
	if d.buildErr != nil {
		return nil, d.buildErr
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func TestFailedBuildKeepsActiveAppRunning(t *testing.T) {
	appID := uuid.New()
	deps := &rollbackDeploymentStore{active: domain.Deployment{ID: uuid.New(), AppID: appID, ContainerID: "active", Status: domain.DeploymentStatusRunning}}
	apps := &rollbackAppStore{app: domain.App{ID: appID, Name: "app"}}
	dk := &rollbackDocker{buildErr: errors.New("build failed")}
	svc := newRollbackService(deps, apps, dk, &rollbackEnv{})

	err := svc.RunBuild(context.Background(), domain.Deployment{ID: uuid.New(), AppID: appID, ImageTag: "app:new"}, apps.app, strings.NewReader("tar"))
	if err == nil {
		t.Fatal("RunBuild() expected error")
	}
	if dk.mutations != 0 {
		t.Fatalf("docker mutations = %d", dk.mutations)
	}
	if len(apps.statuses) != 1 || apps.statuses[0] != domain.AppStatusRunning {
		t.Fatalf("app statuses = %v", apps.statuses)
	}
	if got := deps.statuses[len(deps.statuses)-1]; got != domain.DeploymentStatusFailed {
		t.Fatalf("last deployment status = %s", got)
	}
}

func TestDeployResolvesEnvironmentBeforeStoppingActiveContainer(t *testing.T) {
	appID := uuid.New()
	deps := &rollbackDeploymentStore{active: domain.Deployment{ID: uuid.New(), AppID: appID, ContainerID: "active", Status: domain.DeploymentStatusRunning}}
	apps := &rollbackAppStore{app: domain.App{ID: appID, Name: "app"}}
	dk := &rollbackDocker{}
	envErr := errors.New("decrypt failed")
	svc := newRollbackService(deps, apps, dk, &rollbackEnv{err: envErr})

	err := svc.RunBuild(context.Background(), domain.Deployment{ID: uuid.New(), AppID: appID, ImageTag: "app:new"}, apps.app, strings.NewReader("tar"))
	if !errors.Is(err, envErr) {
		t.Fatalf("RunBuild() error = %v", err)
	}
	if dk.mutations != 0 {
		t.Fatalf("failed prerequisite caused %d docker mutations", dk.mutations)
	}
}

func TestStopAppPreservesAppAndMarksRuntimeStopped(t *testing.T) {
	appID := uuid.New()
	deps := &rollbackDeploymentStore{active: domain.Deployment{ID: uuid.New(), AppID: appID, ContainerID: "active", Status: domain.DeploymentStatusRunning}}
	apps := &rollbackAppStore{app: domain.App{ID: appID, Name: "app", Status: domain.AppStatusRunning, PublicURL: "https://app.example"}}
	dk := &rollbackDocker{}
	svc := newRollbackService(deps, apps, dk, &rollbackEnv{})

	if err := svc.StopApp(context.Background(), apps.app); err != nil {
		t.Fatal(err)
	}
	if dk.mutations != 2 {
		t.Fatalf("docker mutations = %d, want stop and remove", dk.mutations)
	}
	if got := deps.statuses[len(deps.statuses)-1]; got != domain.DeploymentStatusStopped {
		t.Fatalf("deployment status = %s", got)
	}
	if len(apps.statuses) != 1 || apps.statuses[0] != domain.AppStatusStopped {
		t.Fatalf("app statuses = %v", apps.statuses)
	}
	if len(apps.urls) != 1 || apps.urls[0] != "" {
		t.Fatalf("public URLs = %v", apps.urls)
	}
}

func TestStopAppIsIdempotentAfterRuntimeWasStopped(t *testing.T) {
	appID := uuid.New()
	deps := &rollbackDeploymentStore{
		activeErr: domain.ErrDeploymentNotFound,
		recent:    []domain.Deployment{{ID: uuid.New(), AppID: appID, Status: domain.DeploymentStatusStopped, ContainerID: "removed"}},
	}
	apps := &rollbackAppStore{app: domain.App{ID: appID, Name: "app", Status: domain.AppStatusStopped}}
	dk := &rollbackDocker{}
	svc := newRollbackService(deps, apps, dk, &rollbackEnv{})

	if err := svc.StopApp(context.Background(), apps.app); err != nil {
		t.Fatal(err)
	}
	if dk.mutations != 0 {
		t.Fatalf("idempotent stop caused %d Docker mutations", dk.mutations)
	}
	if len(deps.statuses) != 0 {
		t.Fatalf("deployment statuses = %v", deps.statuses)
	}
}

func TestRollbackReactivatesStoppedDeployment(t *testing.T) {
	appID := uuid.New()
	targetID := uuid.New()
	deps := &rollbackDeploymentStore{
		target:    domain.Deployment{ID: targetID, AppID: appID, ImageTag: "app:stopped", Status: domain.DeploymentStatusStopped},
		activeErr: domain.ErrDeploymentNotFound,
	}
	apps := &rollbackAppStore{app: domain.App{ID: appID, Name: "app", Status: domain.AppStatusStopped}}
	dk := &rollbackDocker{}
	svc := newRollbackService(deps, apps, dk, &rollbackEnv{})

	restored, err := svc.Rollback(context.Background(), "app", targetID, "dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != targetID {
		t.Fatalf("restored deployment = %s", restored.ID)
	}
	if dk.mutations != 1 {
		t.Fatalf("docker mutations = %d, want one container start", dk.mutations)
	}
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
