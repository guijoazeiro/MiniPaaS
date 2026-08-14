package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type rolloutStore struct {
	active    domain.Deployment
	target    domain.Deployment
	events    *[]string
	candidate string
}

func (s *rolloutStore) Create(context.Context, uuid.UUID, string) (domain.Deployment, error) {
	return s.target, nil
}
func (s *rolloutStore) GetByID(_ context.Context, id uuid.UUID) (domain.Deployment, error) {
	if id == s.target.ID {
		return s.target, nil
	}
	if id == s.active.ID {
		return s.active, nil
	}
	return domain.Deployment{}, domain.ErrDeploymentNotFound
}
func (s *rolloutStore) GetActive(context.Context, uuid.UUID) (domain.Deployment, error) {
	if s.active.ID == uuid.Nil {
		return domain.Deployment{}, domain.ErrDeploymentNotFound
	}
	return s.active, nil
}
func (s *rolloutStore) ListRunning(context.Context) ([]domain.Deployment, error) { return nil, nil }
func (s *rolloutStore) ListByApp(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return []domain.Deployment{s.target}, nil
}
func (s *rolloutStore) ListAll(context.Context, string, string, int, int) ([]domain.DeploymentListItem, error) {
	return nil, nil
}
func (s *rolloutStore) CountAll(context.Context, string, string) (int64, error) { return 0, nil }
func (s *rolloutStore) ListForRetention(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return nil, nil
}
func (s *rolloutStore) UpdateRunning(_ context.Context, id uuid.UUID, containerID string, port int, imageTag string, durationMs int) error {
	s.target.ID = id
	s.target.Status = domain.DeploymentStatusRunning
	s.target.ContainerID = containerID
	s.target.Port = port
	s.target.ImageTag = imageTag
	s.target.DurationMs = durationMs
	s.active = s.target
	s.events = appendEvent(s.events, "promote-legacy")
	return nil
}
func (s *rolloutStore) UpdateStatus(_ context.Context, id uuid.UUID, status domain.DeploymentStatus) error {
	if id == s.target.ID {
		s.target.Status = status
	}
	if id == s.active.ID {
		s.active.Status = status
	}
	s.events = appendEvent(s.events, "status:"+string(status))
	return nil
}

func (s *rolloutStore) RequestCancel(_ context.Context, id uuid.UUID) (domain.Deployment, error) {
	s.target.ID = id
	s.target.Status = domain.DeploymentStatusCancelRequested
	s.target.CancelRequested = true
	return s.target, nil
}

func (s *rolloutStore) MarkCancelled(_ context.Context, id uuid.UUID) error {
	s.target.ID = id
	s.target.Status = domain.DeploymentStatusCancelled
	s.target.CancelRequested = true
	return nil
}

func (s *rolloutStore) CreateGit(context.Context, uuid.UUID, string, string, string) (domain.Deployment, error) {
	return s.target, nil
}

func (s *rolloutStore) UpdateGitMetadata(context.Context, uuid.UUID, string, string, string, string) error {
	return nil
}

func (s *rolloutStore) CreateGitTriggered(context.Context, uuid.UUID, string, string, string, string, string) (domain.Deployment, error) {
	return s.target, nil
}

func (s *rolloutStore) CreateGitRetry(context.Context, uuid.UUID, string, string, string, uuid.UUID, int) (domain.Deployment, error) {
	return s.target, nil
}
func (s *rolloutStore) UpdateCandidate(_ context.Context, id uuid.UUID, containerID string, port int) error {
	s.target.ID = id
	s.target.CandidateContainerID = containerID
	s.target.CandidatePort = port
	s.candidate = containerID
	s.events = appendEvent(s.events, "candidate-persist")
	return nil
}
func (s *rolloutStore) PromoteCandidate(_ context.Context, id uuid.UUID, containerID string, port int, imageTag string, durationMs int) error {
	s.target.ID = id
	s.target.Status = domain.DeploymentStatusRunning
	s.target.ContainerID = containerID
	s.target.Port = port
	s.target.ImageTag = imageTag
	s.target.DurationMs = durationMs
	s.target.CandidateContainerID = ""
	s.target.CandidatePort = 0
	s.active = s.target
	s.candidate = ""
	s.events = appendEvent(s.events, "promote")
	return nil
}
func (s *rolloutStore) ClearCandidate(_ context.Context, id uuid.UUID) error {
	if id == s.target.ID {
		s.target.CandidateContainerID = ""
		s.target.CandidatePort = 0
	}
	s.candidate = ""
	s.events = appendEvent(s.events, "candidate-clear")
	return nil
}
func (s *rolloutStore) ListCandidates(context.Context) ([]domain.Deployment, error) { return nil, nil }

func appendEvent(events *[]string, value string) *[]string {
	if events != nil {
		*events = append(*events, value)
	}
	return events
}

type rolloutDocker struct {
	events       *[]string
	readinessErr error
}

