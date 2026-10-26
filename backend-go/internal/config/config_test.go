// config_test.go
// Tests for the configuration loader and validator.
// These tests ensure that missing or invalid environment variables cause Load() to return an error,
// and that a fully valid configuration loads without error.

package config

import (
    "os"
    "testing"
)

// withEnv sets the provided environment variables for the duration of fn, restoring previous values afterwards.
func withEnv(env map[string]string, fn func()) {
    // Save original values.
    orig := make(map[string]string)
    for k := range env {
        orig[k] = os.Getenv(k)
    }
    // Apply new values.
    for k, v := range env {
        os.Setenv(k, v)
    }
    fn()
    // Restore originals.
    for k, v := range orig {
        os.Setenv(k, v)
    }
}

func TestLoad_ConfigValidation(t *testing.T) {
    // A minimal valid configuration.
    valid := map[string]string{
        "DATABASE_URL":    "postgres://user:pass@localhost:5432/db",
        "SESSION_SECRET":   "aVeryStrongSecretKeyWithAtLeast32Bytes!",
        "ENV":              "local",
        "ALLOWED_ORIGINS":  "http://localhost:3000",
        "COOKIE_DOMAIN":    "localhost",
    }

    tests := []struct {
        name    string
        env     map[string]string // overrides merged onto valid
        wantErr bool
    }{
        {"valid config", map[string]string{}, false},
        {"missing DATABASE_URL", map[string]string{"DATABASE_URL": "", "POSTGRES_DSN": ""}, true},
        {"missing SESSION_SECRET", map[string]string{"SESSION_SECRET": ""}, true},
        {"short SESSION_SECRET", map[string]string{"SESSION_SECRET": "short"}, true},
        {"weak SESSION_SECRET", map[string]string{"SESSION_SECRET": "default_secret_key"}, true},
        {"invalid ENV", map[string]string{"ENV": "staging"}, true},
        {"missing ALLOWED_ORIGINS", map[string]string{"ALLOWED_ORIGINS": ""}, true},
        {"empty ALLOWED_ORIGINS list", map[string]string{"ALLOWED_ORIGINS": "  ,   ,"}, true},
        {"missing COOKIE_DOMAIN", map[string]string{"COOKIE_DOMAIN": ""}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Merge overrides onto the valid baseline.
            env := make(map[string]string)
            for k, v := range valid {
                env[k] = v
            }
            for k, v := range tt.env {
                env[k] = v
            }
            withEnv(env, func() {
                _, err := Load()
                if (err != nil) != tt.wantErr {
                    t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
                }
            })
        })
    }
}
