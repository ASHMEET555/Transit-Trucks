package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidSession = errors.New("invalid session token")
	ErrExpiredSession = errors.New("session has expired")
	ErrUserInactive   = errors.New("user account is inactive")
)

const (
	SessionCookieName = "transitops_session"
	SlidingDuration   = 8 * time.Hour
	AbsoluteDuration  = 7 * 24 * time.Hour
)

type SessionUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	IsActive      bool   `json:"is_active"`
	DriverLicense string `json:"driver_license,omitempty"`
}

func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func GenerateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateSession(ctx context.Context, db *pgxpool.Pool, email, ip, userAgent string) (string, time.Time, error) {
	rawToken, err := GenerateRawToken()
	if err != nil {
		return "", time.Time{}, err
	}
	tokenHash := HashToken(rawToken)

	now := time.Now().UTC()
	expiresAt := now.Add(SlidingDuration)
	absExpiresAt := now.Add(AbsoluteDuration)

	query := `INSERT INTO sessions (token_hash, user_email, created_at, expires_at, absolute_expires_at, last_seen_at, ip, user_agent)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = db.Exec(ctx, query, tokenHash, email, now, expiresAt, absExpiresAt, now, ip, userAgent)
	if err != nil {
		return "", time.Time{}, err
	}

	return rawToken, expiresAt, nil
}

func ValidateSession(ctx context.Context, db *pgxpool.Pool, rawToken string) (*SessionUser, error) {
	if rawToken == "" {
		return nil, ErrInvalidSession
	}
	tokenHash := HashToken(rawToken)

	now := time.Now().UTC()
	var (
		u            SessionUser
		expiresAt    time.Time
		absExpiresAt time.Time
	)

	query := `SELECT s.token_hash, u.id, u.email, u.role, u.is_active, COALESCE(u.driver_license, ''), s.expires_at, s.absolute_expires_at
	          FROM sessions s
	          JOIN users u ON s.user_email = u.email
	          WHERE s.token_hash = $1`

	var returnedHash string
	err := db.QueryRow(ctx, query, tokenHash).Scan(
		&returnedHash,
		&u.ID,
		&u.Email,
		&u.Role,
		&u.IsActive,
		&u.DriverLicense,
		&expiresAt,
		&absExpiresAt,
	)
	if err != nil {
		return nil, ErrInvalidSession
	}

	if !u.IsActive {
		return nil, ErrUserInactive
	}

	if now.After(expiresAt) || now.After(absExpiresAt) {
		_, _ = db.Exec(ctx, "DELETE FROM sessions WHERE token_hash = $1", tokenHash)
		return nil, ErrExpiredSession
	}

	// 8-hour sliding expiry bounded by 7-day absolute
	newExpiresAt := now.Add(SlidingDuration)
	if newExpiresAt.After(absExpiresAt) {
		newExpiresAt = absExpiresAt
	}

	_, _ = db.Exec(ctx, "UPDATE sessions SET expires_at = $1, last_seen_at = $2 WHERE token_hash = $3", newExpiresAt, now, tokenHash)

	return &u, nil
}

func RevokeSession(ctx context.Context, db *pgxpool.Pool, rawToken string) error {
	tokenHash := HashToken(rawToken)
	_, err := db.Exec(ctx, "DELETE FROM sessions WHERE token_hash = $1", tokenHash)
	return err
}

func RevokeOtherSessions(ctx context.Context, db *pgxpool.Pool, userEmail, currentRawToken string) error {
	tokenHash := HashToken(currentRawToken)
	_, err := db.Exec(ctx, "DELETE FROM sessions WHERE user_email = $1 AND token_hash != $2", userEmail, tokenHash)
	return err
}

func CheckLockout(ctx context.Context, db *pgxpool.Pool, email, ip string) (bool, int, error) {
	window := time.Now().UTC().Add(-15 * time.Minute)

	var emailFailures int
	err := db.QueryRow(ctx, `SELECT COUNT(*) FROM login_attempts WHERE email = $1 AND success = false AND attempted_at > $2`, email, window).Scan(&emailFailures)
	if err != nil {
		return false, 0, err
	}

	var ipFailures int
	err = db.QueryRow(ctx, `SELECT COUNT(*) FROM login_attempts WHERE ip = $1 AND success = false AND attempted_at > $2`, ip, window).Scan(&ipFailures)
	if err != nil {
		return false, 0, err
	}

	if emailFailures >= 5 || ipFailures >= 20 {
		return true, 900, nil
	}

	return false, 0, nil
}

func RecordLoginAttempt(ctx context.Context, db *pgxpool.Pool, email, ip string, success bool) error {
	_, err := db.Exec(ctx, `INSERT INTO login_attempts (email, ip, success, attempted_at) VALUES ($1, $2, $3, $4)`,
		email, ip, success, time.Now().UTC())
	if err != nil {
		return err
	}

	if success {
		// Reset email failure count on successful login
		_, _ = db.Exec(ctx, `DELETE FROM login_attempts WHERE email = $1`, email)
	}
	return nil
}

func SetSessionCookie(w http.ResponseWriter, rawToken string, expiresAt time.Time, isLocal bool, domain string) {
	cookieDomain := domain
	if isLocal || domain == "localhost" || domain == "127.0.0.1" {
		cookieDomain = ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		Domain:   cookieDomain,
		Expires:  expiresAt,
		MaxAge:   int(SlidingDuration.Seconds()),
		HttpOnly: true,
		Secure:   !isLocal,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter, isLocal bool, domain string) {
	cookieDomain := domain
	if isLocal || domain == "localhost" || domain == "127.0.0.1" {
		cookieDomain = ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Domain:   cookieDomain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !isLocal,
		SameSite: http.SameSiteLaxMode,
	})
}

func GetSessionToken(r *http.Request) string {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	authHdr := r.Header.Get("Authorization")
	if strings.HasPrefix(authHdr, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHdr, "Bearer "))
	}
	return ""
}
