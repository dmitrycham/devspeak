package config

import (
	"strings"
	"testing"
)

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

	keys := []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_HOST", "POSTGRES_PORT", "HTTP_ADDR"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all keys first, so real env vars don't leak into the test.
			// t.Setenv restores the old values after the test.
			for _, k := range keys {
				t.Setenv(k, "")
			}
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
