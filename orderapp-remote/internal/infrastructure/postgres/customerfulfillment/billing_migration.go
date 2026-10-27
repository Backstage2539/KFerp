package customerfulfillment

import (
	"context"
	"fmt"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

// ReleaseLegacyBillingDrafts retires customer monthly-settlement drafts from
// the old billing flow while preserving their headers and line snapshots.
func (r *Repository) ReleaseLegacyBillingDrafts(ctx context.Context) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.schema+":customer-billing-draft-release"); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,customer_id,settlement_no,round(total_amount*100)::bigint
		FROM %s.customer_settlement_batches
		WHERE status='draft' AND COALESCE(processing_billing_run_id,0)=0
		ORDER BY id
		FOR UPDATE
	`, r.schema))
	if err != nil {
		return err
	}
	type draft struct {
		id, customerID, totalCents int64
		settlementNo               string
	}
	var drafts []draft
	for rows.Next() {
		var item draft
		if err := rows.Scan(&item.id, &item.customerID, &item.settlementNo, &item.totalCents); err != nil {
			rows.Close()
			return err
		}
		drafts = append(drafts, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, item := range drafts {
		var lineCount int
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.customer_settlement_lines WHERE settlement_batch_id=$1 AND customer_id=$2 AND reversed_at IS NULL`, r.schema), item.id, item.customerID).Scan(&lineCount); err != nil {
			return err
		}
		feeResult, execErr := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_fee_items
			SET settlement_batch_id=0,status=CASE WHEN status='settled' THEN 'unsettled' ELSE status END
			WHERE customer_id=$1 AND settlement_batch_id=$2
		`, r.schema), item.customerID, item.id)
		if execErr != nil {
			return execErr
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_settlement_lines
			SET reversed_at=COALESCE(reversed_at,now()),settlement_status='draft_archived'
			WHERE customer_id=$1 AND settlement_batch_id=$2 AND reversed_at IS NULL
		`, r.schema), item.customerID, item.id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_batches SET status='reversed' WHERE id=$1 AND customer_id=$2 AND status='draft'`, r.schema), item.id, item.customerID); err != nil {
			return err
		}
		entityID := item.id
		if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, "system:migration:pr740", "customer_fulfillment_settlement", &entityID, "legacy_draft_release", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("draft"), postgresinfra.StrPtr("reversed"), postgresinfra.AuditMeta{
			"customer_id":            item.customerID,
			"settlement_no":          item.settlementNo,
			"total_amount_cents":     item.totalCents,
			"archived_line_count":    lineCount,
			"released_fee_count":     feeResult.RowsAffected(),
			"processing_run_skipped": false,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
