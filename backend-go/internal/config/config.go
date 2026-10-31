package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

var knownWeakSecrets = map[string]bool{
	"default_secret_key":                 true,
	"12345678901234567890123456789012": true,
	"abcdefghijklmnopqrstuvwxyz012345": true,
	"0123456789abcdef0123456789abcdef": true,
	"thisisnotasecretkeyatallpleasedont": true,
	"session_secret_session_secret_32":  true,
	"passwordpasswordpasswordpassword":   true,
	"supersecretkeythatisatleast32bytes": true,
	"change-me-change-me-change-me-32":  true,
}

type Config struct {
	DatabaseURL    string
	SessionSecret  string
	Env            string
	AllowedOrigins []string
	CookieDomain   string
	Port              string
	MetricsToken      string
	JobSecret         string
	TrustedProxyCIDRs []*net.IPNet
	// Live-tracking / simulator settings. All optional, no required validation.
	SimEnabled   bool
	OSRMURL      string
	SimTimeScale float64
	SimSeed      int64
}

func (c *Config) IsLocal() bool {
	return c.Env == "local"
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("POSTGRES_DSN")
	}
	if strings.TrimSpace(dbURL) == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return nil, errors.New("SESSION_SECRET is required (no defaults permitted)")
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 bytes, got %d", len(secret))
	}
	lowerSecret := strings.ToLower(secret)
	if knownWeakSecrets[lowerSecret] || isRepetitive(secret) {
		return nil, errors.New("SESSION_SECRET is on the known-weak list or trivially repetitive")
	}

	envVal := os.Getenv("ENV")
	if envVal != "local" && envVal != "production" {
		return nil, fmt.Errorf("ENV must be 'local' or 'production', got %q", envVal)
	}

	originsRaw := os.Getenv("ALLOWED_ORIGINS")
	if strings.TrimSpace(originsRaw) == "" {
		return nil, errors.New("ALLOWED_ORIGINS is required")
	}
	var origins []string
	for _, part := range strings.Split(originsRaw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		return nil, errors.New("ALLOWED_ORIGINS must contain at least one origin")
	}

	cookieDomain := os.Getenv("COOKIE_DOMAIN")
	if strings.TrimSpace(cookieDomain) == "" {
		return nil, errors.New("COOKIE_DOMAIN is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	metricsToken := os.Getenv("METRICS_TOKEN")
	jobSecret := os.Getenv("JOB_SECRET")

	if envVal == "production" {
		if strings.TrimSpace(metricsToken) == "" || metricsToken == "transitops-metrics-dev" {
			return nil, errors.New("METRICS_TOKEN is required in production and must not use dev default")
		}
		if strings.TrimSpace(jobSecret) == "" || jobSecret == "transitops-job-secret-dev" {
			return nil, errors.New("JOB_SECRET is required in production and must not use dev default")
		}
	}

	trustedProxiesRaw := os.Getenv("TRUSTED_PROXY_CIDRS")
	var trustedCIDRs []*net.IPNet
	if strings.TrimSpace(trustedProxiesRaw) != "" {
		for _, part := range strings.Split(trustedProxiesRaw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			if !strings.Contains(trimmed, "/") {
				trimmed += "/32"
			}
			_, ipNet, err := net.ParseCIDR(trimmed)
			if err != nil {
				return nil, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS entry %q: %w", trimmed, err)
			}
			trustedCIDRs = append(trustedCIDRs, ipNet)
		}
	}

	return &Config{
		DatabaseURL:       dbURL,
		SessionSecret:     secret,
		Env:               envVal,
		AllowedOrigins:    origins,
		CookieDomain:      cookieDomain,
		Port:              port,
		MetricsToken:      metricsToken,
		JobSecret:         jobSecret,
		TrustedProxyCIDRs: trustedCIDRs,
		SimEnabled:        os.Getenv("SIM_ENABLED") == "1",
		OSRMURL:           firstNonEmpty(os.Getenv("OSRM_URL"), "http://localhost:5000"),
		SimTimeScale:      parseTimeScale(os.Getenv("SIM_TIME_SCALE")),
		SimSeed:           parseSeed(os.Getenv("SIM_SEED")),
	}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// parseTimeScale parses SIM_TIME_SCALE, defaulting to 20 on empty/invalid.
func parseTimeScale(raw string) float64 {
	if strings.TrimSpace(raw) == "" {
		return 20
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil && v > 0 {
		return v
	}
	return 20
}

// parseSeed parses SIM_SEED, defaulting to 0 on empty/invalid.
func parseSeed(raw string) int64 {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	if v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil {
		return v
	}
	return 0
}

func isRepetitive(s string) bool {
	if len(s) == 0 {
		return true
	}
	first := s[0]
	for i := 1; i < len(s); i++ {
		if s[i] != first {
			return false
		}
	}
	return true
}
