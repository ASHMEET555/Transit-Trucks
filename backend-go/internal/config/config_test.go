package config

import (
	"os"
	"strings"
	"testing"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgresql://transitops:transitops@localhost:5432/transitops?sslmode=disable")
	t.Setenv("SESSION_SECRET", "k8n2f9a7d3h6j1m4p8r2t5w7y9b0c2e4g6")
	t.Setenv("ENV", "local")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")
	t.Setenv("COOKIE_DOMAIN", "localhost")
	t.Setenv("PORT", "8000")
}

func TestConfigLoadSuccess(t *testing.T) {
	setValidEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
	if cfg.Port != "8000" {
		t.Errorf("expected port 8000, got %s", cfg.Port)
	}
	if !cfg.IsLocal() {
		t.Errorf("expected IsLocal() to be true")
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("expected 2 allowed origins, got %d", len(cfg.AllowedOrigins))
	}
}

func TestConfigMissingDatabaseURL(t *testing.T) {
	setValidEnv(t)
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("POSTGRES_DSN")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("expected error for missing DATABASE_URL, got: %v", err)
	}
}

func TestConfigMissingSessionSecret(t *testing.T) {
	setValidEnv(t)
	os.Unsetenv("SESSION_SECRET")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SESSION_SECRET is required") {
		t.Fatalf("expected error for missing SESSION_SECRET, got: %v", err)
	}
}

func TestConfigShortSessionSecret(t *testing.T) {
	setValidEnv(t)
	t.Setenv("SESSION_SECRET", "too-short")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "must be at least 32 bytes") {
		t.Fatalf("expected error for short SESSION_SECRET, got: %v", err)
	}
}

func TestConfigKnownWeakSessionSecret(t *testing.T) {
	setValidEnv(t)
	t.Setenv("SESSION_SECRET", "12345678901234567890123456789012")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "known-weak list") {
		t.Fatalf("expected error for weak SESSION_SECRET, got: %v", err)
	}
}

func TestConfigRepetitiveSessionSecret(t *testing.T) {
	setValidEnv(t)
	t.Setenv("SESSION_SECRET", strings.Repeat("a", 32))
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "trivially repetitive") {
		t.Fatalf("expected error for repetitive SESSION_SECRET, got: %v", err)
	}
}

func TestConfigInvalidEnv(t *testing.T) {
	setValidEnv(t)
	t.Setenv("ENV", "staging")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ENV must be 'local' or 'production'") {
		t.Fatalf("expected error for invalid ENV, got: %v", err)
	}
}

func TestConfigMissingAllowedOrigins(t *testing.T) {
	setValidEnv(t)
	os.Unsetenv("ALLOWED_ORIGINS")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ALLOWED_ORIGINS is required") {
		t.Fatalf("expected error for missing ALLOWED_ORIGINS, got: %v", err)
	}
}

func TestConfigMissingCookieDomain(t *testing.T) {
	setValidEnv(t)
	os.Unsetenv("COOKIE_DOMAIN")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "COOKIE_DOMAIN is required") {
		t.Fatalf("expected error for missing COOKIE_DOMAIN, got: %v", err)
	}
}
