package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type DockerRunner interface {
	BuildImage(ctx context.Context, tar io.Reader, tag string) (io.ReadCloser, error)
	RunContainer(ctx context.Context, opts docker.RunOptions) (docker.ContainerInfo, error)
	StopContainer(ctx context.Context, id string) error
	RemoveContainer(ctx context.Context, id string) error
	RemoveImage(ctx context.Context, ref string) error
}

type CaddyRouter interface {
	UpsertRoute(ctx context.Context, appName string, port int) (publicURL string, err error)
	RemoveRoute(ctx context.Context, appName string) error
}

type EnvDecryptor interface {
	Decrypted(ctx context.Context, appID uuid.UUID) (map[string]string, error)
}

type DeploymentService struct {
	deps           store.DeploymentStore
	apps           store.AppStore
	rollbacks      store.RollbackStore
	docker         DockerRunner
	caddy          CaddyRouter
	env            EnvDecryptor
	imageRetention int
	log            *slog.Logger
}

func NewDeploymentService(deps store.DeploymentStore, apps store.AppStore, rb store.RollbackStore, dk DockerRunner, cd CaddyRouter, env EnvDecryptor, retention int, log *slog.Logger) *DeploymentService {
	if retention < 1 {
		retention = 5
	}
	return &DeploymentService{
		deps: deps, apps: apps, rollbacks: rb,
		docker: dk, caddy: cd, env: env,
		imageRetention: retention, log: log,
	}
}

func (s *DeploymentService) Create(ctx context.Context, appName string) (domain.Deployment, domain.App, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.Deployment{}, domain.App{}, err
	}
	tag := fmt.Sprintf("%s:ts-%d", app.Name, time.Now().Unix())
	dep, err := s.deps.Create(ctx, app.ID, tag)
	if err != nil {
		return domain.Deployment{}, domain.App{}, fmt.Errorf("service.Create: %w", err)
	}
	return dep, app, nil
}

func (s *DeploymentService) Get(ctx context.Context, id uuid.UUID) (domain.Deployment, error) {
	return s.deps.GetByID(ctx, id)
}

func (s *DeploymentService) ListByApp(ctx context.Context, appID uuid.UUID, limit int) ([]domain.Deployment, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.deps.ListByApp(ctx, appID, limit)
}

func (s *DeploymentService) RunBuild(ctx context.Context, dep domain.Deployment, app domain.App, src io.Reader) error {
	start := time.Now()

	if err := s.deps.UpdateStatus(ctx, dep.ID, domain.DeploymentStatusBuilding); err != nil {
		return fmt.Errorf("service.RunBuild: mark building: %w", err)
	}

	buildLog, err := s.docker.BuildImage(ctx, src, dep.ImageTag)
	if err != nil {
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: build image: %w", err)
	}
	if _, err := io.Copy(io.Discard, buildLog); err != nil {
		_ = buildLog.Close()
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: drain build log: %w", err)
	}
	_ = buildLog.Close()

	if prev, err := s.deps.GetActive(ctx, app.ID); err == nil && prev.ContainerID != "" {
		if err := s.docker.StopContainer(ctx, prev.ContainerID); err != nil {
			s.log.Warn("stop previous container", "app", app.Name, "container", prev.ContainerID, "err", err)
		}
		if err := s.docker.RemoveContainer(ctx, prev.ContainerID); err != nil {
			s.log.Warn("remove previous container", "app", app.Name, "container", prev.ContainerID, "err", err)
		}
		_ = s.deps.UpdateStatus(ctx, prev.ID, domain.DeploymentStatusSuperseded)
	}

	envVars, err := s.env.Decrypted(ctx, app.ID)
	if err != nil {
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: decrypt env: %w", err)
	}

	info, err := s.docker.RunContainer(ctx, docker.RunOptions{
		Image: dep.ImageTag,
		Name:  fmt.Sprintf("minipaas-%s", app.Name),
		Env:   envVars,
	})
	if err != nil {
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: run container: %w", err)
	}

	durationMs := int(time.Since(start).Milliseconds())
	if err := s.deps.UpdateRunning(ctx, dep.ID, info.ID, info.Port, dep.ImageTag, durationMs); err != nil {
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: mark running: %w", err)
	}
	if err := s.apps.UpdateStatus(ctx, app.ID, domain.AppStatusRunning); err != nil {
		s.log.Warn("update app status", "app", app.Name, "err", err)
	}

	publicURL, err := s.caddy.UpsertRoute(ctx, app.Name, info.Port)
	if err != nil {
		s.log.Error("caddy route", "app", app.Name, "err", err)
	} else if err := s.apps.UpdatePublicURL(ctx, app.ID, publicURL); err != nil {
		s.log.Warn("update public url", "app", app.Name, "err", err)
	}

	s.log.Info("deploy ok",
		"app", app.Name,
		"deployment", dep.ID,
		"container", info.ID,
		"port", info.Port,
		"dur_ms", time.Since(start).Milliseconds(),
	)

	s.pruneImages(ctx, app.ID)
	return nil
}

