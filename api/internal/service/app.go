package service

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

var appNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type AppService struct {
	apps store.AppStore
}

func NewAppService(apps store.AppStore) *AppService {
	return &AppService{apps: apps}
}

func (s *AppService) Create(ctx context.Context, name string) (domain.App, error) {
	if !appNameRE.MatchString(name) {
		return domain.App{}, domain.ErrAppNameInvalid
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
