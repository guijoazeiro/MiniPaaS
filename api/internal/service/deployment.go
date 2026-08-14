package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
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

type DockerfileBuilder interface {
	BuildImageWithDockerfile(ctx context.Context, tar io.Reader, tag, dockerfile string) (io.ReadCloser, error)
}

type ContainerReadiness interface {
	WaitContainerReady(ctx context.Context, id string, port int, opts docker.ReadinessOptions) error
}

type AtomicCaddyRouter interface {
	SwitchRoute(ctx context.Context, appName string, port int) (publicURL string, err error)
}

type CaddyRouter interface {
	UpsertRoute(ctx context.Context, appName string, port int) (publicURL string, err error)
	RemoveRoute(ctx context.Context, appName string) error
}

type EnvDecryptor interface {
	Decrypted(ctx context.Context, appID uuid.UUID) (map[string]string, error)
}

type DeploymentLogWriter interface {
	Append(ctx context.Context, deploymentID uuid.UUID, stage, stream, message string) (domain.DeploymentLog, error)
}

// DeploymentRepository contains the capabilities required by every public
// deployment operation. Optional extensions such as candidate metadata remain
// type-asserted because older/custom stores can safely omit them.
type DeploymentRepository interface {
	store.DeploymentStore
	store.DeploymentCancellationStore
	store.GitDeploymentStore
	store.TriggeredGitDeploymentStore
	store.GitRetryDeploymentStore
}

type DeploymentServiceOptions struct {
	Logs                DeploymentLogWriter
	ReadyTimeout        time.Duration
	BuildTimeout        time.Duration
	MaxConcurrentBuilds int
	CustomDomains       CustomDomainRouteSync
	RuntimeLimits       *docker.ResourceLimits
}

type DeploymentService struct {
	deps              DeploymentRepository
	apps              store.AppStore
	rollbacks         store.RollbackStore
	docker            DockerRunner
	caddy             CaddyRouter
	env               EnvDecryptor
	imageRetention    int
	restartPolicy     string
	restartMaxRetries int
	log               *slog.Logger
	logs              DeploymentLogWriter
	canceller         store.DeploymentCancellationStore
	gitDeployments    store.GitDeploymentStore
	triggeredGit      store.TriggeredGitDeploymentStore
	readyTimeout      time.Duration
	buildTimeout      time.Duration
	buildSlots        chan struct{}
	customDomains     CustomDomainRouteSync
	runtimeLimits     docker.ResourceLimits
	executionsMu      sync.Mutex
	executions        map[uuid.UUID]*deploymentExecution
	rolloutsMu        sync.Mutex
	rollouts          map[uuid.UUID]chan struct{}
}

type deploymentExecution struct {
	cancel context.CancelFunc
}

func NewDeploymentService(deps DeploymentRepository, apps store.AppStore, rb store.RollbackStore, dk DockerRunner, cd CaddyRouter, env EnvDecryptor, retention int, restartPolicy string, restartMaxRetries int, log *slog.Logger, options ...DeploymentServiceOptions) *DeploymentService {
	if retention < 1 {
		retention = 5
	}
	var logWriter DeploymentLogWriter
	readyTimeout := 60 * time.Second
	buildTimeout := 15 * time.Minute
	maxConcurrentBuilds := 2
	var customRoutes CustomDomainRouteSync
	var runtimeLimits docker.ResourceLimits
	for _, option := range options {
		if option.Logs != nil {
			logWriter = option.Logs
		}
		if option.ReadyTimeout > 0 {
			readyTimeout = option.ReadyTimeout
		}
		if option.BuildTimeout > 0 {
			buildTimeout = option.BuildTimeout
		}
		if option.MaxConcurrentBuilds > 0 {
			maxConcurrentBuilds = option.MaxConcurrentBuilds
		}
		if option.CustomDomains != nil {
			customRoutes = option.CustomDomains
		}
		if option.RuntimeLimits != nil {
			runtimeLimits = *option.RuntimeLimits
		}
	}
	return &DeploymentService{
		deps: deps, apps: apps, rollbacks: rb,
		docker: dk, caddy: cd, env: env,
		imageRetention: retention, restartPolicy: restartPolicy, restartMaxRetries: restartMaxRetries, log: log, logs: logWriter,
		canceller: deps, gitDeployments: deps, triggeredGit: deps, readyTimeout: readyTimeout,
		customDomains: customRoutes, runtimeLimits: runtimeLimits,
		buildTimeout: buildTimeout, buildSlots: make(chan struct{}, maxConcurrentBuilds),
		executions: make(map[uuid.UUID]*deploymentExecution),
		rollouts:   make(map[uuid.UUID]chan struct{}),
	}
}

