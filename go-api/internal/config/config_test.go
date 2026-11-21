package config

import (
	"strings"
	"testing"
	"time"
)

// clearEnv empties every variable that Load reads, so real env vars
// don't leak into a test. t.Setenv restores the old values after the test.
func clearEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_HOST", "POSTGRES_PORT",
		"HTTP_ADDR", "AI_SERVICE_URL", "AI_TIMEOUT",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		wantErr  string // substring of the error; "" = no error
		wantAddr string
		wantDSN  string
	}{
		{
			name:     "defaults",
			env:      map[string]string{"POSTGRES_USER": "u", "POSTGRES_PASSWORD": "p", "POSTGRES_DB": "d"},
			wantAddr: ":8080",
			wantDSN:  "postgres://u:p@localhost:5432/d?sslmode=disable",
		},
		{
			name: "all values set",
			env: map[string]string{
				"POSTGRES_USER": "u", "POSTGRES_PASSWORD": "p", "POSTGRES_DB": "d",
				"POSTGRES_HOST": "db", "POSTGRES_PORT": "6543", "HTTP_ADDR": ":9000",
			},
			wantAddr: ":9000",
			wantDSN:  "postgres://u:p@db:6543/d?sslmode=disable",
		},
		{
			name:     "special characters in password are escaped",
			env:      map[string]string{"POSTGRES_USER": "u", "POSTGRES_PASSWORD": "p@ss:w/rd", "POSTGRES_DB": "d"},
			wantAddr: ":8080",
			wantDSN:  "postgres://u:p%40ss%3Aw%2Frd@localhost:5432/d?sslmode=disable",
		},
		{
			name:    "missing user",
			env:     map[string]string{"POSTGRES_PASSWORD": "p", "POSTGRES_DB": "d"},
			wantErr: "POSTGRES_USER",
		},
		{
			name:    "missing password",
			env:     map[string]string{"POSTGRES_USER": "u", "POSTGRES_DB": "d"},
			wantErr: "POSTGRES_PASSWORD",
		},
		{
			name:    "missing db",
			env:     map[string]string{"POSTGRES_USER": "u", "POSTGRES_PASSWORD": "p"},
			wantErr: "POSTGRES_DB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.HTTPAddr != tt.wantAddr {
				t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, tt.wantAddr)
			}
			if cfg.DatabaseURL != tt.wantDSN {
				t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, tt.wantDSN)
			}
		})
	}
}

func TestLoad_AI(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantErr     string // substring of the error; "" = no error
		wantURL     string
		wantTimeout time.Duration
	}{
		{name: "defaults", wantURL: "http://localhost:8000", wantTimeout: 30 * time.Second},
		{
			name:    "values set",
			env:     map[string]string{"AI_SERVICE_URL": "http://ai:8000", "AI_TIMEOUT": "45s"},
			wantURL: "http://ai:8000", wantTimeout: 45 * time.Second,
		},
		{
			name:    "https URL",
			env:     map[string]string{"AI_SERVICE_URL": "https://ai.example:8443"},
			wantURL: "https://ai.example:8443", wantTimeout: 30 * time.Second,
		},
		{name: "URL with a bad port", env: map[string]string{"AI_SERVICE_URL": "http://localhost:bad"}, wantErr: "AI_SERVICE_URL"},
		{name: "URL without a scheme", env: map[string]string{"AI_SERVICE_URL": "localhost:8000"}, wantErr: "AI_SERVICE_URL"},
		{name: "URL with another scheme", env: map[string]string{"AI_SERVICE_URL": "ftp://ai"}, wantErr: "AI_SERVICE_URL"},
		{name: "URL without a host", env: map[string]string{"AI_SERVICE_URL": "http://"}, wantErr: "AI_SERVICE_URL"},
		{name: "URL with only a port", env: map[string]string{"AI_SERVICE_URL": "http://:8000"}, wantErr: "AI_SERVICE_URL"},
		{name: "timeout without unit", env: map[string]string{"AI_TIMEOUT": "30"}, wantErr: "AI_TIMEOUT"},
		{name: "zero timeout", env: map[string]string{"AI_TIMEOUT": "0s"}, wantErr: "must be positive"},
		{name: "negative timeout", env: map[string]string{"AI_TIMEOUT": "-5s"}, wantErr: "must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("POSTGRES_USER", "u")
			t.Setenv("POSTGRES_PASSWORD", "p")
			t.Setenv("POSTGRES_DB", "d")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.AIServiceURL != tt.wantURL {
				t.Errorf("AIServiceURL = %q, want %q", cfg.AIServiceURL, tt.wantURL)
			}
			if cfg.AITimeout != tt.wantTimeout {
				t.Errorf("AITimeout = %v, want %v", cfg.AITimeout, tt.wantTimeout)
			}
		})
	}
}

// The startup error is logged, so it must not repeat a secret from the URL.
func TestLoad_AIServiceURLErrorHidesSecrets(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "parse error (bad port)", url: "http://user:s3cret@localhost:bad?token=s3cret"},
		// An unescaped "/" ends the host part, so Go reads the password as the port.
		{name: "parse error (/ in password)", url: "http://user:s3cret/x@ai:8000"},
		{name: "parse error (bad escape in password)", url: "http://user:s3c%zzret@ai:8000"},
		{name: "wrong scheme", url: "ftp://user:s3cret@ai?token=s3cret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("POSTGRES_USER", "u")
			t.Setenv("POSTGRES_PASSWORD", "p")
			t.Setenv("POSTGRES_DB", "d")
			t.Setenv("AI_SERVICE_URL", tt.url)

			_, err := Load()

			if err == nil || !strings.Contains(err.Error(), "AI_SERVICE_URL") {
				t.Fatalf("error = %v, want it to name AI_SERVICE_URL", err)
			}
			// "s3c", not "s3cret": even a part of the secret must not leak.
			if strings.Contains(err.Error(), "s3c") {
				t.Errorf("error %q contains a part of the secret", err)
			}
		})
	}
}
