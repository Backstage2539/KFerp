package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/production"
)

// Credit already posted quantities into the reservation projection without
// touching physical stock. The reconciliation event preserves the source state.
func (r Repository) creditLegacyConsumptionTx(ctx context.Context, tx pgx.Tx, materials []app.LegacyReviewMaterial) error {
	for _, m := range materials {
		var g, n int64
		if factor := app.OperationUnitFactor(m.Unit); factor > 0 {
			g = int64(math.Round(m.ConsumedQty * factor))
		} else {
			n = int64(m.ConsumedQty)
		}
		_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET consumed_g=GREATEST(consumed_g,$2),consumed_units=GREATEST(consumed_units,$3),reserved_g=GREATEST(reserved_g,$2),reserved_units=GREATEST(reserved_units,$3),status='reserved',updated_at=now() WHERE id=$1`, r.schema), m.ReservationID, g, n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) legacyOperationCostsTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow, materials []app.LegacyReviewMaterial, bindings map[string]app.OperationMaterialBinding, total float64) (map[string]float64, error) {
	out := map[string]float64{}
	if total <= 0 {
		return out, nil
	}
	nodes := map[string]bool{}
	sum := 0.0
	for _, m := range materials {
		if m.ConsumedQty <= 0 {
			continue
		}
		node := bindings[m.Key].NodeKey
		nodes[node] = true
		var cost float64
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT GREATEST(COALESCE((SELECT SUM(i.total_cost) FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$1 AND e.status='submitted' AND e.purpose='material_consumption_for_manufacture' AND i.material_id=$2 AND i.product_id=$3 AND i.bom_spec_id=$4 AND i.spec_g=$5 AND i.owner_customer_id=$6),0),COALESCE((SELECT SUM(CASE WHEN l.deduct_g>0 THEN l.deduct_g*b.unit_cost/1000 ELSE l.deduct_units*b.unit_cost END) FROM %[1]s.material_consumption_logs l JOIN %[1]s.material_batches b ON b.id=l.material_batch_id WHERE l.running_item_id=$7 AND l.material_id=$2 AND $3=0),0))::float8`, r.schema), wo.ID, m.MaterialID, m.ProductID, m.BomSpecID, m.SpecG, m.OwnerCustomerID, wo.RunningItemID).Scan(&cost)
		if err != nil {
			return nil, err
		}
		out[node] += cost
		sum += cost
	}
	if len(nodes) == 1 {
		for node := range nodes {
			out[node] = total
		}
		return out, nil
	}
	if math.Abs(total-sum) > 0.01 {
		return nil, fmt.Errorf("历史材料成本 %g 无法逐项归入工序（可追溯 %g）；请核对原耗料单据后转换", total, sum)
	}
	return out, nil
}
