package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditMeta map[string]any

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

func auditInsert(ctx context.Context, pool *pgxpool.Pool, schema string, actor, entityType string, entityID *int64, action string, field, oldv, newv *string, meta AuditMeta) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "unknown"
	}
	entityType = strings.TrimSpace(entityType)
	if entityType == "" {
		entityType = "unknown"
	}
	action = strings.TrimSpace(action)
	if action == "" {
		action = "unknown"
	}

	var metaJSON any = nil
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			metaJSON = b
		}
	}

	q := fmt.Sprintf(`INSERT INTO %s.audit_logs(actor, entity_type, entity_id, action, field, old_value, new_value, meta)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, schema)
	// best-effort: ignore error
	_, _ = pool.Exec(ctx, q, actor, entityType, entityID, action, field, oldv, newv, metaJSON)
}
