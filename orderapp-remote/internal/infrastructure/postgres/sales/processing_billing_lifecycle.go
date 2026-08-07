package sales

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

type processingBillingRunRecord struct {
	ID                int64
	CustomerID        int64
	TemplateID        int64
	TemplateVersionID int64
	SettlementBatchID int64
	SettlementNo      string
	RunKind           string
	SourceRunID       int64
	Status            string
	TotalAmount       float64
	Currency          string
}

func (r Repository) ListProcessingBillingRuns(ctx context.Context, query salesapp.ProcessingBillingRunsQuery) ([]salesapp.ProcessingBillingRun, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT r.id,r.customer_id,COALESCE(c.name,''),r.template_id,r.template_version_id,
		       r.settlement_batch_id,COALESCE(s.settlement_no,''),COALESCE(r.run_kind,'standard'),
		       COALESCE(r.source_billing_run_id,0),COALESCE(r.status,''),COALESCE(r.total_amount,0)::float8,
		       COALESCE(NULLIF(r.currency,''),'CNY'),COUNT(DISTINCT bwo.work_order_id)::int,
		       COALESCE(to_char(r.confirmed_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(r.paid_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(r.reversed_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(r.lifecycle_reason,'')
		FROM %s.processing_billing_runs r
		JOIN %s.customer_settlement_batches s ON s.id=r.settlement_batch_id
		LEFT JOIN %s.customers c ON c.id=r.customer_id
		LEFT JOIN %s.processing_billing_work_orders bwo ON bwo.billing_run_id=r.id
		WHERE r.customer_id=$1
		GROUP BY r.id,r.customer_id,c.name,r.template_id,r.template_version_id,r.settlement_batch_id,
		         s.settlement_no,r.run_kind,r.source_billing_run_id,r.status,r.total_amount,r.currency,
		         r.confirmed_at,r.paid_at,r.reversed_at,r.lifecycle_reason
		ORDER BY r.confirmed_at DESC,r.id DESC
		LIMIT $2
	`, r.schema, r.schema, r.schema, r.schema), query.CustomerID, query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]salesapp.ProcessingBillingRun, 0)
	for rows.Next() {
		var row salesapp.ProcessingBillingRun
		if err := rows.Scan(
			&row.ID, &row.CustomerID, &row.CustomerName, &row.TemplateID, &row.TemplateVersionID,
			&row.SettlementBatchID, &row.SettlementNo, &row.RunKind, &row.SourceBillingRunID,
			&row.Status, &row.TotalAmount, &row.Currency, &row.WorkOrderCount,
			&row.ConfirmedAt, &row.PaidAt, &row.ReversedAt, &row.Reason,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) PayProcessingBilling(ctx context.Context, cmd salesapp.PayProcessingBillingCommand) (salesapp.ProcessingBillingLifecycleResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	record, err := r.lockProcessingBillingRunTx(ctx, tx, cmd.BillingRunID)
	if err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if record.Status == salesapp.ProcessingBillingStatusPaid {
		result := processingBillingLifecycleResult(record)
		result.Reused = true
		if err := tx.Commit(ctx); err != nil {
			return salesapp.ProcessingBillingLifecycleResult{}, err
		}
		return result, nil
	}
	if record.Status == salesapp.ProcessingBillingStatusReversed {
		return salesapp.ProcessingBillingLifecycleResult{}, fmt.Errorf("reversed bill cannot be paid")
	}
	if record.Status != salesapp.ProcessingBillingStatusConfirmed {
		return salesapp.ProcessingBillingLifecycleResult{}, fmt.Errorf("billing status cannot be paid: %s", record.Status)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.processing_billing_runs
		SET status='paid',paid_at=now(),lifecycle_reason=CASE WHEN $2='' THEN lifecycle_reason ELSE $2 END
		WHERE id=$1
	`, r.schema), record.ID, cmd.Note); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_settlement_batches SET status='paid',paid_at=now() WHERE id=$1
	`, r.schema), record.SettlementBatchID); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_processing_bill", &record.ID, "pay", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(record.Status), postgresinfra.StrPtr(salesapp.ProcessingBillingStatusPaid), postgresinfra.AuditMeta{
		"customer_id": record.CustomerID, "settlement_batch_id": record.SettlementBatchID, "settlement_no": record.SettlementNo, "note": cmd.Note,
	}); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	record.Status = salesapp.ProcessingBillingStatusPaid
	return processingBillingLifecycleResult(record), nil
}

func (r Repository) ReverseProcessingBilling(ctx context.Context, cmd salesapp.ReverseProcessingBillingCommand) (salesapp.ProcessingBillingLifecycleResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	source, err := r.lockProcessingBillingRunTx(ctx, tx, cmd.BillingRunID)
	if err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if source.RunKind == salesapp.ProcessingBillingRunKindReversal {
		return salesapp.ProcessingBillingLifecycleResult{}, fmt.Errorf("reversal bill cannot be reversed")
	}
	if existing, found, err := r.linkedProcessingBillingResultTx(ctx, tx, source.ID, salesapp.ProcessingBillingRunKindReversal, ""); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	} else if found {
		existing.Reused = true
		if err := tx.Commit(ctx); err != nil {
			return salesapp.ProcessingBillingLifecycleResult{}, err
		}
		return existing, nil
	}
	if source.Status != salesapp.ProcessingBillingStatusConfirmed && source.Status != salesapp.ProcessingBillingStatusPaid {
		return salesapp.ProcessingBillingLifecycleResult{}, fmt.Errorf("billing status cannot be reversed: %s", source.Status)
	}
	reversal, err := r.createLinkedProcessingBillingRunTx(ctx, tx, source, salesapp.ProcessingBillingRunKindReversal, -source.TotalAmount, cmd.Reason, "", cmd.Actor)
	if err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if err := r.copyProcessingBillingWorkOrdersTx(ctx, tx, source.ID, reversal.BillingRunID, salesapp.ProcessingBillingRunKindReversal); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if err := r.copyProcessingBillingReversalLinesTx(ctx, tx, source, reversal, cmd); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.processing_billing_runs SET status='reversed',reversed_at=now(),lifecycle_reason=$2 WHERE id=$1
	`, r.schema), source.ID, cmd.Reason); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_batches SET status='reversed' WHERE id=$1`, r.schema), source.SettlementBatchID); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_processing_bill", &source.ID, "reverse", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(source.Status), postgresinfra.StrPtr(salesapp.ProcessingBillingStatusReversed), postgresinfra.AuditMeta{
		"customer_id": source.CustomerID, "settlement_batch_id": source.SettlementBatchID, "reversal_billing_run_id": reversal.BillingRunID, "reason": cmd.Reason,
	}); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	return reversal, nil
}

func (r Repository) AdjustProcessingBilling(ctx context.Context, cmd salesapp.AdjustProcessingBillingCommand) (salesapp.ProcessingBillingLifecycleResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	source, err := r.lockProcessingBillingRunTx(ctx, tx, cmd.BillingRunID)
	if err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if source.Status == salesapp.ProcessingBillingStatusReversed || source.RunKind == salesapp.ProcessingBillingRunKindReversal {
		return salesapp.ProcessingBillingLifecycleResult{}, fmt.Errorf("reversed bill cannot be adjusted")
	}
	if source.Status != salesapp.ProcessingBillingStatusConfirmed && source.Status != salesapp.ProcessingBillingStatusPaid {
		return salesapp.ProcessingBillingLifecycleResult{}, fmt.Errorf("billing status cannot be adjusted: %s", source.Status)
	}
	if existing, found, err := r.linkedProcessingBillingResultTx(ctx, tx, source.ID, salesapp.ProcessingBillingRunKindAdjustment, cmd.RequestKey); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	} else if found {
		existing.Reused = true
		if err := tx.Commit(ctx); err != nil {
			return salesapp.ProcessingBillingLifecycleResult{}, err
		}
		return existing, nil
	}
	if err := r.validateProcessingBillingAdjustmentWorkOrdersTx(ctx, tx, source.ID, cmd.Lines); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	total := 0.0
	for _, line := range cmd.Lines {
		total = roundProcessingBillingMoney(total + line.Amount)
	}
	adjustment, err := r.createLinkedProcessingBillingRunTx(ctx, tx, source, salesapp.ProcessingBillingRunKindAdjustment, total, cmd.Reason, cmd.RequestKey, cmd.Actor)
	if err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if err := r.copyProcessingBillingWorkOrdersTx(ctx, tx, source.ID, adjustment.BillingRunID, salesapp.ProcessingBillingRunKindAdjustment); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	for _, line := range cmd.Lines {
		var feeItemID int64
		sourceID := line.WorkOrderID
		if sourceID <= 0 {
			sourceID = source.ID
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_fee_items(
				customer_id,source_type,source_id,fee_type,amount,currency,occurred_at,settlement_batch_id,status,note,processing_billing_run_id
			) VALUES($1,'processing_billing_adjustment',$2,$3,$4,$5,now(),$6,'settled',$7,$8)
			RETURNING id
		`, r.schema), source.CustomerID, sourceID, line.FeeType, line.Amount, source.Currency, adjustment.SettlementBatchID,
			strings.TrimSpace(line.FeeName+" / "+cmd.Reason), adjustment.BillingRunID).Scan(&feeItemID); err != nil {
			return salesapp.ProcessingBillingLifecycleResult{}, err
		}
		calculationJSON, err := json.Marshal(map[string]any{
			"source_billing_run_id": source.ID, "reason": cmd.Reason, "manual_adjustment": true,
		})
		if err != nil {
			return salesapp.ProcessingBillingLifecycleResult{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.processing_billing_line_snapshots(
				billing_run_id,work_order_id,rule_id,line_kind,source_line_id,fee_item_id,fee_type,fee_name,
				basis,base_quantity,unit_price,amount,reason,calculation_json
			) VALUES($1,$2,NULL,'adjustment',0,$3,$4,$5,'manual_adjustment',1,$6,$6,$7,$8::jsonb)
		`, r.schema), adjustment.BillingRunID, line.WorkOrderID, feeItemID, line.FeeType, line.FeeName, line.Amount, cmd.Reason, calculationJSON); err != nil {
			return salesapp.ProcessingBillingLifecycleResult{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_processing_bill", &source.ID, "adjust", nil, nil, nil, postgresinfra.AuditMeta{
		"customer_id": source.CustomerID, "adjustment_billing_run_id": adjustment.BillingRunID, "reason": cmd.Reason, "amount": total, "request_key": cmd.RequestKey,
	}); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	return adjustment, nil
}

func (r Repository) lockProcessingBillingRunTx(ctx context.Context, tx pgx.Tx, billingRunID int64) (processingBillingRunRecord, error) {
	var row processingBillingRunRecord
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT r.id,r.customer_id,r.template_id,r.template_version_id,r.settlement_batch_id,
		       COALESCE(s.settlement_no,''),COALESCE(r.run_kind,'standard'),COALESCE(r.source_billing_run_id,0),
		       COALESCE(r.status,''),COALESCE(r.total_amount,0)::float8,COALESCE(NULLIF(r.currency,''),'CNY')
		FROM %s.processing_billing_runs r
		JOIN %s.customer_settlement_batches s ON s.id=r.settlement_batch_id
		WHERE r.id=$1
		FOR UPDATE OF r,s
	`, r.schema, r.schema), billingRunID).Scan(
		&row.ID, &row.CustomerID, &row.TemplateID, &row.TemplateVersionID, &row.SettlementBatchID,
		&row.SettlementNo, &row.RunKind, &row.SourceRunID, &row.Status, &row.TotalAmount, &row.Currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return processingBillingRunRecord{}, fmt.Errorf("processing bill not found")
	}
	return row, err
}

