package config

import (
	"strings"
	"testing"
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
}
