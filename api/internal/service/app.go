package service

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/authctx"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

var appNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type AppService struct {
	apps           store.AppStore
	maxAppsPerUser int
	createMu       sync.Mutex
}

type AppServiceOptions struct {
	MaxAppsPerUser int
}

type AppCounter interface {
	Count(ctx context.Context) (int64, error)
}

func NewAppService(apps store.AppStore, options ...AppServiceOptions) *AppService {
	var maxAppsPerUser int
	for _, option := range options {
		if option.MaxAppsPerUser > 0 {
			maxAppsPerUser = option.MaxAppsPerUser
		}
	}
	return &AppService{apps: apps, maxAppsPerUser: maxAppsPerUser}
}

func (s *AppService) Create(ctx context.Context, name string) (domain.App, error) {
	if !appNameRE.MatchString(name) {
		return domain.App{}, domain.ErrAppNameInvalid
	}
	if s.maxAppsPerUser > 0 {
		if _, ok := authctx.UserID(ctx); ok {
			counter, hasCounter := s.apps.(AppCounter)
			if hasCounter {
				s.createMu.Lock()
				defer s.createMu.Unlock()
				count, err := counter.Count(ctx)
				if err != nil {
					return domain.App{}, fmt.Errorf("service.Create: count apps: %w", err)
				}
				if count >= int64(s.maxAppsPerUser) {
					return domain.App{}, domain.ErrAppCapacityExceeded
				}
			}
		}
	}
	return s.apps.Create(ctx, name)
}

func (s *AppService) GetByName(ctx context.Context, name string) (domain.App, error) {
	return s.apps.GetByName(ctx, name)
}

func (s *AppService) List(ctx context.Context) ([]domain.App, error) {
	return s.apps.List(ctx)
}

func (s *AppService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.apps.Delete(ctx, id)
}

func (s *AppService) SetStatus(ctx context.Context, id uuid.UUID, status domain.AppStatus) error {
	if err := s.apps.UpdateStatus(ctx, id, status); err != nil {
		return fmt.Errorf("service.SetStatus: %w", err)
	}
	return nil
}
