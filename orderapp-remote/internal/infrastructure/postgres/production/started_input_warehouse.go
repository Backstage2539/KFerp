package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
)

// A frozen source is a default, not authority to deduct stock that has already
// moved. Submitted picking/return/consumption documents determine the remaining
// issued stock's actual location; preview and submit both run this query.
func startedMaterialWarehouseTx(ctx context.Context, tx pgx.Tx, schema string, work int64, m app.OperationMaterialUse) (string, error) {
	var warehouse string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT warehouse FROM (
 SELECT warehouse,SUM(qty) net FROM (
 SELECT CASE WHEN e.purpose='material_transfer_for_manufacture' AND NOT e.is_return THEN i.to_warehouse ELSE i.from_warehouse END warehouse,
 (CASE WHEN e.purpose='material_transfer_for_manufacture' AND NOT e.is_return THEN 1 ELSE -1 END)*(CASE WHEN $7 THEN i.qty_g ELSE i.qty_units END) qty
 FROM %[1]s.stock_entries e JOIN %[1]s.stock_entry_items i ON i.stock_entry_id=e.id
 WHERE e.work_order_id=$1 AND e.status='submitted' AND e.purpose IN('material_transfer_for_manufacture','material_consumption_for_manufacture')
 AND i.material_id=$2 AND i.product_id=$3 AND i.bom_spec_id=$4 AND i.spec_g=$5 AND i.owner_customer_id=$6 AND i.bom_variant_id=$8
 UNION ALL
 SELECT l.warehouse,CASE WHEN $7 THEN l.qty_change_g ELSE l.qty_change_units END
 FROM %[1]s.stock_ledger_entries l JOIN %[1]s.production_operation_events e ON e.id=l.source_doc_id AND e.work_order_id=$1
 WHERE l.source_doc_type='production_input_return' AND l.item_type=CASE WHEN $2::bigint>0 THEN 'material' ELSE 'finished_product' END AND l.item_id=CASE WHEN $2::bigint>0 THEN $2 ELSE $3 END AND l.bom_spec_id=$4 AND l.spec_g=$5 AND l.owner_customer_id=$6 AND l.bom_variant_id=$8
 ) moves GROUP BY warehouse) issued WHERE net>0 ORDER BY CASE WHEN warehouse='wip' THEN 0 ELSE 1 END,warehouse LIMIT 1`, schema), work, m.MaterialID, m.ProductID, m.BomSpecID, m.SpecG, m.OwnerCustomerID, app.OperationUnitFactor(m.Unit) > 0, m.BomVariantID).Scan(&warehouse)
	if err == pgx.ErrNoRows {
		return m.Warehouse, nil
	}
	if err != nil {
		return "", err
	}
	return warehouse, nil
}
