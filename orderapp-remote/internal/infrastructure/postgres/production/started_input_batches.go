package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
)

func startedMaterialBatchesTx(ctx context.Context, tx pgx.Tx, schema string, work int64, m app.OperationMaterialUse) ([]app.OperationMaterialBatch, error) {
	sql := `SELECT b.id,b.batch_code,l.qty_g,l.qty_units FROM %[1]s.material_batches b JOIN %[1]s.material_batch_locations l ON l.material_batch_id=b.id WHERE b.material_id=$1 AND b.owner_customer_id=$2 AND l.warehouse=$3 AND b.status='active' AND b.quality_status NOT IN('hold','reject') AND (l.qty_g>0 OR l.qty_units>0) ORDER BY b.received_at,b.id`
	args := []any{m.MaterialID, m.OwnerCustomerID, m.Warehouse}
	product := m.ProductID > 0
	identity := m.MaterialID
	batchColumn := "material_batch_id"
	if product {
		identity = m.ProductID
		batchColumn = "stock_batch_id"
		sql = `SELECT b.id,b.batch_code,b.remaining_g,b.remaining_units FROM %[1]s.stock_batches b LEFT JOIN LATERAL(SELECT warehouse FROM %[1]s.stock_ledger_entries WHERE item_type=b.item_type AND item_id=b.item_id AND source_batch_code=b.batch_code AND owner_customer_id=b.owner_customer_id ORDER BY id DESC LIMIT 1) l ON true WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.owner_customer_id=$2 AND COALESCE(l.warehouse,'finished_goods')=$3 AND b.bom_spec_id=$4 AND b.spec_g=$5 AND b.bom_variant_id=$6 AND b.quality_status NOT IN('hold','reject') AND (b.remaining_g>0 OR b.remaining_units>0) AND NOT EXISTS(SELECT 1 FROM %[1]s.customer_stock_entitlement_receipts er JOIN %[1]s.customer_stock_entitlements e ON e.id=er.entitlement_id WHERE er.stock_batch_id=b.id AND e.status='active') ORDER BY b.created_at,b.id`
		args = []any{m.ProductID, m.OwnerCustomerID, m.Warehouse, m.BomSpecID, m.SpecG, m.BomVariantID}
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(sql, schema), args...)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		id, g, n int64
		code     string
	}
	all := []candidate{}
	for rows.Next() {
		var b candidate
		if err = rows.Scan(&b.id, &b.code, &b.g, &b.n); err != nil {
			rows.Close()
			return nil, err
		}
		all = append(all, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := []app.OperationMaterialBatch{}
	for _, b := range all {
		var g, n int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)),0)::bigint,COALESCE(SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)),0)::bigint FROM %[1]s.work_order_material_reservation_batches rb JOIN %[1]s.work_order_material_reservations r ON r.id=rb.reservation_id WHERE rb.%[2]s=$1 AND rb.work_order_id<>$2 AND rb.warehouse=$3 AND rb.owner_customer_id=$4 AND rb.status='reserved' AND r.status='reserved'`, schema, batchColumn), b.id, work, m.Warehouse, m.OwnerCustomerID).Scan(&g, &n); err != nil {
			return nil, err
		}
		pg, pn, e := stockrepo.OperationOtherProcessingReservedTx(ctx, tx, schema, work, b.id, identity, m.OwnerCustomerID, m.Warehouse, product)
		if e != nil {
			return nil, e
		}
		qty := float64(max(int64(0), b.n-n-pn))
		if factor := app.OperationUnitFactor(m.Unit); factor > 0 {
			qty = float64(max(int64(0), b.g-g-pg)) / factor
		}
		if qty > 0 {
			result = append(result, app.OperationMaterialBatch{BatchID: b.id, BatchCode: b.code, AvailableQty: qty})
		}
	}
	return result, nil
}
