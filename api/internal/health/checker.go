package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type ContainerInspector interface {
	InspectContainer(ctx context.Context, id string) (docker.ContainerState, error)
}

type DeploymentLogWriter interface {
	Append(ctx context.Context, deploymentID uuid.UUID, stage, stream, message string) (domain.DeploymentLog, error)
}

type Checker struct {
	deps     store.DeploymentStore
	apps     store.AppStore
	docker   ContainerInspector
	interval time.Duration
	log      *slog.Logger
	logs     DeploymentLogWriter
	wg       sync.WaitGroup
}

func New(deps store.DeploymentStore, apps store.AppStore, docker ContainerInspector, interval time.Duration, log *slog.Logger, logs ...DeploymentLogWriter) *Checker {
	var writer DeploymentLogWriter
	if len(logs) > 0 { writer = logs[0] }
	return &Checker{deps: deps, apps: apps, docker: docker, interval: interval, log: log, logs: writer}
}

func (c *Checker) Start(ctx context.Context) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.run(ctx)
	}()
}

func (c *Checker) Stop() { c.wg.Wait() }

func (c *Checker) run(ctx context.Context) {
	c.Check(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Check(ctx)
		}
	}
}

func (c *Checker) Check(ctx context.Context) {
	deployments, err := c.deps.ListRunning(ctx)
	if err != nil {
		c.log.Error("health: list running deployments", "err", err)
		return
	}
	for _, dep := range deployments {
		c.checkDeployment(ctx, dep)
	}
}

func (c *Checker) ContainerState(ctx context.Context, appID uuid.UUID) (string, error) {
	dep, err := c.deps.GetActive(ctx, appID)
	if err != nil {
		if !errors.Is(err, domain.ErrDeploymentNotFound) {
			return "", err
		}
		deployments, listErr := c.deps.ListByApp(ctx, appID, 50)
		if listErr != nil {
			return "", fmt.Errorf("health.ContainerState: list deployments: %w", listErr)
		}
		for _, candidate := range deployments {
			if candidate.Status == domain.DeploymentStatusStopped {
				return "stopped", nil
			}
			if candidate.ContainerID != "" {
				dep = candidate
				break
			}
		}
		if dep.ContainerID == "" {
			return "", domain.ErrDeploymentNotFound
		}
	}
	if dep.ContainerID == "" {
		return "missing", nil
	}
	state, err := c.docker.InspectContainer(ctx, dep.ContainerID)
	if err != nil {
		return "", fmt.Errorf("health.ContainerState: %w", err)
	}
	return state.Status, nil
}

func (c *Checker) checkDeployment(ctx context.Context, dep domain.Deployment) {
	if dep.ContainerID == "" {
		c.markFailed(ctx, dep)
		return
	}
	state, err := c.docker.InspectContainer(ctx, dep.ContainerID)
	if err != nil {
		c.log.Warn("health: inspect container", "deployment", dep.ID, "container", dep.ContainerID, "err", err)
		return
	}
	if state.Status != "exited" && state.Status != "dead" && state.Status != "missing" {
		return
	}
	c.event(ctx, dep.ID, "health_check", "stderr", "Container terminou com estado: "+state.Status)
	c.markFailed(ctx, dep)
}

func (c *Checker) markFailed(ctx context.Context, dep domain.Deployment) {
	if err := c.deps.UpdateStatus(ctx, dep.ID, domain.DeploymentStatusFailed); err != nil {
		c.log.Error("health: mark deployment failed", "deployment", dep.ID, "err", err)
		return
	}
	if err := c.apps.UpdateStatus(ctx, dep.AppID, domain.AppStatusFailed); err != nil {
		c.log.Error("health: mark app failed", "app", dep.AppID, "err", err)
	}
	c.log.Warn("health: container unhealthy", "deployment", dep.ID, "container", dep.ContainerID)
}

func (c *Checker) event(ctx context.Context, deploymentID uuid.UUID, stage, stream, message string) {
	if c.logs == nil { return }
	if _, err := c.logs.Append(ctx, deploymentID, stage, stream, message); err != nil {
		c.log.Warn("persist deployment log", "deployment", deploymentID, "err", err)
	}
}
