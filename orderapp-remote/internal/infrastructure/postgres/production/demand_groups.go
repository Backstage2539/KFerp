package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"math"
	app "orderapp/internal/application/production"
	domain "orderapp/internal/domain/production"
	"strings"
)

// Legacy selection_key remains accepted. V2 includes frozen conversion, customer,
// warehouse, variant and the exact order subset, so adjacent specs cannot collide.
func productionDemandSelectionID(row UnprodNeedRow) string {
	raw := fmt.Sprintf("%d|%d|%d|%d|%s|%s|%s", row.ProductID, row.BomSpecID, row.BomVariantID, row.SpecG, row.SalesSpecSnapshotJSON, row.OrderNos, row.DemandStatus)
	return fmt.Sprintf("v2:%x", sha256.Sum256([]byte(raw)))
}
func productionDemandIsSelected(row app.UnprodNeedRow, selected map[string]bool) bool {
	if row.SelectionID != "" && selected[row.SelectionID] {
		return true
	}
	key := row.SelectionKey
	if key == "" {
		key = productionDemandSelectionKey(row.ProductID, row.BomSpecID, row.SpecG)
	}
	return selected[key]
}
func groupProductionDemandProducts(rows []app.UnprodNeedRow) []app.ProductionDemandProductGroup {
	out := []app.ProductionDemandProductGroup{}
	products := map[int64]int{}
	specs := map[string]int{}
	for _, row := range rows {
		id := row.ParentProductID
		if id <= 0 {
			id = row.ProductID
		}
		i, ok := products[id]
		if !ok {
			i = len(out)
			products[id] = i
			out = append(out, app.ProductionDemandProductGroup{ProductID: id, ProductName: firstNonEmpty(row.ParentProductName, row.Product), Specs: []app.ProductionDemandSpecGroup{}})
		}
		key := fmt.Sprintf("%d:%d:%s:%s", id, row.BomSpecID, row.SpecLabel, row.SalesUnit)
		j, ok := specs[key]
		if !ok {
			j = len(out[i].Specs)
			specs[key] = j
			out[i].Specs = append(out[i].Specs, app.ProductionDemandSpecGroup{Key: key, SpecLabel: row.SpecLabel, SalesUnit: row.SalesUnit, Rows: []app.UnprodNeedRow{}})
		}
		g := &out[i].Specs[j]
		g.Rows = append(g.Rows, row)
		g.SalesSpecCount += row.SalesSpecCount
		g.GapSalesSpecCount += row.GapSalesSpecCount
	}
	return out
}

