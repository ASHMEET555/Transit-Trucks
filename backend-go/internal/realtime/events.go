package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const ChannelFleetEvents = "fleet_events"
const MaxPayloadBytes = 7999

type Event struct {
	Type       string         `json:"type"`
	IDs        map[string]any `json:"ids"`
	OccurredAt string         `json:"occurred_at"`
}

// PublishTx emits an event through Postgres pg_notify inside the current transaction.
// The notification is only delivered if the transaction commits successfully.
func PublishTx(ctx context.Context, tx pgx.Tx, eventType string, ids map[string]any) error {
	if ids == nil {
		ids = make(map[string]any)
	}

	ev := Event{
		Type:       eventType,
		IDs:        ids,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	}

	payloadBytes, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal event %s: %w", eventType, err)
	}

	if len(payloadBytes) > MaxPayloadBytes {
		return fmt.Errorf("event %s payload exceeded %d bytes limit (was %d bytes)", eventType, MaxPayloadBytes, len(payloadBytes))
	}

	_, err = tx.Exec(ctx, "SELECT pg_notify($1, $2)", ChannelFleetEvents, string(payloadBytes))
	if err != nil {
		return fmt.Errorf("pg_notify failed for %s: %w", eventType, err)
	}

	return nil
}
