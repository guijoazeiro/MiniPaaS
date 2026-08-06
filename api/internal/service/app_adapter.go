package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type AppFacade struct {
	App *AppService
	Dep *DeploymentService
}

func (f *AppFacade) Create(ctx context.Context, name string) (domain.App, error) {
	return f.App.Create(ctx, name)
}
func (f *AppFacade) GetByName(ctx context.Context, name string) (domain.App, error) {
	return f.App.GetByName(ctx, name)
}
func (f *AppFacade) List(ctx context.Context) ([]domain.App, error) { return f.App.List(ctx) }
func (f *AppFacade) Delete(ctx context.Context, id uuid.UUID) error { return f.App.Delete(ctx, id) }
func (f *AppFacade) StopApp(ctx context.Context, id uuid.UUID) error {
	return f.Dep.StopApp(ctx, id)
}
