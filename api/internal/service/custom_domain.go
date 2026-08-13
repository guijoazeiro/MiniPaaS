package service

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store"
)

type DNSResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

type CustomDomainRouter interface {
	SwitchCustomRoute(ctx context.Context, domainID, hostname string, port int) error
	RemoveCustomRoute(ctx context.Context, domainID string) error
}

// CustomDomainRouteSync is consumed by deployments and rollbacks so verified
// custom domains follow the same active container as the default subdomain.
type CustomDomainRouteSync interface {
	SyncRoutes(ctx context.Context, appID uuid.UUID, port int) error
	RemoveRoutes(ctx context.Context, appID uuid.UUID) error
}

type CustomDomainService struct {
	domains    store.CustomDomainStore
	apps       store.AppStore
	deployments store.DeploymentStore
	router     CustomDomainRouter
	resolver   DNSResolver
	baseDomain string
	expectedIP string
}

func NewCustomDomainService(domains store.CustomDomainStore, apps store.AppStore, deployments store.DeploymentStore, router CustomDomainRouter, baseDomain, expectedIP string) *CustomDomainService {
	return &CustomDomainService{
		domains: domains, apps: apps, deployments: deployments, router: router,
		resolver: net.DefaultResolver, baseDomain: strings.ToLower(strings.TrimSuffix(strings.TrimSpace(baseDomain), ".")), expectedIP: strings.TrimSpace(expectedIP),
	}
}

func (s *CustomDomainService) List(ctx context.Context, appName string) ([]domain.CustomDomain, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return nil, err
	}
	items, err := s.domains.ListByApp(ctx, app.ID)
	if err != nil {
		return nil, fmt.Errorf("service.ListCustomDomains: %w", err)
	}
	if items == nil {
		items = []domain.CustomDomain{}
	}
	return items, nil
}

func (s *CustomDomainService) Create(ctx context.Context, appName, hostname string) (domain.CustomDomain, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	hostname, err = normalizeHostname(hostname, s.baseDomain)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	domainRecord, err := s.domains.Create(ctx, app.ID, hostname)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	return domainRecord, nil
}

func (s *CustomDomainService) Verify(ctx context.Context, appName string, id uuid.UUID) (domain.CustomDomain, error) {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	domainRecord, err := s.domains.Get(ctx, id)
	if err != nil {
		return domain.CustomDomain{}, err
	}
	if domainRecord.AppID != app.ID {
		return domain.CustomDomain{}, domain.ErrCustomDomainNotFound
	}
	verifiedAt := time.Now().UTC()
	addresses, lookupErr := s.resolver.LookupHost(ctx, domainRecord.Hostname)
	if lookupErr != nil || !s.matchesExpectedIP(addresses) {
		message := "DNS did not resolve to the MiniPaaS host"
		if lookupErr != nil {
			message = lookupErr.Error()
		}
		_ = s.domains.UpdateVerification(ctx, id, domain.CustomDomainStatusError, message, nil)
		return domain.CustomDomain{}, fmt.Errorf("%w: %s", domain.ErrCustomDomainDNSNotConfigured, message)
	}

	status := domain.CustomDomainStatusVerified
	if active, activeErr := s.activeDeployment(ctx, app.ID); activeErr == nil && active.ContainerID != "" {
		if err := s.router.SwitchCustomRoute(ctx, domainRecord.ID.String(), domainRecord.Hostname, active.Port); err != nil {
			_ = s.domains.UpdateVerification(ctx, id, domain.CustomDomainStatusError, err.Error(), &verifiedAt)
			return domain.CustomDomain{}, fmt.Errorf("service.VerifyCustomDomain: route: %w", err)
		}
		status = domain.CustomDomainStatusActive
	}
	if err := s.domains.UpdateVerification(ctx, id, status, "", &verifiedAt); err != nil {
		return domain.CustomDomain{}, fmt.Errorf("service.VerifyCustomDomain: persist: %w", err)
	}
	domainRecord.Status = status
	domainRecord.LastError = ""
	domainRecord.VerifiedAt = &verifiedAt
	return domainRecord, nil
}

