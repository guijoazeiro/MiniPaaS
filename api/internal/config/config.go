package config

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                string
	DatabaseURL         string
	DockerHost          string
	CaddyAdminURL       string
	BaseDomain          string
	EncryptionKey       []byte
	JWTSecret           string
	TokenTTL            time.Duration
	AdminUsername       string
	AdminPassword       string
	ImageRetention      int
	HealthCheckInterval time.Duration
	RestartPolicy       string
	RestartMaxRetries   int
	DashboardOrigin     string
	LogLevel            string
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
	restartPolicy := env("RESTART_POLICY", "on-failure")
	if restartPolicy != "no" && restartPolicy != "always" && restartPolicy != "on-failure" && restartPolicy != "unless-stopped" {
		return nil, fmt.Errorf("config: RESTART_POLICY is invalid")
	}
	restartMaxRetries, err := strconv.Atoi(env("RESTART_MAX_RETRIES", "3"))
	if err != nil || restartMaxRetries < 0 {
		return nil, fmt.Errorf("config: RESTART_MAX_RETRIES must be a non-negative integer")
	}

	return &Config{
		Port:                env("PORT", ":8080"),
		DatabaseURL:         databaseURL,
		DockerHost:          os.Getenv("DOCKER_HOST"),
		CaddyAdminURL:       caddyURL,
		BaseDomain:          baseDomain,
		EncryptionKey:       encKey,
		JWTSecret:           jwtSecret,
		TokenTTL:            ttl,
		AdminUsername:       os.Getenv("ADMIN_USERNAME"),
		AdminPassword:       os.Getenv("ADMIN_PASSWORD"),
		ImageRetention:      retention,
		HealthCheckInterval: healthCheckInterval,
		RestartPolicy:       restartPolicy,
		RestartMaxRetries:   restartMaxRetries,
		DashboardOrigin:     env("DASHBOARD_ORIGIN", "http://localhost:3000"),
		LogLevel:            env("LOG_LEVEL", "info"),
	}, nil

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
