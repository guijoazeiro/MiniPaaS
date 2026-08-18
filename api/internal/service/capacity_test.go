package service

import (
	"context"
	"testing"

	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type capacityAppStore struct {
	store.AppStore
	apps []domain.App
}

func (s *capacityAppStore) List(context.Context) ([]domain.App, error) { return s.apps, nil }

type capacityQueue struct{}

func (capacityQueue) BuildQueueStats() domain.BuildQueueStats {
	return domain.BuildQueueStats{Limit: 2, Active: 1, Queued: 3}
}

func TestCapacityServiceSummarizesAppsAndQueue(t *testing.T) {
	svc := NewCapacityService(&capacityAppStore{apps: []domain.App{{Status: domain.AppStatusRunning}, {Status: domain.AppStatusFailed}}}, capacityQueue{}, CapacityOptions{MaxAppsPerUser: 20, ContainerMemoryLimitBytes: 64})
	snapshot, err := svc.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.AppsTotal != 2 || snapshot.AppsRunning != 1 || snapshot.MaxAppsPerUser != 20 {
		t.Fatalf("app capacity = %#v", snapshot)
	}
	if snapshot.Builds != (domain.BuildQueueStats{Limit: 2, Active: 1, Queued: 3}) {
		t.Fatalf("build capacity = %#v", snapshot.Builds)
	}
}
