package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func (r Repository) GetEmployeeOrderDraft(ctx context.Context, employeeID int64) (*salesapp.EmployeeOrderDraft, error) {
	var draft salesapp.EmployeeOrderDraft
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, employee_id, payload, updated_at
		FROM %s.employee_order_drafts
		WHERE employee_id=$1
	`, r.schema), employeeID).Scan(&draft.ID, &draft.EmployeeID, &draft.Payload, &draft.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	draft.Payload = append(json.RawMessage(nil), draft.Payload...)
	return &draft, nil
}

func (r Repository) SaveEmployeeOrderDraft(ctx context.Context, cmd salesapp.SaveEmployeeOrderDraftCommand) (salesapp.EmployeeOrderDraft, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return salesapp.EmployeeOrderDraft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize first-save races as well as updates so one employee always has
	// exactly one server draft and one matching audit entry.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, cmd.EmployeeID); err != nil {
		return salesapp.EmployeeOrderDraft{}, err
	}

	var draft salesapp.EmployeeOrderDraft
	var oldPayload json.RawMessage
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, payload
		FROM %s.employee_order_drafts
		WHERE employee_id=$1
		FOR UPDATE
	`, r.schema), cmd.EmployeeID).Scan(&draft.ID, &oldPayload)
	action := "update"
	if err == pgx.ErrNoRows {
		action = "create"
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.employee_order_drafts(employee_id, payload, created_at, updated_at)
			VALUES($1, $2::jsonb, now(), now())
			RETURNING id, employee_id, payload, updated_at
		`, r.schema), cmd.EmployeeID, []byte(cmd.Payload)).Scan(&draft.ID, &draft.EmployeeID, &draft.Payload, &draft.UpdatedAt)
	} else if err == nil {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			UPDATE %s.employee_order_drafts
			SET payload=$2::jsonb, updated_at=now()
			WHERE id=$1
			RETURNING id, employee_id, payload, updated_at
		`, r.schema), draft.ID, []byte(cmd.Payload)).Scan(&draft.ID, &draft.EmployeeID, &draft.Payload, &draft.UpdatedAt)
	}
	if err != nil {
		return salesapp.EmployeeOrderDraft{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "employee_order_draft", &draft.ID, action, postgresinfra.StrPtr("payload"), nil, nil, postgresinfra.AuditMeta{
		"employee_id": cmd.EmployeeID,
	}); err != nil {
		return salesapp.EmployeeOrderDraft{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.EmployeeOrderDraft{}, err
	}
	draft.Payload = append(json.RawMessage(nil), draft.Payload...)
	return draft, nil
}

func (r Repository) DeleteEmployeeOrderDraft(ctx context.Context, employeeID int64, actor string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	deleted, err := deleteEmployeeOrderDraftTx(ctx, tx, r.schema, employeeID, actor, "manual")
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return deleted, nil
}

func deleteEmployeeOrderDraftTx(ctx context.Context, tx pgx.Tx, schema string, employeeID int64, actor, reason string) (bool, error) {
	if employeeID <= 0 {
		return false, nil
	}
	var draftID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		DELETE FROM %s.employee_order_drafts
		WHERE employee_id=$1
		RETURNING id
	`, schema), employeeID).Scan(&draftID)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	reason = strings.TrimSpace(reason)
	if err := postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "employee_order_draft", &draftID, "delete", nil, nil, nil, postgresinfra.AuditMeta{
		"employee_id": employeeID,
		"reason":      reason,
	}); err != nil {
		return false, err
	}
	return true, nil
}
