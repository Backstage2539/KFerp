package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Unbound legacy reservations cannot be assigned to a specific batch safely.
// Protect their full amount in every candidate batch and once per warehouse.
func guardSelectedSemiFinishedUnboundReservationsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) error {
	selected, err := normalizedSelectedSemiFinishedTx(ctx, tx, schema, planID)
	if err != nil {
		return err
	}
	type key struct {
		material, owner int64
		warehouse       string
	}
	totals := map[key][2]int64{}
	for _, s := range selected {
		k := key{s.materialID, s.owner, s.warehouse}
		total := totals[k]
		total[0] += s.qtyG
		total[1] += s.qtyUnits
		totals[k] = total
		unboundG, unboundN, e := unboundWorkOrderReservationQtyTx(ctx, tx, schema, 0, s.materialID, s.warehouse, s.owner)
		if e != nil {
			return e
		}
		var freeG, freeN int64
		e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT GREATEST(0,COALESCE(l.qty_g,0)-COALESCE(r.g,0)),GREATEST(0,COALESCE(l.qty_units,0)-COALESCE(r.n,0)) FROM %[1]s.material_batches b LEFT JOIN %[1]s.material_batch_locations l ON l.material_batch_id=b.id AND l.warehouse=$2 LEFT JOIN LATERAL (SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) g,SUM(GREATEST(0,reserved_units-consumed_units-returned_units)) n FROM %[1]s.work_order_material_reservation_batches WHERE material_batch_id=b.id AND warehouse=$2 AND status='reserved') r ON true WHERE b.id=$1 AND b.material_id=$3 AND b.owner_customer_id=$4 AND b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN('hold','reject')`, schema), s.batchID, s.warehouse, s.materialID, s.owner).Scan(&freeG, &freeN)
		if e == pgx.ErrNoRows {
			return fmt.Errorf("所选半成品批次已不可用，请重新核对")
		}
		if e != nil {
			return e
		}
		if s.qtyG > nonnegativeQuantity(freeG-unboundG) || s.qtyUnits > nonnegativeQuantity(freeN-unboundN) {
			return fmt.Errorf("%s 尚有其他工单未分配批次的预留，请先刷新物料分配", s.warehouse)
		}
	}
	for k, total := range totals {
		freeG, freeN, e := componentSourceAvailabilityTx(ctx, tx, schema, "material", k.material, 0, 0, 0, k.warehouse, k.owner, false)
		if e != nil {
			return e
		}
		unboundG, unboundN, e := unboundWorkOrderReservationQtyTx(ctx, tx, schema, 0, k.material, k.warehouse, k.owner)
		if e != nil {
			return e
		}
		if total[0] > nonnegativeQuantity(freeG-unboundG) || total[1] > nonnegativeQuantity(freeN-unboundN) {
			return fmt.Errorf("%s 的半成品库存已被其他工单预留，请重新核对", k.warehouse)
		}
	}
	return nil
}
