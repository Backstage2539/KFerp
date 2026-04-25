package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
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

type auditExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type AuditEntry struct {
	Actor      string
	EntityType string
	EntityID   *int64
	Action     string
	Field      *string
	OldValue   *string
	NewValue   *string
	Meta       AuditMeta
}

type AuditService struct {
	exec   auditExecer
	schema string
}

func NewAuditService(exec auditExecer, schema string) AuditService {
	return AuditService{exec: exec, schema: schema}
}

func (s AuditService) Insert(ctx context.Context, entry AuditEntry) error {
	entry.Actor = strings.TrimSpace(entry.Actor)
	if entry.Actor == "" {
		entry.Actor = "unknown"
	}
	entry.EntityType = strings.TrimSpace(entry.EntityType)
	if entry.EntityType == "" {
		entry.EntityType = "unknown"
	}
	entry.Action = strings.TrimSpace(entry.Action)
	if entry.Action == "" {
		entry.Action = "unknown"
	}

	var metaJSON any = nil
	if entry.Meta != nil {
		if b, err := json.Marshal(entry.Meta); err == nil {
			metaJSON = b
		}
	}

	q := fmt.Sprintf(`INSERT INTO %s.audit_logs(actor, entity_type, entity_id, action, field, old_value, new_value, meta)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, s.schema)
	_, err := s.exec.Exec(ctx, q, entry.Actor, entry.EntityType, entry.EntityID, entry.Action, entry.Field, entry.OldValue, entry.NewValue, metaJSON)
	return err
}

func auditInsert(ctx context.Context, pool *pgxpool.Pool, schema string, actor, entityType string, entityID *int64, action string, field, oldv, newv *string, meta AuditMeta) {
	// best-effort: ignore error
	_ = NewAuditService(pool, schema).Insert(ctx, AuditEntry{
		Actor:      actor,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Field:      field,
		OldValue:   oldv,
		NewValue:   newv,
		Meta:       meta,
	})
}

func auditInsertTx(ctx context.Context, tx auditExecer, schema string, actor, entityType string, entityID *int64, action string, field, oldv, newv *string, meta AuditMeta) error {
	return NewAuditService(tx, schema).Insert(ctx, AuditEntry{
		Actor:      actor,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Field:      field,
		OldValue:   oldv,
		NewValue:   newv,
		Meta:       meta,
	})
}