func splitStructuredProductionDemandRow(ctx context.Context, q productionDemandQueryer, schema string, row UnprodNeedRow) ([]UnprodNeedRow, error) {
	ids := []int64{}
	detailByID := map[int64]app.ProductionDemandOrder{}
	for _, d := range row.OrderDetails {
		ids = append(ids, d.OrderItemID)
		detailByID[d.OrderItemID] = d
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT oi.id,COALESCE(plan.id,0),COALESCE(plan.plan_no,''),COALESCE(plan.status,''),COALESCE(plan.wo_id,0),COALESCE(plan.wo_no,''),COALESCE(plan.wo_status,'')
 FROM %[1]s.order_items oi JOIN %[1]s.orders o ON o.id=oi.order_id
 LEFT JOIN LATERAL (
  SELECT pp.id,pp.plan_no,pp.status,wo.id wo_id,wo.work_order_no wo_no,wo.status wo_status
  FROM %[1]s.production_plan_items pi JOIN %[1]s.production_plans pp ON pp.id=pi.production_plan_id LEFT JOIN %[1]s.work_orders wo ON wo.production_plan_item_id=pi.id
  WHERE pp.status<>'cancelled' AND COALESCE(wo.status,'')<>'cancelled' AND pi.product_id=oi.product_id
   AND ((jsonb_array_length(pi.demand_sources_json)>0 AND EXISTS(SELECT 1 FROM jsonb_array_elements(pi.demand_sources_json) source WHERE (source->>'order_item_id')::bigint=oi.id))
    OR (jsonb_array_length(pi.demand_sources_json)=0 AND pi.bom_spec_id=COALESCE(oi.bom_spec_id,0) AND pi.bom_variant_id=COALESCE(oi.bom_variant_id,0) AND pi.spec_g=$2 AND pi.customer_id=COALESCE(o.customer_id,0) AND o.order_no=ANY(string_to_array(replace(pi.order_nos,' ',''),','))))
  ORDER BY pp.id DESC,wo.id DESC LIMIT 1
 ) plan ON true WHERE oi.id=ANY($1::bigint[]) ORDER BY oi.id`, schema), ids, row.SpecG)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UnprodNeedRow{}
	groupIndex := map[string]int{}
	for rows.Next() {
		var itemID int64
		var state productionDemandPlanState
		var planStatus, woStatus string
		if err := rows.Scan(&itemID, &state.ProductionPlanID, &state.ProductionPlanNo, &planStatus, &state.WorkOrderID, &state.WorkOrderNo, &woStatus); err != nil {
			return nil, err
		}
		state.Status = productionDemandStatusFromPlan(planStatus, woStatus)
		if state.Status == "" {
			state.Status = "unplanned"
		}
		key := fmt.Sprintf("%s:%d:%d", state.Status, state.ProductionPlanID, state.WorkOrderID)
		i, ok := groupIndex[key]
		if !ok {
			i = len(out)
			groupIndex[key] = i
			n := row
			n.OrderDetails = nil
			n.OrderNos = ""
			n.SalesSpecCount = 0
			n.DemandStatus = state.Status
			n.DemandStatusLabel = productionDemandStatusLabel(state.Status)
			n.ProductionPlanID = state.ProductionPlanID
			n.ProductionPlanNo = state.ProductionPlanNo
			n.WorkOrderID = state.WorkOrderID
			n.WorkOrderNo = state.WorkOrderNo
			out = append(out, n)
		}
		n := &out[i]
		d := detailByID[itemID]
		n.OrderDetails = append(n.OrderDetails, d)
		n.SalesSpecCount += d.Quantity
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		n := &out[i]
		nos := map[string]bool{}
		forced := 0.0
		for _, d := range n.OrderDetails {
			nos[d.OrderNo] = true
			if d.ForceProduce {
				forced += d.Quantity
			}
		}
		n.OrderNos = joinProductionDemandOrderNos(nos)
		n.NeedUnits = int64(math.Ceil(n.SalesSpecCount))
		conversion := n.InventoryQtyPerSalesUnit
		if conversion <= 0 && row.SalesSpecCount > 0 {
			conversion = row.NeedInventoryQty / row.SalesSpecCount
		}
		n.NeedInventoryQty = domain.SalesSpecCountToInventoryQuantity(n.SalesSpecCount, conversion)
		n.NeedG = domain.InventoryQuantityToLegacyGrams(n.NeedInventoryQty, n.InventoryUnit)
		n.GapInventoryQty = n.NeedInventoryQty
		if n.DemandStatus == "unplanned" {
			forceQty := forced * conversion
			n.GapInventoryQty = forceQty + math.Max(0, n.NeedInventoryQty-forceQty-row.AvailableInventoryQty)
		}
		n.GapSalesSpecCount = n.SalesSpecCount
		if conversion > 0 {
			n.GapSalesSpecCount = n.GapInventoryQty / conversion
		}
		n.GapG = domain.InventoryQuantityToLegacyGrams(n.GapInventoryQty, n.InventoryUnit)
		n.DemandSelectable = n.DemandStatus == "unplanned" && strings.TrimSpace(n.BlockingReason) == "" && productionDemandHasGap(*n)
	}
	return out, nil
}

func savePlanDemandSourcesTx(ctx context.Context, tx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, schema string, itemID int64, sources []app.ProductionDemandOrder) error {
	raw, err := json.Marshal(sources)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		raw = []byte("[]")
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET demand_sources_json=$2 WHERE id=$1`, schema), itemID, raw)
	return err
}