func (s *CustomDomainService) Delete(ctx context.Context, appName string, id uuid.UUID) error {
	app, err := s.apps.GetByName(ctx, appName)
	if err != nil {
		return err
	}
	domainRecord, err := s.domains.Get(ctx, id)
	if err != nil {
		return err
	}
	if domainRecord.AppID != app.ID {
		return domain.ErrCustomDomainNotFound
	}
	if domainRecord.VerifiedAt != nil {
		if err := s.router.RemoveCustomRoute(ctx, domainRecord.ID.String()); err != nil {
			return fmt.Errorf("service.DeleteCustomDomain: route: %w", err)
		}
	}
	return s.domains.Delete(ctx, id)
}

func (s *CustomDomainService) SyncRoutes(ctx context.Context, appID uuid.UUID, port int) error {
	items, err := s.domains.ListByApp(ctx, appID)
	if err != nil {
		return fmt.Errorf("service.SyncCustomDomainRoutes: list: %w", err)
	}
	for _, item := range items {
		if item.VerifiedAt == nil || item.Status == domain.CustomDomainStatusPending {
			continue
		}
		if err := s.router.SwitchCustomRoute(ctx, item.ID.String(), item.Hostname, port); err != nil {
			_ = s.domains.UpdateVerification(ctx, item.ID, domain.CustomDomainStatusError, err.Error(), item.VerifiedAt)
			return fmt.Errorf("service.SyncCustomDomainRoutes: %s: %w", item.Hostname, err)
		}
		if err := s.domains.UpdateVerification(ctx, item.ID, domain.CustomDomainStatusActive, "", item.VerifiedAt); err != nil {
			return fmt.Errorf("service.SyncCustomDomainRoutes: persist %s: %w", item.Hostname, err)
		}
	}
	return nil
}

func (s *CustomDomainService) RemoveRoutes(ctx context.Context, appID uuid.UUID) error {
	items, err := s.domains.ListByApp(ctx, appID)
	if err != nil {
		return fmt.Errorf("service.RemoveCustomDomainRoutes: list: %w", err)
	}
	for _, item := range items {
		if item.VerifiedAt == nil {
			continue
		}
		if err := s.router.RemoveCustomRoute(ctx, item.ID.String()); err != nil {
			return fmt.Errorf("service.RemoveCustomDomainRoutes: %s: %w", item.Hostname, err)
		}
		if err := s.domains.UpdateVerification(ctx, item.ID, domain.CustomDomainStatusVerified, "", item.VerifiedAt); err != nil {
			return fmt.Errorf("service.RemoveCustomDomainRoutes: persist %s: %w", item.Hostname, err)
		}
	}
	return nil
}

func (s *CustomDomainService) activeDeployment(ctx context.Context, appID uuid.UUID) (domain.Deployment, error) {
	if s.deployments == nil {
		return domain.Deployment{}, domain.ErrDeploymentNotFound
	}
	return s.deployments.GetActive(ctx, appID)
}

func (s *CustomDomainService) matchesExpectedIP(addresses []string) bool {
	if s.expectedIP == "" {
		return len(addresses) > 0
	}
	expected := net.ParseIP(s.expectedIP)
	if expected == nil {
		return false
	}
	for _, address := range addresses {
		if ip := net.ParseIP(address); ip != nil && ip.Equal(expected) {
			return true
		}
	}
	return false
}

func normalizeHostname(raw, baseDomain string) (string, error) {
	host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
	if host == "" || len(host) > 253 || host == baseDomain || strings.HasSuffix(host, "."+baseDomain) {
		return "", domain.ErrCustomDomainInvalid
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "", domain.ErrCustomDomainInvalid
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", domain.ErrCustomDomainInvalid
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return "", domain.ErrCustomDomainInvalid
			}
		}
	}
	return host, nil
}