func (r Repository) linkedProcessingBillingResultTx(ctx context.Context, tx pgx.Tx, sourceRunID int64, runKind, requestKey string) (salesapp.ProcessingBillingLifecycleResult, bool, error) {
	query := fmt.Sprintf(`
		SELECT r.id,r.source_billing_run_id,r.settlement_batch_id,COALESCE(s.settlement_no,''),
		       COALESCE(r.status,''),COALESCE(r.total_amount,0)::float8,COALESCE(NULLIF(r.currency,''),'CNY')
		FROM %s.processing_billing_runs r
		JOIN %s.customer_settlement_batches s ON s.id=r.settlement_batch_id
		WHERE r.source_billing_run_id=$1 AND r.run_kind=$2
	`, r.schema, r.schema)
	args := []any{sourceRunID, runKind}
	if requestKey != "" {
		query += " AND r.request_key=$3"
		args = append(args, requestKey)
	}
	query += " ORDER BY r.id LIMIT 1"
	var result salesapp.ProcessingBillingLifecycleResult
	err := tx.QueryRow(ctx, query, args...).Scan(
		&result.BillingRunID, &result.SourceBillingRunID, &result.SettlementBatchID, &result.SettlementNo,
		&result.Status, &result.TotalAmount, &result.Currency,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return salesapp.ProcessingBillingLifecycleResult{}, false, nil
	}
	return result, err == nil, err
}

