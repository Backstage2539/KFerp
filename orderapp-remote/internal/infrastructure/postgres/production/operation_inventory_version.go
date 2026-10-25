package production

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Include component inventory and all competing reservation versions. The
// serializable posting transaction also catches changes after this snapshot.
func (r Repository) operationInventoryVersionTx(ctx context.Context, tx pgx.Tx, workID int64) (string, error) {
	return r.operationInventoryVersionForMaterialsTx(ctx, tx, workID, nil)
}

func (r Repository) operationInventoryVersionForMaterialsTx(ctx context.Context, tx pgx.Tx, workID int64, extraMaterialIDs []int64) (string, error) {
	var version string
	extra := ""
	has, err := schemaColumnExistsTx(ctx, tx, r.schema, "customer_processing_material_reservations", "id")
	if err != nil {
		return "", err
	}
	if has {
		extra = fmt.Sprintf(` UNION ALL SELECT 'processing:'||b.id,b.xmin::text FROM %s.customer_processing_material_reservations b WHERE b.material_id IN(SELECT material_id FROM components WHERE material_id>0) OR b.component_product_id IN(SELECT component_id FROM components WHERE component_type='product')`, r.schema)
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`WITH components AS(
 SELECT material_id,component_id,component_type FROM %[1]s.work_order_material_reservations WHERE work_order_id=$1
 UNION SELECT 0,unnest($2::bigint[]),'material'
),versions AS(
 SELECT 'location:'||l.material_batch_id||':'||l.warehouse k,l.xmin::text v FROM %[1]s.material_batch_locations l WHERE l.material_id IN(SELECT material_id FROM components)
 UNION ALL SELECT 'material:'||b.id,b.xmin::text FROM %[1]s.material_batches b WHERE b.material_id IN(SELECT material_id FROM components)
 UNION ALL SELECT 'product:'||b.id,b.xmin::text FROM %[1]s.stock_batches b WHERE b.item_type='finished_product' AND b.item_id IN(SELECT component_id FROM components WHERE component_type='product')
 UNION ALL SELECT 'inventory:'||i.product_id||':'||i.bom_spec_id||':'||i.spec_g||':'||i.warehouse,i.xmin::text FROM %[1]s.finished_inventory i WHERE i.product_id IN(SELECT component_id FROM components WHERE component_type='product')
 UNION ALL SELECT 'reservation:'||b.id,b.xmin::text FROM %[1]s.work_order_material_reservations b WHERE b.material_id IN(SELECT material_id FROM components WHERE material_id>0) OR b.component_id IN(SELECT component_id FROM components WHERE component_type='product')
 UNION ALL SELECT 'binding:'||b.id,b.xmin::text FROM %[1]s.work_order_material_reservation_batches b WHERE b.material_id IN(SELECT material_id FROM components WHERE material_id>0) OR (b.component_type='product' AND b.component_id IN(SELECT component_id FROM components WHERE component_type='product'))
	 %[2]s
	 UNION ALL SELECT 'entitlement:'||r.id,r.xmin::text||':'||e.xmin::text FROM %[1]s.customer_stock_entitlement_receipts r JOIN %[1]s.customer_stock_entitlements e ON e.id=r.entitlement_id WHERE e.product_id IN(SELECT component_id FROM components WHERE component_type='product')
	 ) SELECT COALESCE(md5(string_agg(k||':'||v,',' ORDER BY k)),'') FROM versions`, r.schema, extra), workID, extraMaterialIDs).Scan(&version)
	return version, err
}
