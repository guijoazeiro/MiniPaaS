package service

import (
	"context"
	"fmt"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type CapacityOptions struct {
	MaxAppsPerUser            int
	ContainerMemoryLimitBytes int64
	ContainerNanoCPUs         int64
	ContainerPidsLimit        int64
}

type CapacityQueue interface {
	BuildQueueStats() domain.BuildQueueStats
}

type CapacityService struct {
	apps  store.AppStore
	queue CapacityQueue
	opts  CapacityOptions
}

func NewCapacityService(apps store.AppStore, queue CapacityQueue, opts CapacityOptions) *CapacityService {
	return &CapacityService{apps: apps, queue: queue, opts: opts}
}

func (s *CapacityService) Get(ctx context.Context) (domain.CapacitySnapshot, error) {
	apps, err := s.apps.List(ctx)
	if err != nil {
		return domain.CapacitySnapshot{}, fmt.Errorf("service.GetCapacity: list apps: %w", err)
	}
	snapshot := domain.CapacitySnapshot{
		AppsTotal:                 len(apps),
		MaxAppsPerUser:            s.opts.MaxAppsPerUser,
		ContainerMemoryLimitBytes: s.opts.ContainerMemoryLimitBytes,
		ContainerNanoCPUs:         s.opts.ContainerNanoCPUs,
		ContainerPidsLimit:        s.opts.ContainerPidsLimit,
	}
	for _, app := range apps {
		if app.Status == domain.AppStatusRunning {
			snapshot.AppsRunning++
		}
	}
	if s.queue != nil {
		snapshot.Builds = s.queue.BuildQueueStats()
	}
	return snapshot, nil
}
