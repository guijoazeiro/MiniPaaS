package cmd

import "testing"

func TestConfiguredTokenPrefersEnvironment(t *testing.T) {
	t.Setenv("MINIPAAS_TOKEN", "  env-token  ")
	if got := configuredToken("saved-token"); got != "env-token" {
		t.Fatalf("configuredToken() = %q, want env-token", got)
	}
}

func TestConfiguredTokenFallsBackToConfig(t *testing.T) {
	t.Setenv("MINIPAAS_TOKEN", "")
	if got := configuredToken("saved-token"); got != "saved-token" {
		t.Fatalf("configuredToken() = %q, want saved-token", got)
	}
}
