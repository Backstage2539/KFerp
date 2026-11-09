package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Copy the unknown-cost state with physical batch identity; a transfer or return
// must never turn an unavailable cost into a known zero.
func (r Repository) copyStockBatchCostAvailabilityTx(ctx context.Context, tx pgx.Tx, sourceID, targetID int64) error {
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.stock_batches target SET cost_unavailable=source.cost_unavailable FROM %[1]s.stock_batches source WHERE target.id=$2 AND source.id=$1`, r.schema), sourceID, targetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("成本来源批次已变化，请重新核对")
	}
	return nil
}

// Manufacturing receipts inherit the unknown state from actual consumed
// batches and posted operation events; an amount of zero is not a valuation.
func (r Repository) workOrderCostUnavailableTx(ctx context.Context, tx pgx.Tx, workOrderID int64) (bool, error) {
	var unknown bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.stock_entries e JOIN %[1]s.stock_entry_items i ON i.stock_entry_id=e.id JOIN %[1]s.stock_entry_batch_allocations a ON a.stock_entry_item_id=i.id LEFT JOIN %[1]s.material_batches mb ON mb.id=a.material_batch_id LEFT JOIN %[1]s.stock_batches sb ON sb.id=a.stock_batch_id WHERE e.work_order_id=$1 AND e.status='submitted' AND e.purpose='material_consumption_for_manufacture' AND (COALESCE(mb.cost_unavailable,false) OR COALESCE(sb.cost_unavailable,false)))`, r.schema), workOrderID).Scan(&unknown)
	if err != nil || unknown {
		return unknown, err
	}
	events, err := stockSchemaRelationExistsTx(ctx, tx, r.schema, "production_operation_events")
	if err != nil {
		return false, err
	}
	if events {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.production_operation_events WHERE work_order_id=$1 AND status='posted' AND COALESCE((payload->>'cost_unavailable')::boolean,false)) OR EXISTS(SELECT 1 FROM %[1]s.job_cards j WHERE j.work_order_id=$1 AND COALESCE(to_jsonb(j)->>'scheduling_mode','batch')='quantity' AND COALESCE(to_jsonb(j)->>'cost_method','time')<>'piece' AND COALESCE(NULLIF(to_jsonb(j)->>'throughput_per_hour','')::numeric,0)<=0 AND NOT EXISTS(SELECT 1 FROM %[1]s.production_operation_events e WHERE e.job_card_id=j.id AND e.kind='complete' AND e.status='posted'))`, r.schema), workOrderID).Scan(&unknown)
		if err != nil || unknown {
			return unknown, err
		}
	}
	logs, err := stockSchemaRelationExistsTx(ctx, tx, r.schema, "material_consumption_logs")
	if err != nil {
		return false, err
	}
	if logs {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.work_orders w JOIN %[1]s.material_consumption_logs l ON l.running_item_id=w.running_item_id LEFT JOIN %[1]s.material_batches mb ON mb.id=l.material_batch_id LEFT JOIN %[1]s.stock_batches sb ON l.material_batch_id=0 AND sb.batch_code=l.material_batch_code WHERE w.id=$1 AND (l.deduct_g>0 OR l.deduct_units>0) AND (COALESCE(mb.cost_unavailable,false) OR COALESCE(sb.cost_unavailable,false)))`, r.schema), workOrderID).Scan(&unknown)
	}
	return unknown, err
}
