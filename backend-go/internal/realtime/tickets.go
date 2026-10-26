package realtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidTicket = errors.New("invalid, expired, or previously consumed ticket")
)

// HashTicket returns the hex-encoded SHA-256 hash of an opaque ticket token.
func HashTicket(ticket string) string {
	sum := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(sum[:])
}

// IssueTicket generates a 32-byte cryptographically secure opaque token,
// stores its SHA-256 hash in realtime_tickets with a 30-second TTL, cleans up stale tickets,
// and returns the raw ticket string and TTL in seconds.
func IssueTicket(ctx context.Context, db *pgxpool.Pool, userID, role string) (string, int, error) {
	// Background cleanup of expired tickets (> 5 minutes ago)
	_, _ = db.Exec(ctx, `DELETE FROM realtime_tickets WHERE expires_at < now() - INTERVAL '5 minutes'`)

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", 0, fmt.Errorf("failed to generate random ticket bytes: %w", err)
	}

	rawTicket := base64.RawURLEncoding.EncodeToString(tokenBytes)
	tokenHash := HashTicket(rawTicket)

	_, err := db.Exec(ctx, `
		INSERT INTO realtime_tickets (token_hash, user_id, role, expires_at)
		VALUES ($1, $2, $3, now() + INTERVAL '30 seconds')`,
		tokenHash, userID, role)
	if err != nil {
		return "", 0, fmt.Errorf("failed to store ticket hash: %w", err)
	}

	return rawTicket, 30, nil
}

// ConsumeTicket atomically validates and claims a ticket in a single operation.
// It returns the user_id and role associated with the ticket, or ErrInvalidTicket
// if the ticket does not exist, has already been used, or has expired.
func ConsumeTicket(ctx context.Context, db *pgxpool.Pool, rawTicket string) (string, string, error) {
	if rawTicket == "" {
		return "", "", ErrInvalidTicket
	}

	tokenHash := HashTicket(rawTicket)

	var userID, role string
	err := db.QueryRow(ctx, `
		UPDATE realtime_tickets
		SET used_at = now()
		WHERE token_hash = $1
		  AND used_at IS NULL
		  AND expires_at > now()
		RETURNING user_id, role`,
		tokenHash).Scan(&userID, &role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrInvalidTicket
		}
		return "", "", fmt.Errorf("failed to consume ticket: %w", err)
	}

	return userID, role, nil
}
