package service

import (
	"context"
	"errors"

	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type ManagedContainerCleaner interface {
	ListManagedContainers(ctx context.Context) ([]docker.ManagedContainer, error)
	RemoveContainer(ctx context.Context, id string) error
}

// ReconcileManagedContainers removes only containers created by MiniPaaS that
// are no longer referenced by the deployment store. Unlabelled Docker
// containers are never touched, which keeps reconciliation safe on shared
// development hosts.
func ReconcileManagedContainers(ctx context.Context, cleaner ManagedContainerCleaner, deployments store.DeploymentStore) (int, error) {
	running, err := deployments.ListRunning(ctx)
	if err != nil {
		return 0, err
	}
	referenced := referencedContainerIDs(running)
	if candidates, ok := deployments.(store.DeploymentCandidateStore); ok {
		items, candidateErr := candidates.ListCandidates(ctx)
		if candidateErr != nil {
			return 0, candidateErr
		}
		referenced = referencedContainerIDs(append(running, items...))
	}
	return reconcileManagedContainerIDs(ctx, cleaner, referenced)
}

func referencedContainerIDs(deployments []domain.Deployment) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, deployment := range deployments {
		if deployment.ContainerID != "" {
			ids[deployment.ContainerID] = struct{}{}
		}
		if deployment.CandidateContainerID != "" {
			ids[deployment.CandidateContainerID] = struct{}{}
		}
	}
	return ids
}

func reconcileManagedContainerIDs(ctx context.Context, cleaner ManagedContainerCleaner, referenced map[string]struct{}) (int, error) {
	items, err := cleaner.ListManagedContainers(ctx)
	if err != nil {
		return 0, err
	}
	removed := 0
	var cleanupErr error
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		if _, ok := referenced[item.ID]; ok {
			continue
		}
		if err := cleaner.RemoveContainer(ctx, item.ID); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		removed++
	}
	return removed, cleanupErr
}
