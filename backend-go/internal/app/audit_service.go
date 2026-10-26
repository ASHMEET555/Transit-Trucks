package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/rbac"
)

type AuditEntry struct {
	ID         int64          `json:"id"`
	Entity     string         `json:"entity"`
	EntityID   string         `json:"entity_id"`
	Action     string         `json:"action"`
	ActorEmail *string        `json:"actor_email,omitempty"`
	Actor      *string        `json:"actor,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
	At         string         `json:"at"`
}

// GET /api/v1/audit
func (a *App) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required.", nil)
		return
	}
	if user.Role != rbac.RoleAdmin {
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Only administrators may view the audit log.", nil)
		return
	}

	q := r.URL.Query()
	entity := q.Get("entity")
	entityID := q.Get("entity_id")
	actor := q.Get("actor")
	if actor == "" {
		actor = q.Get("actor_user_id")
	}
	if actor == "" {
		actor = q.Get("actor_email")
	}
	fromDate := q.Get("from")
	toDate := q.Get("to")
	cursorStr := q.Get("cursor")

	limit := 20
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}
	}

	var conditions []string
	var args []any
	argIdx := 1

	if entity != "" {
		conditions = append(conditions, fmt.Sprintf("entity = $%d", argIdx))
		args = append(args, entity)
		argIdx++
	}
	if entityID != "" {
		conditions = append(conditions, fmt.Sprintf("entity_id = $%d", argIdx))
		args = append(args, entityID)
		argIdx++
	}
	if actor != "" {
		conditions = append(conditions, fmt.Sprintf("actor_email = $%d", argIdx))
		args = append(args, actor)
		argIdx++
	}
	if fromDate != "" {
		if _, err := time.Parse("2006-01-02", fromDate); err == nil {
			conditions = append(conditions, fmt.Sprintf("at >= $%d::DATE", argIdx))
		} else {
			conditions = append(conditions, fmt.Sprintf("at >= $%d::TIMESTAMPTZ", argIdx))
		}
		args = append(args, fromDate)
		argIdx++
	}
	if toDate != "" {
		if _, err := time.Parse("2006-01-02", toDate); err == nil {
			conditions = append(conditions, fmt.Sprintf("at <= ($%d::DATE + INTERVAL '1 day' - INTERVAL '1 microsecond')", argIdx))
		} else {
			conditions = append(conditions, fmt.Sprintf("at <= $%d::TIMESTAMPTZ", argIdx))
		}
		args = append(args, toDate)
		argIdx++
	}
	if cursorStr != "" {
		if cursorID, err := strconv.ParseInt(cursorStr, 10, 64); err == nil {
			conditions = append(conditions, fmt.Sprintf("id < $%d", argIdx))
			args = append(args, cursorID)
			argIdx++
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, entity, entity_id, action, actor_email, details, at
		FROM audit_log
		%s
		ORDER BY at DESC, id DESC
		LIMIT $%d`, whereClause, argIdx)
	args = append(args, limit+1)

	ctx := r.Context()
	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		httpx.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to query audit logs.", nil)
		return
	}
	defer rows.Close()

	var items []AuditEntry
	for rows.Next() {
		var entry AuditEntry
		var actorEmail *string
		var detailsBytes []byte
		var at time.Time
		if err := rows.Scan(&entry.ID, &entry.Entity, &entry.EntityID, &entry.Action, &actorEmail, &detailsBytes, &at); err != nil {
			continue
		}
		entry.ActorEmail = actorEmail
		entry.Actor = actorEmail
		entry.At = at.Format(time.RFC3339)
		if len(detailsBytes) > 0 {
			var d map[string]any
			if err := json.Unmarshal(detailsBytes, &d); err == nil {
				entry.Details = d
			}
		}
		items = append(items, entry)
	}

	var nextCursor *string
	if len(items) > limit {
		lastItem := items[limit-1]
		c := fmt.Sprintf("%d", lastItem.ID)
		nextCursor = &c
		items = items[:limit]
	}

	httpx.WriteJSON(w, http.StatusOK, PaginatedResponse[AuditEntry]{
		Items:      items,
		NextCursor: nextCursor,
	})
}
