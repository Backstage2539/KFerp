package production

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	productionapp "orderapp/internal/application/production"
)

// A submitted supplier keeps its frozen order instructions. Reusing it for a
// later order is unsafe if that order requires an upstream instruction that
// was never included in the supplier's snapshot.
func validateInflightOrderNotesTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`WITH RECURSIVE consumers(allocation_id, item_id) AS (
 SELECT id,production_plan_item_id FROM %[1]s.production_supply_allocations WHERE production_plan_id=$1
	 UNION SELECT c.allocation_id,d.production_plan_item_id FROM consumers c JOIN %[1]s.production_plan_item_dependencies d ON d.depends_on_plan_item_id=c.item_id AND d.production_plan_id=$1
	 ), supplier_nodes(allocation_id,supplier_id) AS (
	 SELECT id,supplier_work_order_id FROM %[1]s.production_supply_allocations WHERE production_plan_id=$1
	 UNION SELECT n.allocation_id,d.depends_on_work_order_id FROM supplier_nodes n JOIN %[1]s.work_order_dependencies d ON d.work_order_id=n.supplier_id
	 )
 SELECT wo.work_order_no,COALESCE(NULLIF(wo.output_type,''),'product'),
 CASE WHEN wo.output_type='material' THEN COALESCE(wo.output_material_id,0) ELSE COALESCE(NULLIF(wo.output_product_id,0),wo.product_id,0) END,
 COALESCE(wo.bom_spec_id,0),COALESCE(wo.bom_variant_id,0),
 COALESCE(to_jsonb(wo)->>'related_orders_snapshot_json','null'),
	 COALESCE(src.value->>'order_no',''),oi.order_id,oi.id,COALESCE(to_jsonb(oi)->>'operation_notes_json','[]')
	 FROM consumers c JOIN supplier_nodes n ON n.allocation_id=c.allocation_id
	 JOIN %[1]s.work_orders wo ON wo.id=n.supplier_id
 JOIN %[1]s.production_plan_items pi ON pi.id=c.item_id
 CROSS JOIN LATERAL jsonb_array_elements(COALESCE(pi.demand_sources_json,'[]'::jsonb)) src(value)
 JOIN %[1]s.order_items oi ON oi.id=(src.value->>'order_item_id')::bigint AND oi.order_id=(src.value->>'order_id')::bigint`, schema), planID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var supplier, typ, rawSnapshot, orderNo, rawNotes string
		var outputID, specID, variantID, orderID, itemID int64
		if err = rows.Scan(&supplier, &typ, &outputID, &specID, &variantID, &rawSnapshot, &orderNo, &orderID, &itemID, &rawNotes); err != nil {
			return err
		}
		var notes []productionapp.ProductionOrderOperationNote
		if err = json.Unmarshal([]byte(rawNotes), &notes); err != nil {
			return err
		}
		var frozen []productionapp.ProductionDemandOrder
		if err = json.Unmarshal([]byte(rawSnapshot), &frozen); err != nil {
			return err
		}
		for _, note := range notes {
			if note.OutputType != typ || note.OutputID != outputID || note.BomSpecID != specID {
				continue
			}
			covered := false
			for _, order := range frozen {
				if order.OrderID != orderID || order.OrderItemID != itemID {
					continue
				}
				for _, old := range order.OperationNotes {
					if old.OutputType == note.OutputType && old.OutputID == note.OutputID && old.BomSpecID == note.BomSpecID && old.BomVariantID == note.BomVariantID && old.RouteID == note.RouteID && old.StepID == note.StepID && old.OperationID == note.OperationID && strings.TrimSpace(old.Note) == strings.TrimSpace(note.Note) {
						covered = true
						break
					}
				}
			}
			if !covered {
				return fmt.Errorf("订单 %s 的工序备注未包含在已有在产工单 %s 中，请改用新生产或先调整备注", orderNo, supplier)
			}
		}
	}
	return rows.Err()
}
