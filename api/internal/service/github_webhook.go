package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type GitHubWebhookDeploymentService interface {
	CreateTriggered(ctx context.Context, appName string, source domain.GitSource, branch, deliveryID string) (domain.Deployment, domain.App, error)
	Run(ctx context.Context, dep domain.Deployment, app domain.App, source domain.GitSource, branch string) error
}

type GitHubWebhookService struct {
	sources     store.GitSourceStore
	apps        store.AppStore
	deliveries  store.GitHubWebhookDeliveryStore
	deployments GitHubWebhookDeploymentService
	log         *slog.Logger
}

type GitHubWebhookResult struct {
	Duplicate   bool `json:"duplicate"`
	Ignored     bool `json:"ignored"`
	Deployments int  `json:"deployments"`
}

type githubPushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Deleted    bool   `json:"deleted"`
	Repository struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}

func NewGitHubWebhookService(sources store.GitSourceStore, apps store.AppStore, deliveries store.GitHubWebhookDeliveryStore, deployments GitHubWebhookDeploymentService, log *slog.Logger) *GitHubWebhookService {
	return &GitHubWebhookService{sources: sources, apps: apps, deliveries: deliveries, deployments: deployments, log: log}
}

func (s *GitHubWebhookService) Handle(ctx context.Context, event, deliveryID string, payload []byte) (GitHubWebhookResult, error) {
	if event != "push" {
		return GitHubWebhookResult{Ignored: true}, nil
	}
	var push githubPushPayload
	if err := json.Unmarshal(payload, &push); err != nil {
		return GitHubWebhookResult{}, fmt.Errorf("decode GitHub push webhook: %w", err)
	}
	if deliveryID == "" || push.Repository.ID <= 0 || push.Repository.FullName == "" || push.Ref == "" {
		return GitHubWebhookResult{}, fmt.Errorf("invalid GitHub push webhook payload")
	}
	claimed, err := s.deliveries.Claim(ctx, deliveryID, event, push.Repository.ID, push.After)
	if err != nil {
		return GitHubWebhookResult{}, err
	}
	if !claimed {
		return GitHubWebhookResult{Duplicate: true}, nil
	}
	complete := func(status string, cause error) {
		message := ""
		if cause != nil {
			message = cause.Error()
		}
		if err := s.deliveries.Complete(context.WithoutCancel(ctx), deliveryID, status, message); err != nil {
			s.log.Error("complete GitHub webhook delivery", "delivery", deliveryID, "err", err)
		}
	}

	branch := strings.TrimPrefix(push.Ref, "refs/heads/")
	if push.Deleted || branch == push.Ref || isZeroSHA(push.After) {
		complete("ignored", nil)
		return GitHubWebhookResult{Ignored: true}, nil
	}
	sources, err := s.sources.ListAutoDeployByRepository(ctx, push.Repository.ID)
	if err != nil {
		complete("failed", err)
		return GitHubWebhookResult{}, err
	}
	started := 0
	for _, source := range sources {
		if source.Branch != branch || !strings.EqualFold(source.Repository, push.Repository.FullName) {
			continue
		}
		if source.GitHubInstallationID == nil || *source.GitHubInstallationID != push.Installation.ID {
			continue
		}
		app, err := s.apps.GetByID(ctx, source.AppID)
		if err != nil {
			s.log.Error("resolve auto-deploy app", "delivery", deliveryID, "app_id", source.AppID, "err", err)
			continue
		}
		source.ExpectedCommitSHA = push.After
		deployment, app, err := s.deployments.CreateTriggered(ctx, app.Name, source, branch, deliveryID)
		if err != nil {
			s.log.Error("create webhook deployment", "delivery", deliveryID, "app", app.Name, "err", err)
			continue
		}
		started++
		go func(dep domain.Deployment, target domain.App, gitSource domain.GitSource) {
			buildCtx := context.WithoutCancel(ctx)
			if err := s.deployments.Run(buildCtx, dep, target, gitSource, branch); err != nil {
				s.log.Error("webhook git build failed", "delivery", deliveryID, "app", target.Name, "deployment", dep.ID, "err", err)
			}
		}(deployment, app, source)
	}
	if started == 0 {
		complete("ignored", nil)
		return GitHubWebhookResult{Ignored: true}, nil
	}
	complete("accepted", nil)
	return GitHubWebhookResult{Deployments: started}, nil
}

func isZeroSHA(value string) bool {
	return value == "" || strings.Trim(value, "0") == ""
}
