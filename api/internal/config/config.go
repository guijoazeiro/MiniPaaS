package config

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                    string
	DatabaseURL             string
	DockerHost              string
	CaddyAdminURL           string
	BaseDomain              string
	PublicIP                string
	EncryptionKey           []byte
	JWTSecret               string
	TokenTTL                time.Duration
	AdminUsername           string
	AdminPassword           string
	ImageRetention          int
	HealthCheckInterval     time.Duration
	DeployReadyTimeout      time.Duration
	RestartPolicy           string
	RestartMaxRetries       int
	MaxDeploySize           int64
	MaxRepositorySize       int64
	GitCloneTimeout         time.Duration
	GitHubAppID             int64
	GitHubAppSlug           string
	GitHubAppPrivateKeyPath string
	GitHubAPIURL            string
	GitHubWebhookSecret     string
	DashboardOrigin         string
	RateLimitWindow         time.Duration
	AuthRateLimit           int
	WebhookRateLimit        int
	ContainerMemoryBytes    int64
	ContainerNanoCPUs       int64
	ContainerPidsLimit      int64
	LogLevel                string
}

func Load() (*Config, error) {
	databaseURL, err := mustEnv("DATABASE_URL")
	if err != nil {
		return nil, err
	}
	baseDomain, err := mustEnv("BASE_DOMAIN")
	if err != nil {
		return nil, err
	}
	jwtSecret, err := mustEnv("JWT_SECRET")
	if err != nil {
		return nil, err
	}
	rawEncryptionKey, err := mustEnv("ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}
	encKey, err := parseHexKey(rawEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("config: Failed to parse encryption key: %w", err)
	}

	caddyURL := env("CADDY_ADMIN_URL", "http://localhost:2019")
	if err := requireLocalhost(caddyURL); err != nil {
		return nil, fmt.Errorf("config: CADDY_ADMIN_URL: %w", err)
	}

	ttl, err := time.ParseDuration(env("TOKEN_TTL", "24h"))
	if err != nil {
		return nil, fmt.Errorf("config: TOKEN_TTL: %w", err)
	}

	retention, err := strconv.Atoi(env("IMAGE_RETENTION", "5"))
	if err != nil || retention < 1 {
		return nil, fmt.Errorf("config: IMAGE_RETENTION must be a positive integer")
	}
	healthCheckInterval, err := time.ParseDuration(env("HEALTH_CHECK_INTERVAL", "30s"))
	if err != nil || healthCheckInterval <= 0 {
		return nil, fmt.Errorf("config: HEALTH_CHECK_INTERVAL must be a positive duration")
	}
	deployReadyTimeout, err := time.ParseDuration(env("DEPLOY_READY_TIMEOUT", "60s"))
	if err != nil || deployReadyTimeout <= 0 {
		return nil, fmt.Errorf("config: DEPLOY_READY_TIMEOUT must be a positive duration")
	}
	restartPolicy := env("RESTART_POLICY", "on-failure")
	if restartPolicy != "no" && restartPolicy != "always" && restartPolicy != "on-failure" && restartPolicy != "unless-stopped" {
		return nil, fmt.Errorf("config: RESTART_POLICY is invalid")
	}
	restartMaxRetries, err := strconv.Atoi(env("RESTART_MAX_RETRIES", "3"))
	if err != nil || restartMaxRetries < 0 {
		return nil, fmt.Errorf("config: RESTART_MAX_RETRIES must be a non-negative integer")
	}
	maxDeploySizeMB, err := strconv.ParseInt(env("MAX_DEPLOY_SIZE_MB", "100"), 10, 64)
	if err != nil || maxDeploySizeMB <= 0 || maxDeploySizeMB > 10_240 {
		return nil, fmt.Errorf("config: MAX_DEPLOY_SIZE_MB must be an integer between 1 and 10240")
	}
	maxRepositorySizeMB, err := strconv.ParseInt(env("MAX_REPOSITORY_SIZE_MB", "250"), 10, 64)
	if err != nil || maxRepositorySizeMB <= 0 || maxRepositorySizeMB > 10_240 {
		return nil, fmt.Errorf("config: MAX_REPOSITORY_SIZE_MB must be an integer between 1 and 10240")
	}
	gitCloneTimeout, err := time.ParseDuration(env("GIT_CLONE_TIMEOUT", "10m"))
	if err != nil || gitCloneTimeout <= 0 {
		return nil, fmt.Errorf("config: GIT_CLONE_TIMEOUT must be a positive duration")
	}
	rateLimitWindow, err := time.ParseDuration(env("RATE_LIMIT_WINDOW", "1m"))
	if err != nil || rateLimitWindow <= 0 {
		return nil, fmt.Errorf("config: RATE_LIMIT_WINDOW must be a positive duration")
	}
	authRateLimit, err := parsePositiveLimit("AUTH_RATE_LIMIT", "10")
	if err != nil {
		return nil, err
	}
	webhookRateLimit, err := parsePositiveLimit("WEBHOOK_RATE_LIMIT", "120")
	if err != nil {
		return nil, err
	}
	containerMemoryBytes, err := parseNonNegativeMB("CONTAINER_MEMORY_LIMIT_MB", "0", 1_048_576)
	if err != nil {
		return nil, err
	}
	containerNanoCPUs, err := parseNonNegativeLimit("CONTAINER_NANO_CPUS", "0", 64_000_000_000)
	if err != nil {
		return nil, err
	}
	containerPidsLimit, err := parseNonNegativeLimit("CONTAINER_PIDS_LIMIT", "0", 1_000_000)
	if err != nil {
		return nil, err
	}
	githubAppID, githubAppSlug, githubAppKeyPath, err := githubAppConfig()
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:                    env("PORT", ":8080"),
		DatabaseURL:             databaseURL,
		DockerHost:              os.Getenv("DOCKER_HOST"),
		CaddyAdminURL:           caddyURL,
		BaseDomain:              baseDomain,
		PublicIP:                strings.TrimSpace(os.Getenv("PUBLIC_IP")),
		EncryptionKey:           encKey,
		JWTSecret:               jwtSecret,
		TokenTTL:                ttl,
		AdminUsername:           os.Getenv("ADMIN_USERNAME"),
		AdminPassword:           os.Getenv("ADMIN_PASSWORD"),
		ImageRetention:          retention,
		HealthCheckInterval:     healthCheckInterval,
		DeployReadyTimeout:      deployReadyTimeout,
		RestartPolicy:           restartPolicy,
		RestartMaxRetries:       restartMaxRetries,
		MaxDeploySize:           maxDeploySizeMB * 1024 * 1024,
		MaxRepositorySize:       maxRepositorySizeMB * 1024 * 1024,
		GitCloneTimeout:         gitCloneTimeout,
		GitHubAppID:             githubAppID,
		GitHubAppSlug:           githubAppSlug,
		GitHubAppPrivateKeyPath: githubAppKeyPath,
		GitHubAPIURL:            env("GITHUB_API_URL", "https://api.github.com"),
		GitHubWebhookSecret:     strings.TrimSpace(os.Getenv("GITHUB_WEBHOOK_SECRET")),
		DashboardOrigin:         env("DASHBOARD_ORIGIN", "http://localhost:3000"),
		RateLimitWindow:         rateLimitWindow,
		AuthRateLimit:           authRateLimit,
		WebhookRateLimit:        webhookRateLimit,
		ContainerMemoryBytes:    containerMemoryBytes,
		ContainerNanoCPUs:       containerNanoCPUs,
		ContainerPidsLimit:      containerPidsLimit,
		LogLevel:                env("LOG_LEVEL", "info"),
	}, nil

}

