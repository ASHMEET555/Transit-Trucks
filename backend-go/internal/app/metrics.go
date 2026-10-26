package app

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
)

func (a *App) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Internal-only: require Bearer token. If missing or invalid, return 404 per spec
	token := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}

	expectedToken := a.cfg.MetricsToken
	if expectedToken == "" && a.cfg.Env != "production" {
		expectedToken = "transitops-metrics-dev"
	}

	if token == "" || expectedToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		w.Header().Set("Content-Type", "application/json")
		httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Path /metrics not found", nil)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("# TransitOps Prometheus Metrics\n"))
}
