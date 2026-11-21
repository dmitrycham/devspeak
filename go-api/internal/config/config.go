// Package config reads the application settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"
)

// Config holds the application settings.
type Config struct {
	HTTPAddr     string        // address the HTTP server listens on, for example ":8080"
	DatabaseURL  string        // PostgreSQL connection string for pgx
	AIServiceURL string        // base URL of ai-service, for example "http://localhost:8000"
	AITimeout    time.Duration // limit for one call to ai-service
}

// Load reads the config from environment variables.
// POSTGRES_USER, POSTGRES_PASSWORD and POSTGRES_DB are required.
// AI_TIMEOUT is a Go duration, for example "30s".
func Load() (Config, error) {
	user, err := required("POSTGRES_USER")
	if err != nil {
		return Config{}, err
	}
	password, err := required("POSTGRES_PASSWORD")
	if err != nil {
		return Config{}, err
	}
	db, err := required("POSTGRES_DB")
	if err != nil {
		return Config{}, err
	}

	// url.URL escapes special characters, so a password like "p@ss" is safe.
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(withDefault("POSTGRES_HOST", "localhost"), withDefault("POSTGRES_PORT", "5432")),
		Path:     db,
		RawQuery: "sslmode=disable",
	}

	aiURL, err := parseAIServiceURL(withDefault("AI_SERVICE_URL", "http://localhost:8000"))
	if err != nil {
		return Config{}, err
	}

	aiTimeout, err := time.ParseDuration(withDefault("AI_TIMEOUT", "30s"))
	if err != nil {
		return Config{}, fmt.Errorf("env AI_TIMEOUT: %w", err)
	}
	if aiTimeout <= 0 {
		return Config{}, fmt.Errorf("env AI_TIMEOUT must be positive, got %s", aiTimeout)
	}

	return Config{
		HTTPAddr:     withDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:  dsn.String(),
		AIServiceURL: aiURL,
		AITimeout:    aiTimeout,
	}, nil
}

// parseAIServiceURL checks the URL at startup, so a typo stops the app
// instead of failing every enrich request later.
func parseAIServiceURL(raw string) (string, error) {
	// The errors never include the URL: it may hold a password or a token,
	// and the startup error goes to the log.
	u, err := url.Parse(raw)
	if err != nil {
		// A fixed message, not err: *url.Error repeats the whole URL, and even
		// its inner reason can quote a part of it. With an unescaped "/" in the
		// password, "invalid port" quotes the password itself.
		return "", errors.New("env AI_SERVICE_URL: invalid URL")
	}
	// url.Parse accepts "localhost:8000": it reads "localhost" as the scheme
	// and leaves the host empty. Hostname, not Host: for "http://:8000"
	// Host is ":8000", but the host name is still empty.
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", errors.New("env AI_SERVICE_URL must be an absolute http(s) URL with a host")
	}
	return raw, nil
}

func required(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("env %s is required", key)
	}
	return v, nil
}

func withDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