func (r Repository) createLinkedProcessingBillingRunTx(ctx context.Context, tx pgx.Tx, source processingBillingRunRecord, runKind string, total float64, reason, requestKey, actor string) (salesapp.ProcessingBillingLifecycleResult, error) {
	var settlementBatchID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_settlement_batches(
			customer_id,settlement_no,period_from,period_to,status,total_amount,confirmed_at,created_by,processing_billing_run_id
		)
		SELECT customer_id,'',period_from,period_to,'confirmed',$2,now(),$3,0
		FROM %s.customer_settlement_batches WHERE id=$1
		RETURNING id
	`, r.schema, r.schema), source.SettlementBatchID, total, actor).Scan(&settlementBatchID); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	var billingRunID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.processing_billing_runs(
			customer_id,template_id,template_version_id,settlement_batch_id,run_kind,source_billing_run_id,
			status,total_amount,currency,created_by,confirmed_at,lifecycle_reason,request_key
		) VALUES($1,$2,$3,$4,$5,$6,'confirmed',$7,$8,$9,now(),$10,$11)
		RETURNING id
	`, r.schema), source.CustomerID, source.TemplateID, source.TemplateVersionID, settlementBatchID, runKind,
		source.ID, total, source.Currency, actor, reason, requestKey).Scan(&billingRunID); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	prefix := "CPA"
	if runKind == salesapp.ProcessingBillingRunKindReversal {
		prefix = "CPR"
	}
	settlementNo := fmt.Sprintf("%s-%d-%08d", prefix, source.CustomerID, billingRunID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_settlement_batches
		SET settlement_no=$2,processing_billing_run_id=$3,total_amount=$4,status='confirmed',confirmed_at=now()
		WHERE id=$1
	`, r.schema), settlementBatchID, settlementNo, billingRunID, total); err != nil {
		return salesapp.ProcessingBillingLifecycleResult{}, err
	}
	return salesapp.ProcessingBillingLifecycleResult{
		BillingRunID: billingRunID, SourceBillingRunID: source.ID, SettlementBatchID: settlementBatchID,
		SettlementNo: settlementNo, Status: salesapp.ProcessingBillingStatusConfirmed,
		TotalAmount: total, Currency: source.Currency,
	}, nil
}

func (r Repository) copyProcessingBillingWorkOrdersTx(ctx context.Context, tx pgx.Tx, sourceRunID, targetRunID int64, runKind string) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.processing_billing_work_orders(
			billing_run_id,run_kind,work_order_id,work_order_no,request_no,product_name,completed_at,
			actual_input_kg,actual_output_kg,actual_minutes,actual_units,factory_material_actual_cost
		)
		SELECT $2,$3,work_order_id,work_order_no,request_no,product_name,completed_at,
		       actual_input_kg,actual_output_kg,actual_minutes,actual_units,factory_material_actual_cost
		FROM %s.processing_billing_work_orders WHERE billing_run_id=$1 ORDER BY id
	`, r.schema, r.schema), sourceRunID, targetRunID, runKind)
	return err
}

