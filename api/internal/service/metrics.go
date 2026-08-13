package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type MetricsDocker interface {
	InspectMetrics(ctx context.Context, id string) (docker.ContainerMetrics, error)
}

type HealthFailureStore interface {
	ListHealthCheckFailures(ctx context.Context, appID uuid.UUID, limit int) ([]domain.DeploymentLog, error)
}

type MetricsService struct {
	apps        store.AppStore
	deployments store.DeploymentStore
	health      HealthFailureStore
	docker      MetricsDocker
}

func NewMetricsService(apps store.AppStore, deployments store.DeploymentStore, health HealthFailureStore, dockerClient MetricsDocker) *MetricsService {
	return &MetricsService{apps: apps, deployments: deployments, health: health, docker: dockerClient}
}

func (s *MetricsService) Get(ctx context.Context, appName string) (domain.AppMetrics, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.AppMetrics{}, err
	}
	deployments, err := s.deployments.ListByApp(ctx, app.ID, 200)
	if err != nil {
		return domain.AppMetrics{}, fmt.Errorf("service.GetAppMetrics: list deployments: %w", err)
	}

	metrics := domain.AppMetrics{
		AppName:             app.Name,
		CollectedAt:         time.Now().UTC(),
		Deployments:         summarizeDeployments(deployments),
		HealthCheckFailures: []domain.HealthCheckFailure{},
	}
	if s.health != nil {
		logs, logErr := s.health.ListHealthCheckFailures(ctx, app.ID, 5)
		if logErr != nil {
			return domain.AppMetrics{}, fmt.Errorf("service.GetAppMetrics: health failures: %w", logErr)
		}
		metrics.HealthCheckFailures = make([]domain.HealthCheckFailure, len(logs))
		for i, item := range logs {
			metrics.HealthCheckFailures[i] = domain.HealthCheckFailure{DeploymentID: item.DeploymentID, Message: item.Message, CreatedAt: item.CreatedAt}
		}
	}

	deployment := metricsDeployment(deployments)
	if deployment == nil {
		state := "unknown"
		if app.Status == domain.AppStatusStopped {
			state = "stopped"
		}
		metrics.Runtime = &domain.RuntimeMetrics{State: state}
		return metrics, nil
	}

	runtime := &domain.RuntimeMetrics{ContainerID: deployment.ContainerID, State: "unknown"}
	if s.docker == nil {
		runtime.State = string(app.Status)
		metrics.Runtime = runtime
		return metrics, nil
	}
	snapshot, inspectErr := s.docker.InspectMetrics(ctx, deployment.ContainerID)
	if inspectErr != nil {
		// A stopped or cleaned-up deployment should not make the whole metrics
		// endpoint fail; the deployment summary is still useful in that state.
		runtime.State = "missing"
		metrics.Runtime = runtime
		return metrics, nil
	}
	runtime.State = snapshot.State
	runtime.RestartCount = snapshot.RestartCount
	runtime.StartedAt = snapshot.StartedAt
	runtime.CPUPercent = snapshot.CPUPercent
	runtime.MemoryUsageBytes = snapshot.MemoryUsageBytes
	runtime.MemoryLimitBytes = snapshot.MemoryLimitBytes
	runtime.MemoryPercent = snapshot.MemoryPercent
	runtime.NetworkRxBytes = snapshot.NetworkRxBytes
	runtime.NetworkTxBytes = snapshot.NetworkTxBytes
	runtime.BlockReadBytes = snapshot.BlockReadBytes
	runtime.BlockWriteBytes = snapshot.BlockWriteBytes
	runtime.Pids = snapshot.Pids
	if snapshot.Running && snapshot.StartedAt != nil {
		uptime := int64(time.Since(*snapshot.StartedAt).Seconds())
		if uptime > 0 {
			runtime.UptimeSeconds = uptime
		}
	}
	metrics.Runtime = runtime
	return metrics, nil
}

func metricsDeployment(items []domain.Deployment) *domain.Deployment {
	for i := range items {
		if items[i].Status == domain.DeploymentStatusRunning && items[i].ContainerID != "" {
			return &items[i]
		}
	}
	for i := range items {
		if items[i].ContainerID != "" {
			return &items[i]
		}
	}
	return nil
}

func summarizeDeployments(items []domain.Deployment) domain.DeploymentMetrics {
	result := domain.DeploymentMetrics{Total: len(items)}
	var durationTotal int64
	var durationCount int64
	for _, item := range items {
		switch item.Status {
		case domain.DeploymentStatusRunning, domain.DeploymentStatusSuperseded, domain.DeploymentStatusRolledBack, domain.DeploymentStatusStopped:
			result.Successful++
		case domain.DeploymentStatusFailed, domain.DeploymentStatusCancelled:
			result.Failed++
		default:
			result.InProgress++
		}
		if item.DurationMs > 0 {
			durationTotal += int64(item.DurationMs)
			durationCount++
		}
	}
	completed := result.Successful + result.Failed
	if completed > 0 {
		result.SuccessRate = float64(result.Successful) / float64(completed) * 100
	}
	if durationCount > 0 {
		result.AverageDurationMs = durationTotal / durationCount
	}
	return result
}
