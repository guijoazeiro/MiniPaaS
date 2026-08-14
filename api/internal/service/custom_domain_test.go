package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
)

type customDomainTestStore struct {
	item domain.CustomDomain
}

func (s *customDomainTestStore) Create(context.Context, uuid.UUID, string) (domain.CustomDomain, error) {
	return s.item, nil
}
func (s *customDomainTestStore) Get(context.Context, uuid.UUID) (domain.CustomDomain, error) {
	if s.item.ID == uuid.Nil {
		return domain.CustomDomain{}, domain.ErrCustomDomainNotFound
	}
	return s.item, nil
}
func (s *customDomainTestStore) ListByApp(context.Context, uuid.UUID) ([]domain.CustomDomain, error) {
	if s.item.ID == uuid.Nil {
		return []domain.CustomDomain{}, nil
	}
	return []domain.CustomDomain{s.item}, nil
}
func (s *customDomainTestStore) UpdateVerification(_ context.Context, id uuid.UUID, status domain.CustomDomainStatus, lastError string, verifiedAt *time.Time) error {
	if s.item.ID != id {
		return errors.New("unexpected domain")
	}
	s.item.Status, s.item.LastError = status, lastError
	s.item.VerifiedAt = verifiedAt
	return nil
}
func (s *customDomainTestStore) Delete(context.Context, uuid.UUID) error { return nil }

type customDomainTestApps struct{ app domain.App }

func (s *customDomainTestApps) Create(context.Context, string) (domain.App, error) { return s.app, nil }
func (s *customDomainTestApps) GetByName(context.Context, string) (domain.App, error) {
	return s.app, nil
}
func (s *customDomainTestApps) GetByID(context.Context, uuid.UUID) (domain.App, error) {
	return s.app, nil
}
func (s *customDomainTestApps) List(context.Context) ([]domain.App, error) {
	return []domain.App{s.app}, nil
}
func (s *customDomainTestApps) UpdateStatus(context.Context, uuid.UUID, domain.AppStatus) error {
	return nil
}
func (s *customDomainTestApps) UpdatePublicURL(context.Context, uuid.UUID, string) error { return nil }
func (s *customDomainTestApps) Delete(context.Context, uuid.UUID) error                  { return nil }

type customDomainTestDeployments struct{ active domain.Deployment }

func (s *customDomainTestDeployments) Create(context.Context, uuid.UUID, string) (domain.Deployment, error) {
	return s.active, nil
}
func (s *customDomainTestDeployments) GetByID(context.Context, uuid.UUID) (domain.Deployment, error) {
	return s.active, nil
}
func (s *customDomainTestDeployments) GetActive(context.Context, uuid.UUID) (domain.Deployment, error) {
	return s.active, nil
}
func (s *customDomainTestDeployments) ListRunning(context.Context) ([]domain.Deployment, error) {
	return []domain.Deployment{s.active}, nil
}
func (s *customDomainTestDeployments) ListByApp(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return []domain.Deployment{s.active}, nil
}
func (s *customDomainTestDeployments) ListAll(context.Context, string, string, int, int) ([]domain.DeploymentListItem, error) {
	return nil, nil
}
func (s *customDomainTestDeployments) CountAll(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (s *customDomainTestDeployments) ListForRetention(context.Context, uuid.UUID, int) ([]domain.Deployment, error) {
	return nil, nil
}
func (s *customDomainTestDeployments) UpdateRunning(context.Context, uuid.UUID, string, int, string, int) error {
	return nil
}
func (s *customDomainTestDeployments) UpdateStatus(context.Context, uuid.UUID, domain.DeploymentStatus) error {
	return nil
}

type customDomainTestResolver struct{ addresses []string }

func (r customDomainTestResolver) LookupHost(context.Context, string) ([]string, error) {
	return r.addresses, nil
}

type customDomainTestRouter struct{ calls []string }

func (r *customDomainTestRouter) SwitchCustomRoute(_ context.Context, id, hostname string, port int) error {
	r.calls = append(r.calls, id+"|"+hostname+"|"+customItoa(port))
	return nil
}
func (r *customDomainTestRouter) RemoveCustomRoute(context.Context, string) error { return nil }

func customItoa(value int) string {
	if value == 0 {
		return "0"
	}
	result := ""
	for value > 0 {
		result = string(rune('0'+value%10)) + result
		value /= 10
	}
	return result
}

func TestCustomDomainVerifyActivatesRoute(t *testing.T) {
	appID, domainID := uuid.New(), uuid.New()
	store := &customDomainTestStore{item: domain.CustomDomain{ID: domainID, AppID: appID, Hostname: "api.example.com", Status: domain.CustomDomainStatusPending}}
	router := &customDomainTestRouter{}
	service := NewCustomDomainService(store, &customDomainTestApps{app: domain.App{ID: appID, Name: "api"}}, &customDomainTestDeployments{active: domain.Deployment{ID: uuid.New(), AppID: appID, ContainerID: "container", Port: 4567, Status: domain.DeploymentStatusRunning}}, router, "minipaas.local", "")
	service.resolver = customDomainTestResolver{addresses: []string{"203.0.113.10"}}

	verified, err := service.Verify(context.Background(), "api", domainID)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Status != domain.CustomDomainStatusActive || store.item.Status != domain.CustomDomainStatusActive {
		t.Fatalf("verified = %#v, stored = %#v", verified, store.item)
	}
	if len(router.calls) != 1 || router.calls[0] != domainID.String()+"|api.example.com|4567" {
		t.Fatalf("route calls = %v", router.calls)
	}
}

func TestNormalizeHostnameRejectsMiniPaaSSubdomains(t *testing.T) {
	if _, err := normalizeHostname("app.minipaas.local", "minipaas.local"); !errors.Is(err, domain.ErrCustomDomainInvalid) {
		t.Fatalf("expected built-in subdomain rejection, got %v", err)
	}
	got, err := normalizeHostname(" API.Example.COM. ", "minipaas.local")
	if err != nil || got != "api.example.com" {
		t.Fatalf("normalized hostname = %q, err = %v", got, err)
	}
}

func TestNormalizeHostnameConvertsInternationalizedDomain(t *testing.T) {
	got, err := normalizeHostname("média.example.com", "minipaas.local")
	if err != nil {
		t.Fatalf("normalizeHostname() error = %v", err)
	}
	if got != "xn--mdia-bpa.example.com" {
		t.Fatalf("normalized hostname = %q, want punycode form", got)
	}
}
