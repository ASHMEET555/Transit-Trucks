package app

import (
	"crypto/sha256"
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/middleware"
	"github.com/ASHMEET555/Transit-Trucks/backend-go/internal/platform/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type idempotencyRecord struct {
	StatusCode   int
	ResponseBody string
	Route        string
	RequestHash  string
	ExpiresAt    time.Time
}

// checkIdempotency validates the Idempotency-Key header, hashes the request body,
// and checks if a cached response exists to replay.
func (a *App) checkIdempotency(w http.ResponseWriter, r *http.Request, bodyBytes []byte) (*idempotencyRecord, string, string, bool) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		httpx.WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required.", nil)
		return nil, "", "", false
	}

	if _, err := uuid.Parse(key); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key must be a valid UUID.", nil)
		return nil, "", "", false
	}

	user := middleware.GetUser(r)
	userID := "anonymous"
	if user != nil {
		userID = user.ID
	}

	h := sha256.Sum256(bodyBytes)
	bodyHash := hex.EncodeToString(h[:])

	ctx := r.Context()
	var rec idempotencyRecord
	err := a.db.QueryRow(ctx, `SELECT status_code, response_body, route, request_hash, expires_at 
		FROM idempotency_keys WHERE user_id = $1 AND key = $2`, userID, key).
		Scan(&rec.StatusCode, &rec.ResponseBody, &rec.Route, &rec.RequestHash, &rec.ExpiresAt)

	if err == nil {
		// Existing record found
		if time.Now().After(rec.ExpiresAt) {
			// Expired: delete and let it execute fresh
			_, _ = a.db.Exec(ctx, `DELETE FROM idempotency_keys WHERE user_id = $1 AND key = $2`, userID, key)
			return nil, key, bodyHash, true
		}

		if rec.Route != r.URL.Path {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REUSED", "Idempotency key was previously used on a different route.", nil)
			return nil, "", "", false
		}

		if rec.RequestHash != bodyHash {
			httpx.WriteError(w, r, http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REUSED", "Idempotency key was previously used with a different request body.", nil)
			return nil, "", "", false
		}

		// Replay cached response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Idempotent-Replay", "true")
		w.WriteHeader(rec.StatusCode)
		_, _ = w.Write([]byte(rec.ResponseBody))
		return &rec, key, bodyHash, false
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		status, code, msg := httpx.MapDBError(err, nil)
		httpx.WriteError(w, r, status, code, msg, nil)
		return nil, "", "", false
	}

	return nil, key, bodyHash, true
}

func saveIdempotency(ctx context.Context, tx pgx.Tx, key, userID, route, requestHash string, statusCode int, responseBody string) error {
	_, err := tx.Exec(ctx, `INSERT INTO idempotency_keys (key, user_id, route, request_hash, status_code, response_body, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now() + INTERVAL '24 hours')`,
		key, userID, route, requestHash, statusCode, responseBody)
	return err
}
