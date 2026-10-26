package app

import (
	"crypto/subtle"
	"net/http"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
)

func (a *App) handleLicenseCheckJob(w http.ResponseWriter, r *http.Request) {
	jobSecret := r.Header.Get("X-Job-Secret")
	expectedSecret := a.cfg.JobSecret
	if expectedSecret == "" && a.cfg.Env != "production" {
		expectedSecret = "transitops-job-secret-dev"
	}

	if jobSecret == "" || expectedSecret == "" || subtle.ConstantTimeCompare([]byte(jobSecret), []byte(expectedSecret)) != 1 {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing job secret.", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok", "notifications_created": 0})
}

func (a *App) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": []any{}})
}

func (a *App) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "read"})
}
