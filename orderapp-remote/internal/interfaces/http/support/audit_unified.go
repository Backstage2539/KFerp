package support

import (
	"context"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditMeta = postgresinfra.AuditMeta

type AuditLogRow struct {
	Ts          string  `json:"ts"`
	Actor       string  `json:"actor"`
	Menu        string  `json:"menu"`
	Feature     string  `json:"feature"`
	Summary     string  `json:"summary"`
	EntityType  string  `json:"entity_type"`
	EntityID    *int64  `json:"entity_id"`
	EntityLabel *string `json:"entity_label"`
	EntityURL   *string `json:"entity_url"`
	Action      string  `json:"action"`
	Field       *string `json:"field"`
	OldValue    *string `json:"old_value"`
	NewValue    *string `json:"new_value"`
	Meta        *string `json:"meta"`
}

type AuditExecer = postgresinfra.AuditExecer
type AuditEntry = postgresinfra.AuditEntry
type AuditService = postgresinfra.AuditService

var NewAuditService = postgresinfra.NewAuditService

func AuditInsert(ctx context.Context, pool *pgxpool.Pool, schema string, actor, entityType string, entityID *int64, action string, field, oldv, newv *string, meta AuditMeta) {
	postgresinfra.AuditInsert(ctx, pool, schema, actor, entityType, entityID, action, field, oldv, newv, meta)
}

func AuditInsertTx(ctx context.Context, tx AuditExecer, schema string, actor, entityType string, entityID *int64, action string, field, oldv, newv *string, meta AuditMeta) error {
	return postgresinfra.AuditInsertTx(ctx, tx, schema, actor, entityType, entityID, action, field, oldv, newv, meta)
}