func (s *DeploymentService) Rollback(ctx context.Context, appName string, targetID uuid.UUID, triggeredBy string) (domain.Deployment, error) {
	start := time.Now()

	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.Deployment{}, err
	}
	target, err := s.deps.GetByID(ctx, targetID)
	if err != nil {
		return domain.Deployment{}, err
	}
	if target.AppID != app.ID {
		return domain.Deployment{}, domain.ErrDeploymentNotFound
	}
	if target.ImageTag == "" {
		return domain.Deployment{}, fmt.Errorf("target deployment has no image tag")
	}
	var fromID uuid.UUID
	current, err := s.deps.GetActive(ctx, app.ID)
	if err == nil {
		if current.ID == target.ID {
			return domain.Deployment{}, domain.ErrDeploymentActive
		}
		fromID = current.ID
		if current.ContainerID != "" {
			if err := s.docker.StopContainer(ctx, current.ContainerID); err != nil {
				s.log.Warn("rollback: stop current", "container", current.ContainerID, "err", err)
			}
			if err := s.docker.RemoveContainer(ctx, current.ContainerID); err != nil {
				s.log.Warn("rollback: remove current", "container", current.ContainerID, "err", err)
			}
		}
		if err := s.deps.UpdateStatus(ctx, current.ID, domain.DeploymentStatusRolledBack); err != nil {
			s.log.Warn("rollback: mark current rolled_back", "err", err)
		}
	} else if !errors.Is(err, domain.ErrDeploymentNotFound) {
		return domain.Deployment{}, fmt.Errorf("service.Rollback: get active deployment: %w", err)
	}
	if target.Status != domain.DeploymentStatusSuperseded && target.Status != domain.DeploymentStatusRolledBack {
		return domain.Deployment{}, domain.ErrDeploymentNotRollbackable
	}

	envVars, err := s.env.Decrypted(ctx, app.ID)
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("service.Rollback: decrypt env: %w", err)
	}

	info, err := s.docker.RunContainer(ctx, docker.RunOptions{
		Image: target.ImageTag,
		Name:  fmt.Sprintf("minipaas-%s", app.Name),
		Env:   envVars,
	})
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("service.Rollback: run container: %w", err)
	}

	durationMs := int(time.Since(start).Milliseconds())
	if err := s.deps.UpdateRunning(ctx, target.ID, info.ID, info.Port, target.ImageTag, durationMs); err != nil {
		return domain.Deployment{}, fmt.Errorf("service.Rollback: mark target running: %w", err)
	}
	if err := s.apps.UpdateStatus(ctx, app.ID, domain.AppStatusRunning); err != nil {
		s.log.Warn("rollback: update app status", "err", err)
	}

	publicURL, err := s.caddy.UpsertRoute(ctx, app.Name, info.Port)
	if err != nil {
		s.log.Error("rollback: caddy route", "err", err)
	} else if err := s.apps.UpdatePublicURL(ctx, app.ID, publicURL); err != nil {
		s.log.Warn("rollback: update public url", "err", err)
	}

	if err := s.rollbacks.Record(ctx, app.ID, fromID, target.ID, triggeredBy); err != nil {
		s.log.Warn("rollback: history", "err", err)
	}

	restored, err := s.deps.GetByID(ctx, target.ID)
	if err != nil {
		return domain.Deployment{}, err
	}
	s.log.Info("rollback ok",
		"app", app.Name, "from", fromID, "to", target.ID,
		"container", info.ID, "port", info.Port,
		"dur_ms", time.Since(start).Milliseconds(),
	)
	return restored, nil
}

func (s *DeploymentService) pruneImages(ctx context.Context, appID uuid.UUID) {
	old, err := s.deps.ListForRetention(ctx, appID, s.imageRetention)
	if err != nil {
		s.log.Warn("prune: list", "err", err)
		return
	}
	seen := map[string]bool{}
	for _, d := range old {
		if d.ImageTag == "" || seen[d.ImageTag] {
			continue
		}
		seen[d.ImageTag] = true
		if err := s.docker.RemoveImage(ctx, d.ImageTag); err != nil {
			s.log.Debug("prune: skip", "image", d.ImageTag, "err", err)
		}
	}
}

func (s *DeploymentService) StopApp(ctx context.Context, app domain.App) error {
	if err := s.caddy.RemoveRoute(ctx, app.Name); err != nil {
		s.log.Warn("caddy remove route", "app", app.Name, "err", err)
	}
	dep, err := s.deps.GetActive(ctx, app.ID)
	if err != nil {
		return nil
	}
	if dep.ContainerID == "" {
		return nil
	}
	if err := s.docker.StopContainer(ctx, dep.ContainerID); err != nil {
		s.log.Warn("stop container", "container", dep.ContainerID, "err", err)
	}
	if err := s.docker.RemoveContainer(ctx, dep.ContainerID); err != nil {
		return fmt.Errorf("service.StopApp: %w", err)
	}
	return s.deps.UpdateStatus(ctx, dep.ID, domain.DeploymentStatusSuperseded)
}

func (s *DeploymentService) markFailed(ctx context.Context, depID, appID uuid.UUID) {
	if err := s.deps.UpdateStatus(ctx, depID, domain.DeploymentStatusFailed); err != nil {
		s.log.Error("mark deployment failed", "deployment", depID, "err", err)
	}
	if err := s.apps.UpdateStatus(ctx, appID, domain.AppStatusFailed); err != nil {
		s.log.Error("mark app failed", "app", appID, "err", err)
	}
}
