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
