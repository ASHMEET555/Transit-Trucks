package app

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func recordAuditLogTx(ctx context.Context, tx pgx.Tx, entity, entityID, action, actorEmail string, details any) {
	var detailsBytes []byte
	if details != nil {
		detailsBytes, _ = json.Marshal(details)
	}
	_, _ = tx.Exec(ctx, `INSERT INTO audit_log (entity, entity_id, action, actor_email, details, at)
		VALUES ($1, $2, $3, $4, $5, now())`,
		entity, entityID, action, actorEmail, detailsBytes)
}
