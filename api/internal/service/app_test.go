package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type quotaAppStore struct {
	store.AppStore
	count int64
}

func (s *quotaAppStore) Count(context.Context) (int64, error) { return s.count, nil }
func (s *quotaAppStore) Create(context.Context, string) (domain.App, error) {
	return domain.App{Name: "created"}, nil
}

func TestAppServiceRejectsUserBeyondCapacity(t *testing.T) {
	owner := uuid.New()
	ctx := authctx.WithIdentity(context.Background(), authctx.Identity{UserID: owner, Method: authctx.AuthMethodSession})
	svc := NewAppService(&quotaAppStore{count: 2}, AppServiceOptions{MaxAppsPerUser: 2})
	if _, err := svc.Create(ctx, "third-app"); err != domain.ErrAppCapacityExceeded {
		t.Fatalf("Create() error = %v, want %v", err, domain.ErrAppCapacityExceeded)
	}
}