func parsePositiveLimit(key, fallback string) (int, error) {
	value, err := strconv.Atoi(env(key, fallback))
	if err != nil || value < 1 || value > 1_000_000 {
		return 0, fmt.Errorf("config: %s must be an integer between 1 and 1000000", key)
	}
	return value, nil
}

func parseNonNegativeLimit(key, fallback string, max int64) (int64, error) {
	value, err := strconv.ParseInt(env(key, fallback), 10, 64)
	if err != nil || value < 0 || value > max {
		return 0, fmt.Errorf("config: %s must be an integer between 0 and %d", key, max)
	}
	return value, nil
}

func parseNonNegativeMB(key, fallback string, maxMB int64) (int64, error) {
	value, err := parseNonNegativeLimit(key, fallback, maxMB)
	if err != nil {
		return 0, err
	}
	return value * 1024 * 1024, nil
}

func githubAppConfig() (int64, string, string, error) {
	rawID := strings.TrimSpace(os.Getenv("GITHUB_APP_ID"))
	slug := strings.TrimSpace(os.Getenv("GITHUB_APP_SLUG"))
	keyPath := strings.TrimSpace(os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"))
	configured := rawID != "" || slug != "" || keyPath != ""
	if !configured {
		return 0, "", "", nil
	}
	if rawID == "" || slug == "" || keyPath == "" {
		return 0, "", "", fmt.Errorf("config: GITHUB_APP_ID, GITHUB_APP_SLUG and GITHUB_APP_PRIVATE_KEY_PATH must be set together")
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return 0, "", "", fmt.Errorf("config: GITHUB_APP_ID must be a positive integer")
	}
	return id, slug, keyPath, nil
}

func requireLocalhost(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	host := u.Hostname()
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("must point at localhost/loopback, got %q", host)
}

func mustEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("config: environment variable %s is required", key)
	}
	return value, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseHexKey(raw string) ([]byte, error) {
	key, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}
	return key, nil
}
