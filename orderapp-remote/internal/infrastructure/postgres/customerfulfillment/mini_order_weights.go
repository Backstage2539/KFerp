package customerfulfillment

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
)

// One request can contain multiple warehouse orders. Sum saved line totals and
// expose a unit weight only when every source agrees; never substitute current BOM.
func loadMiniRequestItemWeights(ctx context.Context, q miniDirectShipQuerier, schema string, requestID int64, item *app.MiniDirectShipItemCommand) error {
	return q.QueryRow(ctx, fmt.Sprintf(`SELECT
 CASE WHEN COUNT(material_unit_weight_g)=COUNT(*) AND MIN(material_unit_weight_g)=MAX(material_unit_weight_g) THEN MIN(material_unit_weight_g)::text END,
 CASE WHEN COUNT(material_item_weight_g)=COUNT(*) THEN SUM(material_item_weight_g)::text END,
 CASE WHEN COUNT(logistics_unit_weight_g)=COUNT(*) AND MIN(logistics_unit_weight_g)=MAX(logistics_unit_weight_g) THEN MIN(logistics_unit_weight_g)::text END,
 CASE WHEN COUNT(logistics_item_weight_g)=COUNT(*) THEN SUM(logistics_item_weight_g)::text END,
 CASE WHEN COUNT(*)=0 THEN '{}'::jsonb ELSE jsonb_build_object(
 'rule_version',1,'source','saved_order_items','order_item_ids',jsonb_agg(oi.id ORDER BY oi.id),
 'historical_unknown',bool_or(oi.weight_snapshot_json='{}'::jsonb OR COALESCE((oi.weight_snapshot_json->>'historical_unknown')::boolean,false)),
 'mixed_unit_weight',COALESCE(MIN(material_unit_weight_g)<>MAX(material_unit_weight_g),false) OR COALESCE(MIN(logistics_unit_weight_g)<>MAX(logistics_unit_weight_g),false)) END::text
 FROM %[1]s.customer_direct_ship_request_orders ro JOIN %[1]s.order_items oi ON oi.order_id=ro.order_id
 WHERE ro.request_id=$1 AND oi.product_id=$2 AND COALESCE(oi.bom_spec_id,0)=$3 AND ($3::bigint>0 OR COALESCE(oi.spec,'')=$4)`, schema), requestID, item.ProductID, item.BomSpecID, fmt.Sprintf("%dg", item.SpecG)).Scan(&item.MaterialUnitWeightG, &item.MaterialItemWeightG, &item.LogisticsUnitWeightG, &item.LogisticsItemWeightG, &item.WeightSnapshotJSON)
}