func (r Repository) copyProcessingBillingReversalLinesTx(ctx context.Context, tx pgx.Tx, source processingBillingRunRecord, reversal salesapp.ProcessingBillingLifecycleResult, cmd salesapp.ReverseProcessingBillingCommand) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,work_order_id,COALESCE(rule_id,0),COALESCE(fee_type,''),COALESCE(fee_name,''),
		       COALESCE(basis,''),COALESCE(base_quantity,0)::float8,COALESCE(unit_price,0)::float8,
		       COALESCE(amount,0)::float8
		FROM %s.processing_billing_line_snapshots WHERE billing_run_id=$1 ORDER BY id
	`, r.schema), source.ID)
	if err != nil {
		return err
	}
	type lineRecord struct {
		ID, WorkOrderID, RuleID         int64
		FeeType, FeeName, Basis         string
		BaseQuantity, UnitPrice, Amount float64
	}
	lines := make([]lineRecord, 0)
	for rows.Next() {
		var line lineRecord
		if err := rows.Scan(&line.ID, &line.WorkOrderID, &line.RuleID, &line.FeeType, &line.FeeName, &line.Basis, &line.BaseQuantity, &line.UnitPrice, &line.Amount); err != nil {
			rows.Close()
			return err
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, line := range lines {
		amount := roundProcessingBillingMoney(-line.Amount)
		var feeItemID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_fee_items(
				customer_id,source_type,source_id,fee_type,amount,currency,occurred_at,settlement_batch_id,status,note,processing_billing_run_id
			) VALUES($1,'processing_billing_reversal',$2,$3,$4,$5,now(),$6,'settled',$7,$8)
			RETURNING id
		`, r.schema), source.CustomerID, line.WorkOrderID, line.FeeType, amount, source.Currency,
			reversal.SettlementBatchID, strings.TrimSpace("冲销："+line.FeeName+" / "+cmd.Reason), reversal.BillingRunID).Scan(&feeItemID); err != nil {
			return err
		}
		calculationJSON, err := json.Marshal(map[string]any{
			"source_billing_run_id": source.ID, "source_line_id": line.ID, "source_basis": line.Basis,
			"source_amount": line.Amount, "reason": cmd.Reason,
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.processing_billing_line_snapshots(
				billing_run_id,work_order_id,rule_id,line_kind,source_line_id,fee_item_id,fee_type,fee_name,
				basis,base_quantity,unit_price,amount,reason,calculation_json
			) VALUES($1,$2,NULLIF($3,0),'reversal',$4,$5,$6,$7,'reversal',$8,$9,$10,$11,$12::jsonb)
		`, r.schema), reversal.BillingRunID, line.WorkOrderID, line.RuleID, line.ID, feeItemID,
			line.FeeType, "冲销："+line.FeeName, line.BaseQuantity, line.UnitPrice, amount, cmd.Reason, calculationJSON); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) validateProcessingBillingAdjustmentWorkOrdersTx(ctx context.Context, tx pgx.Tx, sourceRunID int64, lines []salesapp.ProcessingBillingAdjustmentLineInput) error {
	for _, line := range lines {
		if line.WorkOrderID <= 0 {
			continue
		}
		var exists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT EXISTS(SELECT 1 FROM %s.processing_billing_work_orders WHERE billing_run_id=$1 AND work_order_id=$2)
		`, r.schema), sourceRunID, line.WorkOrderID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("work order is not part of source bill: %d", line.WorkOrderID)
		}
	}
	return nil
}

func processingBillingLifecycleResult(record processingBillingRunRecord) salesapp.ProcessingBillingLifecycleResult {
	return salesapp.ProcessingBillingLifecycleResult{
		BillingRunID: record.ID, SourceBillingRunID: record.SourceRunID,
		SettlementBatchID: record.SettlementBatchID, SettlementNo: record.SettlementNo,
		Status: record.Status, TotalAmount: record.TotalAmount, Currency: record.Currency,
	}
}
