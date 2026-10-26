package app

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/realtime"
	"github.com/jackc/pgx/v5"
)

type Notification struct {
	ID         int64          `json:"id"`
	UserID     *string        `json:"user_id"`
	RoleTarget string         `json:"role_target"`
	Type       string         `json:"type"`
	DedupeKey  string         `json:"dedupe_key"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  string         `json:"created_at"`
	ReadAt     *string        `json:"read_at"`
}

// POST /api/v1/internal/jobs/license-check
// Protected by X-Job-Secret header
func (a *App) handleLicenseCheckJob(w http.ResponseWriter, r *http.Request) {
	jobSecret := r.Header.Get("X-Job-Secret")
	expectedSecret := a.cfg.JobSecret
	if expectedSecret == "" && a.cfg.Env != "production" {
		expectedSecret = "transitops-job-secret-dev"
	}

	if jobSecret == "" || expectedSecret == "" || subtle.ConstantTimeCompare([]byte(jobSecret), []byte(expectedSecret)) != 1 {
		a.metrics.RecordJobResult(false)
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing job secret.", nil)
		return
	}

	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		a.metrics.RecordJobResult(false)
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to start job transaction.", nil)
		return
	}
	defer tx.Rollback(ctx)

	// Fetch active drivers and their license expiry date
	rows, err := tx.Query(ctx, `
		SELECT license_number, license_expiry_date, status, is_active
		FROM drivers
		WHERE is_active = true`)
	if err != nil {
		a.metrics.RecordJobResult(false)
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to query drivers.", nil)
		return
	}
	defer rows.Close()

	type driverInfo struct {
		lic    string
		expiry time.Time
		status string
		active bool
	}
	var drivers []driverInfo
	for rows.Next() {
		var d driverInfo
		if err := rows.Scan(&d.lic, &d.expiry, &d.status, &d.active); err == nil {
			drivers = append(drivers, d)
		}
	}
	rows.Close()

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	notificationsCreated := 0

	for _, d := range drivers {
		daysRemaining := int(d.expiry.Sub(today).Hours() / 24)
		expiryStr := d.expiry.Format("2006-01-02")

		var notifType, dedupeKey string
		var threshold int
		if daysRemaining <= 0 {
			notifType = "license_expired"
			dedupeKey = fmt.Sprintf("%s:%s:expired", d.lic, expiryStr)
			threshold = 0
		} else if daysRemaining <= 7 {
			notifType = "license_expiring_7d"
			dedupeKey = fmt.Sprintf("%s:%s:7d", d.lic, expiryStr)
			threshold = 7
		} else if daysRemaining <= 30 {
			notifType = "license_expiring_30d"
			dedupeKey = fmt.Sprintf("%s:%s:30d", d.lic, expiryStr)
			threshold = 30
		} else {
			continue
		}

		payloadBytes, _ := json.Marshal(map[string]any{
			"days_remaining": daysRemaining,
			"expiry_date":    expiryStr,
		})

		var insertedID int64
		err = tx.QueryRow(ctx, `
			INSERT INTO notifications (role_target, type, dedupe_key, payload, created_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (type, dedupe_key) DO NOTHING
			RETURNING id`,
			rbac.RoleSafetyOfficer, notifType, dedupeKey, payloadBytes).Scan(&insertedID)

		if err == nil && insertedID > 0 {
			notificationsCreated++
			_ = realtime.PublishTx(ctx, tx, realtime.EventNotificationCreated, map[string]any{
				"notification_id": insertedID,
				"type":            notifType,
				"threshold":       threshold,
			})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		a.metrics.RecordJobResult(false)
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to commit job transaction.", nil)
		return
	}

	a.metrics.RecordJobResult(true)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"drivers_checked":       len(drivers),
		"notifications_created": notificationsCreated,
	})
}

// GET /api/v1/notifications
// Roles: Safety Officer, Fleet Manager, Admin
func (a *App) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	unreadOnly := r.URL.Query().Get("unread_only") == "true"
	query := `
		SELECT id, user_id, role_target, type, dedupe_key, payload, created_at, read_at
		FROM notifications`
	if unreadOnly {
		query += ` WHERE read_at IS NULL`
	}
	query += ` ORDER BY read_at NULLS FIRST, created_at DESC, id DESC LIMIT $1`

	rows, err := a.db.Query(ctx, query, limit)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to query notifications.", nil)
		return
	}
	defer rows.Close()

	var items []Notification
	for rows.Next() {
		var n Notification
		var payloadJSON []byte
		var createdAt time.Time
		var readAt *time.Time

		if err := rows.Scan(&n.ID, &n.UserID, &n.RoleTarget, &n.Type, &n.DedupeKey, &payloadJSON, &createdAt, &readAt); err != nil {
			continue
		}
		_ = json.Unmarshal(payloadJSON, &n.Payload)
		n.CreatedAt = createdAt.Format(time.RFC3339)
		if readAt != nil {
			rAt := readAt.Format(time.RFC3339)
			n.ReadAt = &rAt
		}
		items = append(items, n)
	}

	if items == nil {
		items = []Notification{}
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nil,
	})
}

// POST /api/v1/notifications/{id}/read
// Roles: Safety Officer, Fleet Manager, Admin
func (a *App) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	idVal := r.PathValue("id")
	notifID, err := strconv.ParseInt(idVal, 10, 64)
	if err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "MALFORMED_INPUT", "Invalid notification id.", nil)
		return
	}

	ctx := r.Context()
	var readAt time.Time
	err = a.db.QueryRow(ctx, `
		UPDATE notifications
		SET read_at = COALESCE(read_at, now())
		WHERE id = $1
		RETURNING read_at`, notifID).Scan(&readAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Notification not found.", nil)
			return
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update notification.", nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":      notifID,
		"read_at": readAt.Format(time.RFC3339),
	})
}
