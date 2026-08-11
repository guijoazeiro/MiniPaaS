package health

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type fakeDeploymentStore struct {
	running       []domain.Deployment
	recent        []domain.Deployment
	active        domain.Deployment
	activeErr     error
	statusUpdates []uuid.UUID
}

func (f *fakeDeploymentStore) Create(context.Context, uuid.UUID, string) (domain.Deployment, error) {
	return domain.Deployment{}, errors.New("not implemented")
}
func (f *fakeDeploymentStore) GetByID(context.Context, uuid.UUID) (domain.Deployment, error) {
	return domain.Deployment{}, errors.New("not implemented")
}
func (f *fakeDeploymentStore) GetActive(context.Context, uuid.UUID) (domain.Deployment, error) {
	return f.active, f.activeErr
}
func (f *fakeDeploymentStore) ListByApp(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return f.recent, nil
}
func (f *fakeDeploymentStore) ListForRetention(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeDeploymentStore) ListRunning(context.Context) ([]domain.Deployment, error) {
	return f.running, nil
}
func (f *fakeDeploymentStore) UpdateRunning(context.Context, uuid.UUID, string, int, string, int) error {
	return errors.New("not implemented")
}
func (f *fakeDeploymentStore) UpdateStatus(_ context.Context, id uuid.UUID, _ domain.DeploymentStatus) error {
	f.statusUpdates = append(f.statusUpdates, id)
	return nil
}

type fakeAppStore struct{ statusUpdates []uuid.UUID }

func (f *fakeAppStore) Create(context.Context, string) (domain.App, error) {
	return domain.App{}, errors.New("not implemented")
}
func (f *fakeAppStore) GetByName(context.Context, string) (domain.App, error) {
	return domain.App{}, errors.New("not implemented")
}
func (f *fakeAppStore) GetByID(context.Context, uuid.UUID) (domain.App, error) {
	return domain.App{}, errors.New("not implemented")
}
func (f *fakeAppStore) List(context.Context) ([]domain.App, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeAppStore) UpdateStatus(_ context.Context, id uuid.UUID, _ domain.AppStatus) error {
	f.statusUpdates = append(f.statusUpdates, id)
	return nil
}
func (f *fakeAppStore) UpdatePublicURL(context.Context, uuid.UUID, string) error {
	return errors.New("not implemented")
}
func (f *fakeAppStore) Delete(context.Context, uuid.UUID) error { return errors.New("not implemented") }

type fakeInspector struct {
	states map[string]docker.ContainerState
	errs   map[string]error
}

func (f fakeInspector) InspectContainer(_ context.Context, id string) (docker.ContainerState, error) {
	return f.states[id], f.errs[id]
}

func TestCheckerMarksOnlyUnhealthyDeploymentsFailed(t *testing.T) {
	appID := uuid.New()
	exited := domain.Deployment{ID: uuid.New(), AppID: appID, ContainerID: "exited", Status: domain.DeploymentStatusRunning}
	running := domain.Deployment{ID: uuid.New(), AppID: uuid.New(), ContainerID: "running", Status: domain.DeploymentStatusRunning}
	missing := domain.Deployment{ID: uuid.New(), AppID: uuid.New(), ContainerID: "missing", Status: domain.DeploymentStatusRunning}
	deps := &fakeDeploymentStore{running: []domain.Deployment{exited, running, missing}}
	apps := &fakeAppStore{}
	checker := New(deps, apps, fakeInspector{states: map[string]docker.ContainerState{
		"exited":  {Status: "exited"},
		"running": {Status: "running", Running: true},
		"missing": {Status: "missing"},
	}}, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))

	checker.Check(context.Background())

	if len(deps.statusUpdates) != 2 {
		t.Fatalf("deployment updates = %d, want 2", len(deps.statusUpdates))
	}
	if deps.statusUpdates[0] != exited.ID || deps.statusUpdates[1] != missing.ID {
		t.Fatalf("updated deployments = %v, want [%s %s]", deps.statusUpdates, exited.ID, missing.ID)
	}
	if len(apps.statusUpdates) != 2 || apps.statusUpdates[0] != exited.AppID || apps.statusUpdates[1] != missing.AppID {
		t.Fatalf("updated apps = %v, want unhealthy deployment apps", apps.statusUpdates)
	}
}

func TestCheckerContainerState(t *testing.T) {
	appID := uuid.New()
	deps := &fakeDeploymentStore{active: domain.Deployment{AppID: appID, ContainerID: "abc"}}
	checker := New(deps, &fakeAppStore{}, fakeInspector{states: map[string]docker.ContainerState{
		"abc": {Status: "running", Running: true},
	}}, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))

	state, err := checker.ContainerState(context.Background(), appID)
	if err != nil {
		t.Fatal(err)
	}
	if state != "running" {
		t.Fatalf("ContainerState() = %q, want running", state)
	}
}

func TestCheckerContainerStateFallsBackToLatestDeployment(t *testing.T) {
	appID := uuid.New()
	deps := &fakeDeploymentStore{
		activeErr: domain.ErrDeploymentNotFound,
		recent:    []domain.Deployment{{AppID: appID, ContainerID: "exited-container", Status: domain.DeploymentStatusFailed}},
	}
	checker := New(deps, &fakeAppStore{}, fakeInspector{states: map[string]docker.ContainerState{
		"exited-container": {Status: "exited"},
	}}, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))

	state, err := checker.ContainerState(context.Background(), appID)
	if err != nil {
		t.Fatal(err)
	}
	if state != "exited" {
		t.Fatalf("ContainerState() = %q, want exited", state)
	}
}

func TestCheckerContainerStateReportsStoppedWithoutInspectingRemovedContainer(t *testing.T) {
	appID := uuid.New()
	deps := &fakeDeploymentStore{
		activeErr: domain.ErrDeploymentNotFound,
		recent:    []domain.Deployment{{AppID: appID, ContainerID: "removed", Status: domain.DeploymentStatusStopped}},
	}
	checker := New(deps, &fakeAppStore{}, fakeInspector{states: map[string]docker.ContainerState{}}, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	state, err := checker.ContainerState(context.Background(), appID)
	if err != nil {
		t.Fatal(err)
	}
	if state != "stopped" {
		t.Fatalf("ContainerState() = %q, want stopped", state)
	}
}