func (d *rolloutDocker) BuildImage(context.Context, io.Reader, string) (io.ReadCloser, error) {
	*d.events = append(*d.events, "build")
	return io.NopCloser(strings.NewReader("build ok\n")), nil
}
func (d *rolloutDocker) RunContainer(context.Context, docker.RunOptions) (docker.ContainerInfo, error) {
	*d.events = append(*d.events, "candidate-start")
	return docker.ContainerInfo{ID: "candidate-container", Port: 49000}, nil
}
func (d *rolloutDocker) WaitContainerReady(context.Context, string, int, docker.ReadinessOptions) error {
	*d.events = append(*d.events, "candidate-ready")
	return d.readinessErr
}
func (d *rolloutDocker) StopContainer(_ context.Context, id string) error {
	*d.events = append(*d.events, "stop:"+id)
	return nil
}
func (d *rolloutDocker) RemoveContainer(_ context.Context, id string) error {
	*d.events = append(*d.events, "remove:"+id)
	return nil
}
func (d *rolloutDocker) RemoveImage(context.Context, string) error { return nil }

type rolloutCaddy struct{ events *[]string }

func (c *rolloutCaddy) SwitchRoute(_ context.Context, _ string, port int) (string, error) {
	*c.events = append(*c.events, "switch:"+itoa(port))
	return "https://app.example.dev", nil
}
func (c *rolloutCaddy) UpsertRoute(_ context.Context, _ string, port int) (string, error) {
	*c.events = append(*c.events, "upsert:"+itoa(port))
	return "https://app.example.dev", nil
}
func (c *rolloutCaddy) RemoveRoute(context.Context, string) error { return nil }

func itoa(value int) string {
	return strconv.Itoa(value)
}

func TestRunBuildPromotesCandidateBeforeStoppingPrevious(t *testing.T) {
	appID := uuid.New()
	depID := uuid.New()
	events := []string{}
	store := &rolloutStore{
		active: domain.Deployment{ID: uuid.New(), AppID: appID, ContainerID: "old-container", Port: 48000, Status: domain.DeploymentStatusRunning},
		target: domain.Deployment{ID: depID, AppID: appID, ImageTag: "app:new"},
		events: &events,
	}
	apps := &rollbackAppStore{app: domain.App{ID: appID, Name: "app"}}
	svc := NewDeploymentService(store, apps, &rollbackRecorder{}, &rolloutDocker{events: &events}, &rolloutCaddy{events: &events}, &rollbackEnv{}, 5, "on-failure", 3, slog.New(slog.NewTextHandler(io.Discard, nil)), DeploymentServiceOptions{ReadyTimeout: time.Second})

	if err := svc.RunBuild(context.Background(), store.target, apps.app, strings.NewReader("tar")); err != nil {
		t.Fatal(err)
	}

	ready := indexOf(events, "candidate-ready")
	switchAt := indexOf(events, "switch:49000")
	promote := indexOf(events, "promote")
	stop := indexOf(events, "stop:old-container")
	if ready < 0 || switchAt < 0 || promote < 0 || stop < 0 {
		t.Fatalf("rollout events = %v", events)
	}
	if !(ready < switchAt && switchAt < promote && promote < stop) {
		t.Fatalf("rollout order = %v", events)
	}
	if store.active.ContainerID != "candidate-container" || store.target.CandidateContainerID != "" {
		t.Fatalf("active = %#v, target = %#v", store.active, store.target)
	}
}

func TestRunBuildReadinessFailureKeepsPreviousContainer(t *testing.T) {
	appID := uuid.New()
	depID := uuid.New()
	events := []string{}
	store := &rolloutStore{
		active: domain.Deployment{ID: uuid.New(), AppID: appID, ContainerID: "old-container", Port: 48000, Status: domain.DeploymentStatusRunning},
		target: domain.Deployment{ID: depID, AppID: appID, ImageTag: "app:new"},
		events: &events,
	}
	svc := NewDeploymentService(store, &rollbackAppStore{app: domain.App{ID: appID, Name: "app"}}, &rollbackRecorder{}, &rolloutDocker{events: &events, readinessErr: errors.New("health timeout")}, &rolloutCaddy{events: &events}, &rollbackEnv{}, 5, "on-failure", 3, slog.New(slog.NewTextHandler(io.Discard, nil)), DeploymentServiceOptions{ReadyTimeout: time.Second})

	err := svc.RunBuild(context.Background(), store.target, domain.App{ID: appID, Name: "app"}, strings.NewReader("tar"))
	if err == nil || !strings.Contains(err.Error(), "candidate readiness") {
		t.Fatalf("RunBuild() error = %v", err)
	}
	if indexOf(events, "switch:49000") >= 0 {
		t.Fatalf("failed candidate was published: %v", events)
	}
	if indexOf(events, "stop:candidate-container") < 0 || indexOf(events, "remove:candidate-container") < 0 {
		t.Fatalf("failed candidate was not cleaned: %v", events)
	}
	if store.active.ContainerID != "old-container" || store.target.Status != domain.DeploymentStatusFailed {
		t.Fatalf("active = %#v, target = %#v", store.active, store.target)
	}
}

func TestAcquireRolloutSerializesSameApplication(t *testing.T) {
	svc := NewDeploymentService(nil, nil, nil, nil, nil, nil, 1, "no", 0, slog.Default())
	appID := uuid.New()
	release, err := svc.acquireRollout(context.Background(), appID)
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan struct{})
	go func() {
		second, secondErr := svc.acquireRollout(context.Background(), appID)
		if secondErr == nil {
			second()
			close(acquired)
		}
	}()
	select {
	case <-acquired:
		t.Fatal("second rollout acquired before first was released")
	case <-time.After(25 * time.Millisecond):
	}
	release()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("second rollout did not acquire after first was released")
	}
}

func indexOf(values []string, wanted string) int {
	for i, value := range values {
		if value == wanted {
			return i
		}
	}
	return -1
}
