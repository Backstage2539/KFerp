package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Started material is initially booked in full; unallocated components in the
// plan-level packaging workflow are still inventory, not production expense.
// A later material_return event reduces the booked total as remaining_cost falls.
func (r Repository) deferredPackagingRemainderCostTx(ctx context.Context, tx pgx.Tx, workOrderID int64) (float64, error) {
	var value float64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.remaining_cost),0)::float8
 FROM %[1]s.production_started_inputs i
 WHERE i.work_order_id=$1 AND i.status='posted'
 AND EXISTS(SELECT 1 FROM %[1]s.production_operation_events e
  WHERE e.job_card_id=i.job_card_id AND e.kind='complete'
  AND e.status IN ('posting','posted') AND e.payload->>'remainder_mode'='plan_transfer')`, r.schema), workOrderID).Scan(&value)
	return value, err
}