func (s *DeploymentService) beginExecution(ctx context.Context, deploymentID uuid.UUID) (context.Context, func()) {
	runCtx, cancel := context.WithCancel(ctx)
	execution := &deploymentExecution{cancel: cancel}
	s.executionsMu.Lock()
	previous := s.executions[deploymentID]
	s.executions[deploymentID] = execution
	s.executionsMu.Unlock()
	if previous != nil {
		previous.cancel()
	}
	if current, err := s.deps.GetByID(context.WithoutCancel(ctx), deploymentID); err == nil && current.CancelRequested {
		cancel()
	}
	done := func() {
		cancel()
		s.executionsMu.Lock()
		if s.executions[deploymentID] == execution {
			delete(s.executions, deploymentID)
		}
		s.executionsMu.Unlock()
	}
	return runCtx, done
}

func (s *DeploymentService) acquireBuild(ctx context.Context) (func(), error) {
	if s.buildSlots == nil {
		return func() {}, nil
	}
	select {
	case s.buildSlots <- struct{}{}:
		return func() { <-s.buildSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *DeploymentService) Cancel(ctx context.Context, deploymentID uuid.UUID) (domain.Deployment, error) {
	current, err := s.deps.GetByID(ctx, deploymentID)
	if err != nil {
		return domain.Deployment{}, err
	}
	if current.Status != domain.DeploymentStatusPending && current.Status != domain.DeploymentStatusBuilding && current.Status != domain.DeploymentStatusCancelRequested {
		return domain.Deployment{}, domain.ErrDeploymentNotCancellable
	}
	if s.canceller == nil {
		return domain.Deployment{}, fmt.Errorf("service.Cancel: deployment cancellation store unavailable")
	}
	if _, err := s.canceller.RequestCancel(ctx, deploymentID); err != nil {
		return domain.Deployment{}, err
	}
	s.event(ctx, deploymentID, "cleanup", "system", "Cancelamento solicitado")
	s.executionsMu.Lock()
	execution := s.executions[deploymentID]
	s.executionsMu.Unlock()
	if execution != nil {
		execution.cancel()
	} else {
		if err := s.canceller.MarkCancelled(ctx, deploymentID); err != nil {
			return domain.Deployment{}, err
		}
	}
	updated, err := s.deps.GetByID(context.WithoutCancel(ctx), deploymentID)
	if err != nil {
		return domain.Deployment{}, err
	}
	return updated, nil
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

func (s *DeploymentService) CreateGit(ctx context.Context, appName, repository, branch string) (domain.Deployment, domain.App, error) {
	return s.CreateGitTriggered(ctx, appName, repository, branch, domain.DeploymentTriggerManual, "")
}

func (s *DeploymentService) CreateGitTriggered(ctx context.Context, appName, repository, branch, triggerType, deliveryID string) (domain.Deployment, domain.App, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.Deployment{}, domain.App{}, err
	}
	tag := fmt.Sprintf("%s:ts-%d", app.Name, time.Now().UnixNano())
	var dep domain.Deployment
	if triggerType == domain.DeploymentTriggerWebhook {
		if s.triggeredGit == nil {
			return domain.Deployment{}, domain.App{}, fmt.Errorf("service.CreateGit: triggered git deployment store unavailable")
		}
		dep, err = s.triggeredGit.CreateGitTriggered(ctx, app.ID, tag, repository, branch, triggerType, deliveryID)
	} else {
		if s.gitDeployments == nil {
			return domain.Deployment{}, domain.App{}, fmt.Errorf("service.CreateGit: git deployment store unavailable")
		}
		dep, err = s.gitDeployments.CreateGit(ctx, app.ID, tag, repository, branch)
	}
	if err != nil {
		return domain.Deployment{}, domain.App{}, fmt.Errorf("service.CreateGit: %w", err)
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

func (s *DeploymentService) ListAll(ctx context.Context, appName, status string, page, perPage int) (domain.DeploymentPage, error) {
	items, err := s.deps.ListAll(ctx, appName, status, perPage, (page-1)*perPage)
	if err != nil {
		return domain.DeploymentPage{}, fmt.Errorf("service.ListDeployments: %w", err)
	}
	total, err := s.deps.CountAll(ctx, appName, status)
	if err != nil {
		return domain.DeploymentPage{}, fmt.Errorf("service.CountDeployments: %w", err)
	}
	if items == nil {
		items = []domain.DeploymentListItem{}
	}
	return domain.DeploymentPage{Items: items, Page: page, PerPage: perPage, Total: total}, nil
}

func (s *DeploymentService) RunBuild(ctx context.Context, dep domain.Deployment, app domain.App, src io.Reader) error {
	return s.RunBuildWithDockerfile(ctx, dep, app, src, "Dockerfile")
}

func (s *DeploymentService) RunBuildWithDockerfile(ctx context.Context, dep domain.Deployment, app domain.App, src io.Reader, dockerfile string) error {
	runCtx, done := s.beginExecution(ctx, dep.ID)
	defer done()
	return s.runBuildWithDockerfile(runCtx, dep, app, src, dockerfile)
}

func (s *DeploymentService) runBuildWithDockerfile(ctx context.Context, dep domain.Deployment, app domain.App, src io.Reader, dockerfile string) error {
	start := time.Now()
	if err := s.checkCancelled(ctx, dep, app.ID); err != nil {
		return err
	}
	s.event(ctx, dep.ID, "queued", "system", "Deployment iniciado")
	releaseBuildSlot, err := s.acquireBuild(ctx)
	if err != nil {
		return s.finishCancelled(ctx, dep, app.ID, "aguardando limite de builds")
	}
	releasedBuild := false
	releaseBuild := func() {
		if releasedBuild {
			return
		}
		releasedBuild = true
		releaseBuildSlot()
	}
	defer releaseBuild()

	if err := s.deps.UpdateStatus(ctx, dep.ID, domain.DeploymentStatusBuilding); err != nil {
		return fmt.Errorf("service.RunBuild: mark building: %w", err)
	}
	if err := s.checkCancelled(ctx, dep, app.ID); err != nil {
		return err
	}
	s.event(ctx, dep.ID, "building", "system", "Construindo imagem Docker")

	buildCtx := ctx
	cancelBuild := func() {}
	if s.buildTimeout > 0 {
		buildCtx, cancelBuild = context.WithTimeout(ctx, s.buildTimeout)
	}
	var buildLog io.ReadCloser
	if dockerfile == "" || dockerfile == "Dockerfile" {
		buildLog, err = s.docker.BuildImage(buildCtx, src, dep.ImageTag)
	} else if builder, ok := s.docker.(DockerfileBuilder); ok {
		buildLog, err = builder.BuildImageWithDockerfile(buildCtx, src, dep.ImageTag, dockerfile)
	} else {
		err = fmt.Errorf("custom Dockerfile builds are unavailable")
	}
	if err != nil {
		cancelBuild()
		if errors.Is(buildCtx.Err(), context.DeadlineExceeded) {
			s.event(context.WithoutCancel(ctx), dep.ID, "error", "stderr", "tempo limite da construção Docker excedido")
			s.markFailed(context.WithoutCancel(ctx), dep.ID, app.ID)
			return fmt.Errorf("service.RunBuild: build timeout: %w", buildCtx.Err())
		}
		if ctx.Err() != nil {
			return s.finishCancelled(ctx, dep, app.ID, "build interrompido")
		}
		s.event(ctx, dep.ID, "error", "stderr", err.Error())
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: build image: %w", err)
	}
	if err := s.persistBuildOutput(buildCtx, dep.ID, buildLog); err != nil {
		_ = buildLog.Close()
		cancelBuild()
		if errors.Is(buildCtx.Err(), context.DeadlineExceeded) {
			s.event(context.WithoutCancel(ctx), dep.ID, "error", "stderr", "tempo limite da construção Docker excedido")
			s.markFailed(context.WithoutCancel(ctx), dep.ID, app.ID)
			return fmt.Errorf("service.RunBuild: build timeout: %w", buildCtx.Err())
		}
		if ctx.Err() != nil {
			return s.finishCancelled(ctx, dep, app.ID, "build interrompido")
		}
		s.event(ctx, dep.ID, "error", "stderr", err.Error())
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: read build log: %w", err)
	}
	_ = buildLog.Close()
	cancelBuild()
	releaseBuild()
	if err := s.checkCancelled(ctx, dep, app.ID); err != nil {
		return err
	}
	s.event(ctx, dep.ID, "building", "system", "Imagem Docker construída")

	envVars, err := s.env.Decrypted(ctx, app.ID)
	if err != nil {
		if ctx.Err() != nil {
			return s.finishCancelled(ctx, dep, app.ID, "preparação interrompida")
		}
		s.event(ctx, dep.ID, "error", "stderr", err.Error())
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: decrypt env: %w", err)
	}
	if err := s.checkCancelled(ctx, dep, app.ID); err != nil {
		return err
	}

	releaseRollout, err := s.acquireRollout(ctx, app.ID)
	if err != nil {
		return s.finishCancelled(ctx, dep, app.ID, "aguardando outro rollout")
	}
	defer releaseRollout()

	var previous domain.Deployment
	if active, activeErr := s.deps.GetActive(ctx, app.ID); activeErr == nil {
		previous = active
	} else if !errors.Is(activeErr, domain.ErrDeploymentNotFound) {
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: get active deployment: %w", activeErr)
	}
	if err := s.checkCancelled(ctx, dep, app.ID); err != nil {
		return err
	}

	candidateName := fmt.Sprintf("minipaas-%s-%s", app.Name, dep.ID.String()[:8])
	s.event(ctx, dep.ID, "starting", "system", "Iniciando container candidato")
	info, err := s.docker.RunContainer(ctx, docker.RunOptions{
		Image:             dep.ImageTag,
		Name:              candidateName,
		Env:               envVars,
		RestartPolicy:     s.restartPolicy,
		RestartMaxRetries: s.restartMaxRetries,
		MemoryBytes:       s.runtimeLimits.MemoryBytes,
		NanoCPUs:          s.runtimeLimits.NanoCPUs,
		PidsLimit:         s.runtimeLimits.PidsLimit,
		Labels: map[string]string{
			"com.minipaas.managed":    "true",
			"com.minipaas.app":        app.Name,
			"com.minipaas.deployment": dep.ID.String(),
		},
	})
	if err != nil {
		if ctx.Err() != nil {
			return s.finishCancelled(ctx, dep, app.ID, "inicialização interrompida")
		}
		s.event(ctx, dep.ID, "error", "stderr", err.Error())
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: run container: %w", err)
	}
	candidateStore, hasCandidateStore := s.deps.(store.DeploymentCandidateStore)
	if hasCandidateStore {
		if err := candidateStore.UpdateCandidate(ctx, dep.ID, info.ID, info.Port); err != nil {
			_ = s.cleanupCandidate(context.WithoutCancel(ctx), info.ID)
			s.markFailed(ctx, dep.ID, app.ID)
			return fmt.Errorf("service.RunBuild: persist candidate: %w", err)
		}
	}
	cleanupCandidate := func(cleanupCtx context.Context) {
		if err := s.cleanupCandidate(cleanupCtx, info.ID); err != nil {
			s.log.Warn("cleanup candidate", "app", app.Name, "deployment", dep.ID, "container", info.ID, "err", err)
		}
		if hasCandidateStore {
			if err := candidateStore.ClearCandidate(context.WithoutCancel(cleanupCtx), dep.ID); err != nil {
				s.log.Warn("clear candidate metadata", "deployment", dep.ID, "err", err)
			}
		}
	}
	if readiness, ok := s.docker.(ContainerReadiness); ok {
		s.event(ctx, dep.ID, "health_check", "system", "Aguardando o container candidato ficar pronto")
		if err := readiness.WaitContainerReady(ctx, info.ID, info.Port, docker.ReadinessOptions{Timeout: s.readyTimeout}); err != nil {
			if ctx.Err() != nil {
				cleanupCandidate(context.WithoutCancel(ctx))
				return s.finishCancelled(ctx, dep, app.ID, "readiness interrompida")
			}
			cleanupCandidate(context.WithoutCancel(ctx))
			s.event(ctx, dep.ID, "error", "stderr", "container candidato não ficou pronto: "+err.Error())
			s.markFailed(ctx, dep.ID, app.ID)
			return fmt.Errorf("service.RunBuild: candidate readiness: %w", err)
		}
	} else {
		s.event(ctx, dep.ID, "health_check", "system", "Container candidato iniciado; readiness não configurada")
	}
	if ctx.Err() != nil {
		cleanupCandidate(context.WithoutCancel(ctx))
		return s.finishCancelled(ctx, dep, app.ID, "inicialização interrompida")
	}

	durationMs := int(time.Since(start).Milliseconds())
	s.event(ctx, dep.ID, "publishing", "system", "Trocando a rota para o container candidato")
	publicURL, err := s.switchRoute(ctx, app.Name, info.Port)
	if err != nil {
		if ctx.Err() != nil {
			cleanupCandidate(context.WithoutCancel(ctx))
			return s.finishCancelled(ctx, dep, app.ID, "troca de rota interrompida")
		}
		cleanupCandidate(context.WithoutCancel(ctx))
		s.event(ctx, dep.ID, "error", "stderr", "troca de rota falhou: "+err.Error())
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: switch route: %w", err)
	}
	if s.customDomains != nil {
		if err := s.customDomains.SyncRoutes(ctx, app.ID, info.Port); err != nil {
			if previous.ContainerID != "" {
				_, _ = s.switchRoute(context.WithoutCancel(ctx), app.Name, previous.Port)
				_ = s.customDomains.SyncRoutes(context.WithoutCancel(ctx), app.ID, previous.Port)
			} else {
				_ = s.caddy.RemoveRoute(context.WithoutCancel(ctx), app.Name)
			}
			cleanupCandidate(context.WithoutCancel(ctx))
			s.event(ctx, dep.ID, "error", "stderr", "rotas de domínios customizados falharam: "+err.Error())
			s.markFailed(ctx, dep.ID, app.ID)
			return fmt.Errorf("service.RunBuild: sync custom domains: %w", err)
		}
	}
	if err := s.checkCancelled(ctx, dep, app.ID); err != nil {
		if previous.ContainerID != "" {
			_, _ = s.switchRoute(context.WithoutCancel(ctx), app.Name, previous.Port)
			if s.customDomains != nil {
				_ = s.customDomains.SyncRoutes(context.WithoutCancel(ctx), app.ID, previous.Port)
			}
		} else {
			_ = s.caddy.RemoveRoute(context.WithoutCancel(ctx), app.Name)
		}
		cleanupCandidate(context.WithoutCancel(ctx))
		return err
	}
	if hasCandidateStore {
		err = candidateStore.PromoteCandidate(ctx, dep.ID, info.ID, info.Port, dep.ImageTag, durationMs)
	} else {
		err = s.deps.UpdateRunning(ctx, dep.ID, info.ID, info.Port, dep.ImageTag, durationMs)
	}
	if err != nil {
		if ctx.Err() != nil {
			if previous.ContainerID != "" {
				_, _ = s.switchRoute(context.WithoutCancel(ctx), app.Name, previous.Port)
				if s.customDomains != nil {
					_ = s.customDomains.SyncRoutes(context.WithoutCancel(ctx), app.ID, previous.Port)
				}
			} else {
				_ = s.caddy.RemoveRoute(context.WithoutCancel(ctx), app.Name)
			}
			cleanupCandidate(context.WithoutCancel(ctx))
			return s.finishCancelled(ctx, dep, app.ID, "atualização interrompida")
		}
		if previous.ContainerID != "" {
			_, _ = s.switchRoute(context.WithoutCancel(ctx), app.Name, previous.Port)
			if s.customDomains != nil {
				_ = s.customDomains.SyncRoutes(context.WithoutCancel(ctx), app.ID, previous.Port)
			}
		} else {
			_ = s.caddy.RemoveRoute(context.WithoutCancel(ctx), app.Name)
		}
		cleanupCandidate(context.WithoutCancel(ctx))
		s.event(ctx, dep.ID, "error", "stderr", err.Error())
		s.markFailed(ctx, dep.ID, app.ID)
		return fmt.Errorf("service.RunBuild: mark running: %w", err)
	}
	if err := s.apps.UpdateStatus(ctx, app.ID, domain.AppStatusRunning); err != nil {
		s.log.Warn("update app status", "app", app.Name, "err", err)
	}

	if err := s.apps.UpdatePublicURL(ctx, app.ID, publicURL); err != nil {
		s.log.Warn("update public url", "app", app.Name, "err", err)
	}
	if previous.ContainerID != "" && previous.ID != dep.ID {
		s.event(ctx, dep.ID, "cleanup", "system", "Parando o container anterior após a troca")
		if err := s.docker.StopContainer(ctx, previous.ContainerID); err != nil {
			s.log.Warn("stop previous container", "app", app.Name, "container", previous.ContainerID, "err", err)
		}
		if err := s.docker.RemoveContainer(ctx, previous.ContainerID); err != nil {
			s.log.Warn("remove previous container", "app", app.Name, "container", previous.ContainerID, "err", err)
		}
		_ = s.deps.UpdateStatus(ctx, previous.ID, domain.DeploymentStatusSuperseded)
	}

	s.log.Info("deploy ok",
		"app", app.Name,
		"deployment", dep.ID,
		"container", info.ID,
		"port", info.Port,
		"dur_ms", time.Since(start).Milliseconds(),
	)

	s.pruneImages(ctx, app.ID)
	s.event(ctx, dep.ID, "cleanup", "system", "Deployment concluído")
	return nil
}

func (s *DeploymentService) switchRoute(ctx context.Context, appName string, port int) (string, error) {
	if atomic, ok := s.caddy.(AtomicCaddyRouter); ok {
		return atomic.SwitchRoute(ctx, appName, port)
	}
	return s.caddy.UpsertRoute(ctx, appName, port)
}

func (s *DeploymentService) cleanupCandidate(ctx context.Context, containerID string) error {
	if containerID == "" {
		return nil
	}
	stopErr := s.docker.StopContainer(ctx, containerID)
	removeErr := s.docker.RemoveContainer(ctx, containerID)
	return errors.Join(stopErr, removeErr)
}

// acquireRollout serializes the route transition for each application. Image
// builds may run concurrently, but only one candidate can be promoted at a
// time; otherwise two deployments could both observe the same active release
// and leave an already-promoted container orphaned.
func (s *DeploymentService) acquireRollout(ctx context.Context, appID uuid.UUID) (func(), error) {
	s.rolloutsMu.Lock()
	lock := s.rollouts[appID]
	if lock == nil {
		lock = make(chan struct{}, 1)
		s.rollouts[appID] = lock
	}
	s.rolloutsMu.Unlock()

	select {
	case lock <- struct{}{}:
		return func() { <-lock }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// RecoverCandidates cleans candidates left behind by an API restart. The last
// committed running deployment remains the source of truth, so its route is
// restored before the orphan candidate is removed.
func (s *DeploymentService) RecoverCandidates(ctx context.Context) error {
	candidates, ok := s.deps.(store.DeploymentCandidateStore)
	if !ok {
		return nil
	}
	items, err := candidates.ListCandidates(ctx)
	if err != nil {
		return err
	}
	for _, candidate := range items {
		previous, previousErr := s.deps.GetActive(ctx, candidate.AppID)
		app, appErr := s.apps.GetByID(ctx, candidate.AppID)
		if previousErr == nil && appErr == nil && previous.ContainerID != "" {
			if _, routeErr := s.switchRoute(ctx, app.Name, previous.Port); routeErr != nil {
				s.log.Warn("restore route for orphan candidate", "app", app.Name, "deployment", candidate.ID, "err", routeErr)
			}
			if s.customDomains != nil {
				if syncErr := s.customDomains.SyncRoutes(ctx, candidate.AppID, previous.Port); syncErr != nil {
					s.log.Warn("restore custom routes for orphan candidate", "app", app.Name, "deployment", candidate.ID, "err", syncErr)
				}
			}
		} else if appErr == nil {
			if routeErr := s.caddy.RemoveRoute(ctx, app.Name); routeErr != nil {
				s.log.Warn("remove route for orphan candidate", "app", app.Name, "deployment", candidate.ID, "err", routeErr)
			}
		}
		if cleanupErr := s.cleanupCandidate(ctx, candidate.CandidateContainerID); cleanupErr != nil {
			s.log.Warn("cleanup orphan candidate", "deployment", candidate.ID, "container", candidate.CandidateContainerID, "err", cleanupErr)
		}
		_ = candidates.ClearCandidate(ctx, candidate.ID)
		_ = s.deps.UpdateStatus(ctx, candidate.ID, domain.DeploymentStatusFailed)
		s.event(ctx, candidate.ID, "cleanup", "system", "Candidato órfão removido após reinício da API")
	}
	return nil
}

func (s *DeploymentService) checkCancelled(ctx context.Context, dep domain.Deployment, appID uuid.UUID) error {
	if ctx.Err() != nil {
		return s.finishCancelled(ctx, dep, appID, "cancelado")
	}
	current, err := s.deps.GetByID(context.WithoutCancel(ctx), dep.ID)
	if err == nil && current.CancelRequested {
		return s.finishCancelled(ctx, dep, appID, "cancelamento solicitado")
	}
	return nil
}

func (s *DeploymentService) finishCancelled(ctx context.Context, dep domain.Deployment, appID uuid.UUID, reason string) error {
	writeCtx := context.WithoutCancel(ctx)
	if s.canceller != nil {
		if err := s.canceller.MarkCancelled(writeCtx, dep.ID); err != nil {
			s.log.Warn("mark deployment cancelled", "deployment", dep.ID, "err", err)
		}
	} else if err := s.deps.UpdateStatus(writeCtx, dep.ID, domain.DeploymentStatusCancelled); err != nil {
		s.log.Warn("mark deployment cancelled", "deployment", dep.ID, "err", err)
	}
	s.event(writeCtx, dep.ID, "cleanup", "system", "Deployment cancelado: "+reason)
	return domain.ErrDeploymentCancelled
}

func (s *DeploymentService) event(ctx context.Context, deploymentID uuid.UUID, stage, stream, message string) {
	if s.logs == nil || strings.TrimSpace(message) == "" {
		return
	}
	if _, err := s.logs.Append(context.WithoutCancel(ctx), deploymentID, stage, stream, message); err != nil {
		s.log.Warn("persist deployment log", "deployment", deploymentID, "stage", stage, "err", err)
	}
}

func (s *DeploymentService) persistBuildOutput(ctx context.Context, deploymentID uuid.UUID, rc io.Reader) error {
	if rc == nil {
		return nil
	}
	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			s.event(ctx, deploymentID, "building", "stdout", line)
		}
	}
	return scanner.Err()
}

func (s *DeploymentService) UpdateGitMetadata(ctx context.Context, depID uuid.UUID, sha, author, message, branch string) error {
	if s.gitDeployments == nil {
		return fmt.Errorf("git deployment store unavailable")
	}
	return s.gitDeployments.UpdateGitMetadata(ctx, depID, sha, author, message, branch)
}

func (s *DeploymentService) MarkFailed(ctx context.Context, depID, appID uuid.UUID) {
	s.markFailed(ctx, depID, appID)
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
	if target.Status != domain.DeploymentStatusSuperseded && target.Status != domain.DeploymentStatusRolledBack && target.Status != domain.DeploymentStatusStopped {
		return domain.Deployment{}, domain.ErrDeploymentNotRollbackable
	}

	// Resolve every fallible prerequisite before touching the active container.
	envVars, err := s.env.Decrypted(ctx, app.ID)
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("service.Rollback: decrypt env: %w", err)
	}
	releaseRollout, err := s.acquireRollout(ctx, app.ID)
	if err != nil {
		return domain.Deployment{}, fmt.Errorf("service.Rollback: wait for rollout: %w", err)
	}
	defer releaseRollout()

	var previous domain.Deployment
	current, err := s.deps.GetActive(ctx, app.ID)
	if err == nil {
		if current.ID == target.ID {
			return domain.Deployment{}, domain.ErrDeploymentActive
		}
		previous = current
	} else if !errors.Is(err, domain.ErrDeploymentNotFound) {
		return domain.Deployment{}, fmt.Errorf("service.Rollback: get active deployment: %w", err)
	}

	// Keep the current release alive until the rollback candidate has passed
	// readiness and the route has been switched. This gives rollback the same
	// zero-downtime safety property as a normal deployment.
	candidateName := fmt.Sprintf("minipaas-%s-rollback-%s", app.Name, uuid.NewString()[:8])
	info, err := s.docker.RunContainer(ctx, docker.RunOptions{
		Image:             target.ImageTag,
		Name:              candidateName,
		Env:               envVars,
		RestartPolicy:     s.restartPolicy,
		RestartMaxRetries: s.restartMaxRetries,
		MemoryBytes:       s.runtimeLimits.MemoryBytes,
		NanoCPUs:          s.runtimeLimits.NanoCPUs,
		PidsLimit:         s.runtimeLimits.PidsLimit,
		Labels: map[string]string{
			"com.minipaas.managed":    "true",
			"com.minipaas.app":        app.Name,
			"com.minipaas.deployment": target.ID.String(),
		},
	})
	if err != nil {
		if previous.ContainerID == "" {
			_ = s.apps.UpdateStatus(context.WithoutCancel(ctx), app.ID, domain.AppStatusFailed)
		}
		s.log.Error("rollback: candidate failed to start", "app", app.Name, "target", target.ID, "err", err)
		return domain.Deployment{}, fmt.Errorf("service.Rollback: run candidate container: %w", err)
	}

	candidateStore, hasCandidateStore := s.deps.(store.DeploymentCandidateStore)
	if hasCandidateStore {
		if err := candidateStore.UpdateCandidate(ctx, target.ID, info.ID, info.Port); err != nil {
			_ = s.cleanupCandidate(context.WithoutCancel(ctx), info.ID)
			return domain.Deployment{}, fmt.Errorf("service.Rollback: persist candidate: %w", err)
		}
	}
	cleanupCandidate := func(cleanupCtx context.Context) {
		if err := s.cleanupCandidate(cleanupCtx, info.ID); err != nil {
			s.log.Warn("rollback: cleanup candidate", "app", app.Name, "target", target.ID, "container", info.ID, "err", err)
		}
		if hasCandidateStore {
			if err := candidateStore.ClearCandidate(context.WithoutCancel(cleanupCtx), target.ID); err != nil {
				s.log.Warn("rollback: clear candidate metadata", "target", target.ID, "err", err)
			}
		}
	}
	restorePrevious := func() {
		restoreCtx := context.WithoutCancel(ctx)
		if previous.ContainerID != "" {
			if _, restoreErr := s.switchRoute(restoreCtx, app.Name, previous.Port); restoreErr != nil {
				s.log.Error("rollback: restore previous route", "app", app.Name, "port", previous.Port, "err", restoreErr)
			}
			if s.customDomains != nil {
				if syncErr := s.customDomains.SyncRoutes(restoreCtx, app.ID, previous.Port); syncErr != nil {
					s.log.Warn("rollback: restore custom domain routes", "app", app.Name, "err", syncErr)
				}
			}
			return
		}
		if err := s.caddy.RemoveRoute(restoreCtx, app.Name); err != nil {
			s.log.Warn("rollback: remove candidate route", "app", app.Name, "err", err)
		}
		if s.customDomains != nil {
			if err := s.customDomains.RemoveRoutes(restoreCtx, app.ID); err != nil {
				s.log.Warn("rollback: remove candidate custom domain routes", "app", app.Name, "err", err)
			}
		}
	}
	if readiness, ok := s.docker.(ContainerReadiness); ok {
		if err := readiness.WaitContainerReady(ctx, info.ID, info.Port, docker.ReadinessOptions{Timeout: s.readyTimeout}); err != nil {
			cleanupCandidate(context.WithoutCancel(ctx))
			if previous.ContainerID == "" {
				_ = s.apps.UpdateStatus(context.WithoutCancel(ctx), app.ID, domain.AppStatusFailed)
			}
			return domain.Deployment{}, fmt.Errorf("service.Rollback: candidate readiness: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		cleanupCandidate(context.WithoutCancel(ctx))
		return domain.Deployment{}, fmt.Errorf("service.Rollback: candidate cancelled: %w", err)
	}

	publicURL, err := s.switchRoute(ctx, app.Name, info.Port)
	if err != nil {
		cleanupCandidate(context.WithoutCancel(ctx))
		if previous.ContainerID == "" {
			_ = s.apps.UpdateStatus(context.WithoutCancel(ctx), app.ID, domain.AppStatusFailed)
		}
		return domain.Deployment{}, fmt.Errorf("service.Rollback: switch route: %w", err)
	}
	if s.customDomains != nil {
		if err := s.customDomains.SyncRoutes(ctx, app.ID, info.Port); err != nil {
			restorePrevious()
			cleanupCandidate(context.WithoutCancel(ctx))
			return domain.Deployment{}, fmt.Errorf("service.Rollback: sync custom domains: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		restorePrevious()
		cleanupCandidate(context.WithoutCancel(ctx))
		return domain.Deployment{}, fmt.Errorf("service.Rollback: publish cancelled: %w", err)
	}

	durationMs := int(time.Since(start).Milliseconds())
	if hasCandidateStore {
		err = candidateStore.PromoteCandidate(ctx, target.ID, info.ID, info.Port, target.ImageTag, durationMs)
	} else {
		err = s.deps.UpdateRunning(ctx, target.ID, info.ID, info.Port, target.ImageTag, durationMs)
	}
	if err != nil {
		restorePrevious()
		cleanupCandidate(context.WithoutCancel(ctx))
		return domain.Deployment{}, fmt.Errorf("service.Rollback: promote candidate: %w", err)
	}

	if err := s.apps.UpdateStatus(ctx, app.ID, domain.AppStatusRunning); err != nil {
		s.log.Warn("rollback: update app status", "err", err)
	}
	if err := s.apps.UpdatePublicURL(ctx, app.ID, publicURL); err != nil {
		s.log.Warn("rollback: update public url", "err", err)
	}
	if previous.ContainerID != "" && previous.ID != target.ID {
		s.event(ctx, target.ID, "cleanup", "system", "Parando o container anterior após a troca")
		if err := s.docker.StopContainer(ctx, previous.ContainerID); err != nil {
			s.log.Warn("rollback: stop previous", "container", previous.ContainerID, "err", err)
		}
		if err := s.docker.RemoveContainer(ctx, previous.ContainerID); err != nil {
			s.log.Warn("rollback: remove previous", "container", previous.ContainerID, "err", err)
		}
		if err := s.deps.UpdateStatus(ctx, previous.ID, domain.DeploymentStatusRolledBack); err != nil {
			s.log.Warn("rollback: mark previous rolled_back", "err", err)
		}
	}
	if err := s.rollbacks.Record(ctx, app.ID, previous.ID, target.ID, triggeredBy); err != nil {
		s.log.Warn("rollback: history", "err", err)
	}

	restored, err := s.deps.GetByID(ctx, target.ID)
	if err != nil {
		return domain.Deployment{}, err
	}
	s.log.Info("rollback ok",
		"app", app.Name, "from", previous.ID, "to", target.ID,
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
	releaseRollout, err := s.acquireRollout(ctx, app.ID)
	if err != nil {
		return fmt.Errorf("service.StopApp: wait for rollout: %w", err)
	}
	defer releaseRollout()

	if err := s.caddy.RemoveRoute(ctx, app.Name); err != nil {
		return fmt.Errorf("service.StopApp: remove caddy route: %w", err)
	}
	if s.customDomains != nil {
		if err := s.customDomains.RemoveRoutes(ctx, app.ID); err != nil {
			return fmt.Errorf("service.StopApp: remove custom domain routes: %w", err)
		}
	}
	dep, err := s.deps.GetActive(ctx, app.ID)
	if err != nil {
		if !errors.Is(err, domain.ErrDeploymentNotFound) {
			return fmt.Errorf("service.StopApp: get active deployment: %w", err)
		}
		deployments, listErr := s.deps.ListByApp(ctx, app.ID, 50)
		if listErr != nil {
			return fmt.Errorf("service.StopApp: list deployments: %w", listErr)
		}
		if len(deployments) > 0 {
			candidate := deployments[0]
			if candidate.ContainerID != "" && candidate.Status == domain.DeploymentStatusFailed {
				dep = candidate
			}
		}
	}
	if dep.ContainerID != "" {
		if err := s.docker.StopContainer(ctx, dep.ContainerID); err != nil {
			s.log.Warn("stop container", "container", dep.ContainerID, "err", err)
		}
		if err := s.docker.RemoveContainer(ctx, dep.ContainerID); err != nil {
			return fmt.Errorf("service.StopApp: %w", err)
		}
		if err := s.deps.UpdateStatus(ctx, dep.ID, domain.DeploymentStatusStopped); err != nil {
			return fmt.Errorf("service.StopApp: mark deployment stopped: %w", err)
		}
	}
	if err := s.apps.UpdateStatus(ctx, app.ID, domain.AppStatusStopped); err != nil {
		return fmt.Errorf("service.StopApp: mark app stopped: %w", err)
	}
	if err := s.apps.UpdatePublicURL(ctx, app.ID, ""); err != nil {
		return fmt.Errorf("service.StopApp: clear public URL: %w", err)
	}
	return nil
}

func (s *DeploymentService) markFailed(ctx context.Context, depID, appID uuid.UUID) {
	if err := s.deps.UpdateStatus(ctx, depID, domain.DeploymentStatusFailed); err != nil {
		s.log.Error("mark deployment failed", "deployment", depID, "err", err)
	}
	status := domain.AppStatusFailed
	if _, err := s.deps.GetActive(ctx, appID); err == nil {
		status = domain.AppStatusRunning
	}
	if err := s.apps.UpdateStatus(ctx, appID, status); err != nil {
		s.log.Error("mark app failed", "app", appID, "err", err)
	}
}
