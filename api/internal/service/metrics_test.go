package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type metricsTestDeployments struct {
	customDomainTestDeployments
	items []domain.Deployment
}

func (s *metricsTestDeployments) ListByApp(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return s.items, nil
}

type metricsTestHealth struct {
	logs []domain.DeploymentLog
}

func (s metricsTestHealth) ListHealthCheckFailures(context.Context, uuid.UUID, int) ([]domain.DeploymentLog, error) {
	return s.logs, nil
}

type metricsTestDocker struct {
	snapshot docker.ContainerMetrics
}

func (d metricsTestDocker) InspectMetrics(context.Context, string) (docker.ContainerMetrics, error) {
	return d.snapshot, nil
}

func TestSummarizeDeployments(t *testing.T) {
	got := summarizeDeployments([]domain.Deployment{
		{Status: domain.DeploymentStatusRunning, DurationMs: 100},
		{Status: domain.DeploymentStatusSuperseded, DurationMs: 200},
		{Status: domain.DeploymentStatusFailed, DurationMs: 300},
		{Status: domain.DeploymentStatusBuilding},
	})
	if got.Total != 4 || got.Successful != 2 || got.Failed != 1 || got.InProgress != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if math.Abs(got.SuccessRate-66.66666666666667) > 0.0001 || got.AverageDurationMs != 200 {
		t.Fatalf("summary rates = %#v", got)
	}
}

func TestMetricsGetIncludesRuntimeAndHealthFailures(t *testing.T) {
	appID, deploymentID := uuid.New(), uuid.New()
	started := time.Now().UTC().Add(-2 * time.Minute)
	svc := NewMetricsService(
		&customDomainTestApps{app: domain.App{ID: appID, Name: "api", Status: domain.AppStatusRunning}},
		&metricsTestDeployments{items: []domain.Deployment{{ID: deploymentID, AppID: appID, Status: domain.DeploymentStatusRunning, ContainerID: "container", DurationMs: 120}}},
		metricsTestHealth{logs: []domain.DeploymentLog{{DeploymentID: deploymentID, Message: "container exited", CreatedAt: started}}},
		metricsTestDocker{snapshot: docker.ContainerMetrics{State: "running", Running: true, RestartCount: 2, StartedAt: &started, CPUPercent: 12.5, MemoryUsageBytes: 10, MemoryLimitBytes: 100, MemoryPercent: 10}},
	)

	got, err := svc.Get(context.Background(), "api")
	if err != nil {
		t.Fatal(err)
	}
	if got.AppName != "api" || got.Runtime == nil || got.Runtime.State != "running" {
		t.Fatalf("metrics = %#v", got)
	}
	if got.Runtime.RestartCount != 2 || got.Runtime.MemoryPercent != 10 || got.Deployments.Total != 1 {
		t.Fatalf("runtime/deployments = %#v / %#v", got.Runtime, got.Deployments)
	}
	if len(got.HealthCheckFailures) != 1 || got.HealthCheckFailures[0].Message != "container exited" {
		t.Fatalf("health failures = %#v", got.HealthCheckFailures)
	}
	if got.Runtime.UptimeSeconds < 119 {
		t.Fatalf("uptime = %d", got.Runtime.UptimeSeconds)
	}
}
