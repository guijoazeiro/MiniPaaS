package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadMissingRequiredVariableReturnsError(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("BASE_DOMAIN", "example.test")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("a", 64))

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("Load() error = %v, want missing DATABASE_URL error", err)
	}
}

func TestLoadRejectsInvalidMaxDeploySize(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost/minipaas")
	t.Setenv("BASE_DOMAIN", "example.test")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("a", 64))
	t.Setenv("CADDY_ADMIN_URL", "http://localhost:2019")
	t.Setenv("MAX_DEPLOY_SIZE_MB", "0")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "MAX_DEPLOY_SIZE_MB") {
		t.Fatalf("Load() error = %v, want invalid MAX_DEPLOY_SIZE_MB error", err)
	}
}

func TestLoadUsesDefaultMaxDeploySize(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost/minipaas")
	t.Setenv("BASE_DOMAIN", "example.test")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("a", 64))
	t.Setenv("CADDY_ADMIN_URL", "http://localhost:2019")
	t.Setenv("MAX_DEPLOY_SIZE_MB", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxDeploySize != 100*1024*1024 {
		t.Fatalf("MaxDeploySize = %d, want %d", cfg.MaxDeploySize, 100*1024*1024)
	}
	if cfg.MaxRepositorySize != 250*1024*1024 {
		t.Fatalf("MaxRepositorySize = %d, want %d", cfg.MaxRepositorySize, 250*1024*1024)
	}
	if cfg.GitCloneTimeout.String() != "10m0s" {
		t.Fatalf("GitCloneTimeout = %s, want 10m", cfg.GitCloneTimeout)
	}
	if cfg.BuildTimeout != 15*time.Minute || cfg.MaxConcurrentBuilds != 2 {
		t.Fatalf("build limits = timeout:%s concurrency:%d, want 15m/2", cfg.BuildTimeout, cfg.MaxConcurrentBuilds)
	}
	if cfg.RateLimitWindow != time.Minute {
		t.Fatalf("RateLimitWindow = %s, want 1m", cfg.RateLimitWindow)
	}
	if cfg.AuthRateLimit != 10 || cfg.WebhookRateLimit != 120 {
		t.Fatalf("rate limits = auth:%d webhook:%d, want auth:10 webhook:120", cfg.AuthRateLimit, cfg.WebhookRateLimit)
	}
	if cfg.ContainerMemoryBytes != 0 || cfg.ContainerNanoCPUs != 0 || cfg.ContainerPidsLimit != 0 {
		t.Fatalf("container limits = memory:%d cpu:%d pids:%d, want all unlimited by default", cfg.ContainerMemoryBytes, cfg.ContainerNanoCPUs, cfg.ContainerPidsLimit)
	}
	if cfg.ReadinessTimeout != 3*time.Second {
		t.Fatalf("ReadinessTimeout = %s, want 3s", cfg.ReadinessTimeout)
	}
}

func TestLoadRejectsInvalidBuildLimits(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost/minipaas")
	t.Setenv("BASE_DOMAIN", "example.test")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("a", 64))
	t.Setenv("CADDY_ADMIN_URL", "http://localhost:2019")
	t.Setenv("BUILD_TIMEOUT", "0s")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "BUILD_TIMEOUT") {
		t.Fatalf("Load() error = %v, want invalid BUILD_TIMEOUT error", err)
	}

	t.Setenv("BUILD_TIMEOUT", "15m")
	t.Setenv("MAX_CONCURRENT_BUILDS", "0")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MAX_CONCURRENT_BUILDS") {
		t.Fatalf("Load() error = %v, want invalid MAX_CONCURRENT_BUILDS error", err)
	}
}

func TestLoadParsesContainerResourceLimits(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost/minipaas")
	t.Setenv("BASE_DOMAIN", "example.test")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("a", 64))
	t.Setenv("CADDY_ADMIN_URL", "http://localhost:2019")
	t.Setenv("CONTAINER_MEMORY_LIMIT_MB", "64")
	t.Setenv("CONTAINER_NANO_CPUS", "500000000")
	t.Setenv("CONTAINER_PIDS_LIMIT", "128")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ContainerMemoryBytes != 64*1024*1024 {
		t.Fatalf("ContainerMemoryBytes = %d, want %d", cfg.ContainerMemoryBytes, 64*1024*1024)
	}
	if cfg.ContainerNanoCPUs != 500000000 {
		t.Fatalf("ContainerNanoCPUs = %d, want 500000000", cfg.ContainerNanoCPUs)
	}
	if cfg.ContainerPidsLimit != 128 {
		t.Fatalf("ContainerPidsLimit = %d, want 128", cfg.ContainerPidsLimit)
	}
}

func TestLoadRejectsPartialGitHubAppConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost/minipaas")
	t.Setenv("BASE_DOMAIN", "example.test")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("a", 64))
	t.Setenv("CADDY_ADMIN_URL", "http://localhost:2019")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_SLUG", "mini-paas")
	t.Setenv("GITHUB_APP_PRIVATE_KEY_PATH", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("Load() error = %v, want incomplete GitHub App error", err)
	}
}
