// Package config reads the application settings from environment variables.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

// Config holds the application settings.
type Config struct {
	HTTPAddr    string // address the HTTP server listens on, for example ":8080"
	DatabaseURL string // PostgreSQL connection string for pgx
}

// Load reads the config from environment variables.
// POSTGRES_USER, POSTGRES_PASSWORD and POSTGRES_DB are required.
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

	return Config{
		HTTPAddr:    withDefault("HTTP_ADDR", ":8080"),
		DatabaseURL: dsn.String(),
	}, nil
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
