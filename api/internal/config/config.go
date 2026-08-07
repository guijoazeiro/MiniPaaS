package config

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
)

type Config struct {
	Port          string
	DatabaseURL   string
	DockerHost    string
	CaddyAdminURL string
	BaseDomain    string
	EncryptionKey []byte
	JWTSecret     string
	LogLevel      string
}

func Load() (*Config, error) {
	encKey, err := parseHexKey(mustEnv("ENCRYPTION_KEY"))
	if err != nil {
		return nil, fmt.Errorf("config: Failed to parse encryption key: %w", err)
	}

	caddyURL := env("CADDY_ADMIN_URL", "http://localhost:2019")
	if err := requireLocalhost(caddyURL); err != nil {
		return nil, fmt.Errorf("config: CADDY_ADMIN_URL: %w", err)
	}

	return &Config{
		Port:          env("PORT", ":8080"),
		DatabaseURL:   mustEnv("DATABASE_URL"),
		DockerHost:    os.Getenv("DOCKER_HOST"),
		CaddyAdminURL: caddyURL,
		BaseDomain:    mustEnv("BASE_DOMAIN"),
		EncryptionKey: encKey,
		JWTSecret:     mustEnv("JWT_SECRET"),
		LogLevel:      env("LOG_LEVEL", "info"),
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

func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("config: Environment variable %s is required", key))
	}
	return value
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
