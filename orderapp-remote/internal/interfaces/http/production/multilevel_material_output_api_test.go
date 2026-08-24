package production

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	postgresproduction "orderapp/internal/infrastructure/postgres/production"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestProductionPlanAPICreatesMaterialShortageWorkOrderAndCompletesIntoDownstreamWIP(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1010, 10, "MB-ROASTED-OPEN", "在制熟豆", 10_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "MB-GREEN-OPEN", "生产生豆", 20_000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 1020, 20, "MB-BAG-OPEN", "227g包装袋", 100)

	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from":         "2026-08-01",
		"to":           "2026-08-31",
		"selected":     []string{"1-227"},
		"input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("POST /api/production-plans status=%d body=%s", create.Code, create.Body.String())
	}

	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatalf("load production plan id: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d", planID), 2)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(
		"production_plan_id=%d AND output_type='product' AND output_product_id=1 AND output_material_id=0 AND planned_output_g=22700",
		planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(
		"production_plan_id=%d AND output_type='material' AND output_product_id=0 AND output_material_id=10 AND output_qty=12.7 AND output_unit='kg' AND planned_output_g=12700 AND planned_g=15875 AND target_warehouse='wip'",
		planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", fmt.Sprintf(
		"production_plan_id=%d AND material_id=10 AND required_g=12700 AND required_units=0",
		planID,
	), 1)
	graphDetail := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", planID), nil)
	if graphDetail.Code != http.StatusOK {
		t.Fatalf("load manufacturing graph status=%d body=%s", graphDetail.Code, graphDetail.Body.String())
	}
	var graphPayload struct {
		ManufacturingPlan struct {
			Blocking bool `json:"blocking"`
			Nodes    []struct {
				OutputType       string `json:"output_type"`
				OutputMaterialID int64  `json:"output_material_id"`
				RequiredG        int64  `json:"required_g"`
				StockCoveredG    int64  `json:"stock_covered_g"`
				ShortageG        int64  `json:"shortage_g"`
				Action           string `json:"action"`
				Depth            int    `json:"depth"`
			} `json:"nodes"`
			Edges []struct {
				ConsumerPlanItemID int64 `json:"consumer_plan_item_id"`
				SupplierPlanItemID int64 `json:"supplier_plan_item_id"`
				RequiredG          int64 `json:"required_g"`
			} `json:"edges"`
		} `json:"manufacturing_plan"`
	}
	if err := json.Unmarshal(graphDetail.Body.Bytes(), &graphPayload); err != nil {
		t.Fatalf("decode manufacturing graph: %v body=%s", err, graphDetail.Body.String())
	}
	foundRoastedCoverage := false
	for _, node := range graphPayload.ManufacturingPlan.Nodes {
		if node.OutputType == "material" && node.OutputMaterialID == 10 &&
			node.RequiredG == 22_700 && node.StockCoveredG == 10_000 && node.ShortageG == 12_700 &&
			node.Action == "manufacture" && node.Depth == 1 {
			foundRoastedCoverage = true
		}
	}
	if graphPayload.ManufacturingPlan.Blocking || !foundRoastedCoverage || len(graphPayload.ManufacturingPlan.Edges) < 3 {
		t.Fatalf("manufacturing graph=%+v, want non-blocking root/material/component tree with 22.7kg required, 10kg stock and 12.7kg shortage", graphPayload.ManufacturingPlan)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,
		       CASE WHEN output_type='material' THEN '烘焙' ELSE '包装' END,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,
		       15
		FROM %s.production_plan_items
		WHERE production_plan_id=%d;
	`, schema, schema, planID))

	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("POST production plan submit status=%d body=%s", submit.Code, submit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", planID), 2)
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies", "material_id=10 AND required_g=12700 AND required_units=0", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", "material_id=10 AND required_g=22700 AND reserved_g=10000 AND status='reserved'", 1)

	var rootWorkOrderID, upstreamWorkOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND output_type='product' AND output_product_id=1
	`, schema), planID).Scan(&rootWorkOrderID); err != nil {
		t.Fatalf("load product work order: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND output_type='material' AND output_material_id=10
	`, schema), planID).Scan(&upstreamWorkOrderID); err != nil {
		t.Fatalf("load material work order: %v", err)
	}
	workOrderList := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/work-orders?limit=20", nil)
	if workOrderList.Code != http.StatusOK ||
		!strings.Contains(workOrderList.Body.String(), `"output_type":"material"`) ||
		!strings.Contains(workOrderList.Body.String(), `"output_material_id":10`) ||
		!strings.Contains(workOrderList.Body.String(), `"upstream_blocked":true`) ||
		!strings.Contains(workOrderList.Body.String(), `"upstream_dependencies":[{`) {
		t.Fatalf("typed/dependency work-order list status=%d body=%s", workOrderList.Code, workOrderList.Body.String())
	}
	rootDetail := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/produce/work-orders/%d", rootWorkOrderID), nil)
	if rootDetail.Code != http.StatusOK ||
		!strings.Contains(rootDetail.Body.String(), `"upstream_blocked":true`) ||
		!strings.Contains(rootDetail.Body.String(), fmt.Sprintf(`"depends_on_work_order_id":%d`, upstreamWorkOrderID)) {
		t.Fatalf("typed/dependency work-order detail status=%d body=%s", rootDetail.Code, rootDetail.Body.String())
	}
	planDetail := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", planID), nil)
	if planDetail.Code != http.StatusOK ||
		!strings.Contains(planDetail.Body.String(), `"related_work_orders"`) ||
		!strings.Contains(planDetail.Body.String(), `"output_type":"material"`) ||
		!strings.Contains(planDetail.Body.String(), `"output_material_id":10`) {
		t.Fatalf("typed production-plan related work orders status=%d body=%s", planDetail.Code, planDetail.Body.String())
	}

	blocked := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
	if blocked.Code != http.StatusBadRequest || !strings.Contains(blocked.Body.String(), "依赖") {
		t.Fatalf("start downstream before material dependency status=%d body=%s, want dependency rejection", blocked.Code, blocked.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='released' AND running_item_id=0", rootWorkOrderID), 1)

	upstreamStart := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", upstreamWorkOrderID), nil)
	if upstreamStart.Code != http.StatusOK {
		t.Fatalf("start material work order status=%d body=%s", upstreamStart.Code, upstreamStart.Body.String())
	}
	var upstreamRunningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), upstreamWorkOrderID).Scan(&upstreamRunningItemID); err != nil {
		t.Fatalf("load material running item: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "produce_running_items", fmt.Sprintf(
		"id=%d AND output_type='material' AND output_material_id=10 AND output_qty=12.7 AND output_unit='kg' AND target_warehouse='wip'",
		upstreamRunningItemID,
	), 1)

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),actual_input_qty=15.875,actual_output_qty=12.7
		WHERE work_order_id=%d;
	`, schema, upstreamWorkOrderID))
	upstreamComplete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", upstreamWorkOrderID), map[string]any{
		"finished_qty_g":     12_700,
		"finished_qty_units": 0,
		"consumed_input_g":   15_875,
		"warehouse":          "wip",
		"note":               "多级生产测试",
	})
	if upstreamComplete.Code != http.StatusOK {
		t.Fatalf("complete material work order status=%d body=%s", upstreamComplete.Code, upstreamComplete.Body.String())
	}

	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed'", upstreamWorkOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "material_batches", "material_id=10 AND received_g=12700 AND remaining_g=12700 AND status='active' AND quality_status='pass'", 1)
	assertProductionFlowCount(t, pool, schema, "material_batches", "material_id=10 AND unit_cost=62.5", 1)
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_id=10 AND warehouse='wip' AND qty_g=12700", 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches", "item_type='material' AND item_id=10 AND remaining_g=12700 AND unit_cost=62.5", 1)
	assertProductionFlowCount(t, pool, schema, "stock_ledger_entries", fmt.Sprintf("source_doc_type='production_run' AND source_doc_id=%d AND item_type='material' AND item_id=10 AND warehouse='wip' AND qty_change_g=12700", upstreamRunningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND entry_type='finished_receipt' AND status='submitted'", upstreamWorkOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entry_items", "item_type='material' AND material_id=10 AND to_warehouse='wip' AND qty_g=12700", 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='work_order' AND entity_id=%d AND action='complete'", upstreamWorkOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "materials", "id=10 AND onhand_g=22700", 1)
	assertProductionFlowCount(t, pool, schema, "materials", "id=30 AND onhand_g=4125", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf(
		"work_order_id=%d AND material_id=10 AND required_g=22700 AND reserved_g=22700 AND status='reserved'",
		rootWorkOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf(
		"work_order_id=%d AND material_id=10 AND reserved_g=12700 AND consumed_g=0 AND status='reserved'",
		rootWorkOrderID,
	), 1)

	downstreamStart := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
	if downstreamStart.Code != http.StatusOK {
		t.Fatalf("start downstream after material completion status=%d body=%s", downstreamStart.Code, downstreamStart.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='running' AND running_item_id>0", rootWorkOrderID), 1)
	var rootRunningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), rootWorkOrderID).Scan(&rootRunningItemID); err != nil {
		t.Fatalf("load product running item: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),actual_input_qty=22.7,actual_output_qty=22.7
		WHERE work_order_id=%d;
	`, schema, rootWorkOrderID))
	downstreamComplete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", rootWorkOrderID), map[string]any{
		"finished_units":   100,
		"finished_loose_g": 0,
		"consumed_input_g": 22_700,
		"warehouse":        "finished_goods",
		"note":             "验证上游产出批次被下游消耗",
	})
	if downstreamComplete.Code != http.StatusOK {
		t.Fatalf("complete downstream product work order status=%d body=%s", downstreamComplete.Code, downstreamComplete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf(
		"work_order_id=%d AND material_id=10 AND reserved_g=12700 AND consumed_g=12700 AND status='consumed'",
		rootWorkOrderID,
	), 1)
	producedBatchCode := fmt.Sprintf("MP-%010d", upstreamRunningItemID)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf(
		"running_item_id=%d AND material_id=10 AND material_batch_code='%s' AND deduct_g=12700",
		rootRunningItemID, producedBatchCode,
	), 1)
}

func TestProductionPlanAPITypedProductComponentRecursesWithPartialFinishedStock(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedTypedProductComponentFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 9030, 30, "MB-TYPED-GREEN", "生产生豆", 20_000)

	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from":         "2026-08-01",
		"to":           "2026-08-31",
		"selected":     []string{"1-227"},
		"input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create typed product-component plan status=%d body=%s", create.Code, create.Body.String())
	}

	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatalf("load typed product-component plan id: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d", planID), 2)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(
		"production_plan_id=%d AND output_type='product' AND output_product_id=2 AND output_material_id=0 AND output_qty=12700 AND output_unit='g' AND planned_output_g=12700 AND target_warehouse='finished_goods' AND bom_version_id=300 AND component_snapshot_json::text LIKE '%%生产生豆%%'",
		planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(
		"production_plan_id=%d AND output_type='material' AND output_material_id=30",
		planID,
	), 0)
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", fmt.Sprintf(
		"production_plan_id=%d AND component_type='product' AND component_id=2 AND component_spec_g=1 AND material_id=0 AND required_g=12700 AND required_units=0",
		planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_supply_gaps", fmt.Sprintf("production_plan_id=%d", planID), 0)

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,
		       CASE WHEN output_product_id=2 THEN '烘焙' ELSE '包装' END,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15
		FROM %s.production_plan_items
		WHERE production_plan_id=%d;
	`, schema, schema, planID))

	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit typed product-component plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", planID), 2)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf(
		"production_plan_id=%d AND output_type='product' AND output_product_id=2 AND output_material_id=0 AND output_qty=12700 AND output_unit='g' AND target_warehouse='finished_goods' AND bom_version_id=300 AND material_snapshot::text LIKE '%%生产生豆%%'",
		planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies", "component_type='product' AND component_id=2 AND component_spec_g=1 AND material_id=0 AND required_g=12700 AND required_units=0", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", "component_type='product' AND component_id=2 AND component_spec_g=1 AND material_id=0 AND required_g=22700 AND reserved_g=10000 AND status='reserved'", 1)

	var rootWorkOrderID, upstreamWorkOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND output_type='product' AND output_product_id=1
	`, schema), planID).Scan(&rootWorkOrderID); err != nil {
		t.Fatalf("load typed root work order: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND output_type='product' AND output_product_id=2
	`, schema), planID).Scan(&upstreamWorkOrderID); err != nil {
		t.Fatalf("load typed upstream work order: %v", err)
	}
	blocked := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
	if blocked.Code != http.StatusBadRequest || !strings.Contains(blocked.Body.String(), "依赖") {
		t.Fatalf("start typed downstream before upstream status=%d body=%s, want dependency rejection", blocked.Code, blocked.Body.String())
	}

	upstreamStart := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", upstreamWorkOrderID), nil)
	if upstreamStart.Code != http.StatusOK {
		t.Fatalf("start typed product upstream status=%d body=%s", upstreamStart.Code, upstreamStart.Body.String())
	}
	var upstreamRunningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), upstreamWorkOrderID).Scan(&upstreamRunningItemID); err != nil {
		t.Fatalf("load typed upstream running item: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),actual_input_qty=12700,actual_output_qty=12700
		WHERE work_order_id=%d;
	`, schema, upstreamWorkOrderID))
	upstreamComplete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", upstreamWorkOrderID), map[string]any{
		"finished_units":   12_700,
		"finished_loose_g": 0,
		"consumed_input_g": 12_700,
		"warehouse":        "finished_goods",
		"note":             "typed product upstream completion",
	})
	if upstreamComplete.Code != http.StatusOK {
		t.Fatalf("complete typed product upstream status=%d body=%s", upstreamComplete.Code, upstreamComplete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf(
		"work_order_id=%d AND component_type='product' AND component_id=2 AND required_g=22700 AND reserved_g=22700 AND status='reserved'",
		rootWorkOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf(
		"work_order_id=%d AND component_type='product' AND component_id=2 AND material_batch_id=0 AND stock_batch_id>0 AND reserved_g>0 AND status='reserved'",
		rootWorkOrderID,
	), 2)

	downstreamStart := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
	if downstreamStart.Code != http.StatusOK {
		t.Fatalf("start typed downstream after upstream completion status=%d body=%s", downstreamStart.Code, downstreamStart.Body.String())
	}
	var rootRunningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), rootWorkOrderID).Scan(&rootRunningItemID); err != nil {
		t.Fatalf("load typed root running item: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),actual_input_qty=22.7,actual_output_qty=22.7
		WHERE work_order_id=%d;
	`, schema, rootWorkOrderID))
	downstreamComplete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", rootWorkOrderID), map[string]any{
		"finished_units":   100,
		"finished_loose_g": 0,
		"consumed_input_g": 22_700,
		"warehouse":        "finished_goods",
		"note":             "consume typed product batches",
	})
	if downstreamComplete.Code != http.StatusOK {
		t.Fatalf("complete typed downstream status=%d body=%s", downstreamComplete.Code, downstreamComplete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf(
		"work_order_id=%d AND component_type='product' AND component_id=2 AND consumed_g=22700 AND status='consumed'",
		rootWorkOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf(
		"work_order_id=%d AND component_type='product' AND component_id=2 AND consumed_g=reserved_g AND status='consumed'",
		rootWorkOrderID,
	), 2)
	assertProductionFlowCount(t, pool, schema, "stock_batches", "item_type='finished_product' AND item_id=2 AND remaining_g=0", 2)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf(
		"running_item_id=%d AND material_id=2 AND deduct_g>0",
		rootRunningItemID,
	), 2)
	var consumedTypedProductG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(deduct_g),0)::bigint
		FROM %s.material_consumption_logs
		WHERE running_item_id=$1 AND material_id=2
	`, schema), rootRunningItemID).Scan(&consumedTypedProductG); err != nil {
		t.Fatalf("sum typed product consumption: %v", err)
	}
	if consumedTypedProductG != 22_700 {
		t.Fatalf("typed product consumed g=%d, want 22700", consumedTypedProductG)
	}
}

func TestProductionPlanAPISharedUpstreamShortageAllocatesEachDependencyOnce(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedSharedUpstreamMaterialFlow(t, ctx, pool, schema, true)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 11_030, 30, "MB-SHARED-OPEN", "共用组件", 300)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 11_040, 40, "MB-SHARED-RAW", "共用原料", 700)

	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from":         "2026-08-01",
		"to":           "2026-08-31",
		"selected":     []string{"1-227"},
		"input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create shared-upstream plan status=%d body=%s", create.Code, create.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatalf("load shared-upstream plan id: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d", planID), 4)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(
		"production_plan_id=%d AND output_type='material' AND output_material_id=30 AND output_qty=0.7 AND output_unit='kg' AND planned_output_g=700",
		planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", fmt.Sprintf(
		"production_plan_id=%d AND component_type='material' AND component_id=30 AND required_g=300 AND production_plan_item_id IN (SELECT id FROM %s.production_plan_items WHERE production_plan_id=%d AND output_material_id=10)",
		planID, schema, planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", fmt.Sprintf(
		"production_plan_id=%d AND component_type='material' AND component_id=30 AND required_g=400 AND production_plan_item_id IN (SELECT id FROM %s.production_plan_items WHERE production_plan_id=%d AND output_material_id=20)",
		planID, schema, planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", fmt.Sprintf(
		"production_plan_id=%d AND component_type='material' AND component_id=30",
		planID,
	), 2)
	assertSharedManufacturingGraphCoverage(t, app, planID, 1_000, 300, 700, false)

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,
		       CASE WHEN output_type='product' THEN '包装' ELSE '烘焙' END,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15
		FROM %s.production_plan_items WHERE production_plan_id=%d;
	`, schema, schema, planID))
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit shared-upstream plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", planID), 4)
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies", "component_type='material' AND component_id=30 AND required_g=300", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies", "component_type='material' AND component_id=30 AND required_g=400", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies", "component_type='material' AND component_id=30", 2)
}

func TestProductionPlanAPISharedPurchaseLeafPersistsEachConsumerGap(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedSharedUpstreamMaterialFlow(t, ctx, pool, schema, false)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 12_030, 30, "MB-SHARED-PURCHASE", "共用组件", 300)

	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from":         "2026-08-01",
		"to":           "2026-08-31",
		"selected":     []string{"1-227"},
		"input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create shared-purchase plan status=%d body=%s", create.Code, create.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatalf("load shared-purchase plan id: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d", planID), 3)
	assertProductionFlowCount(t, pool, schema, "production_plan_supply_gaps", fmt.Sprintf(
		"production_plan_id=%d AND item_type='material' AND item_id=30 AND required_g=300 AND production_plan_item_id IN (SELECT id FROM %s.production_plan_items WHERE production_plan_id=%d AND output_material_id=10)",
		planID, schema, planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_supply_gaps", fmt.Sprintf(
		"production_plan_id=%d AND item_type='material' AND item_id=30 AND required_g=400 AND production_plan_item_id IN (SELECT id FROM %s.production_plan_items WHERE production_plan_id=%d AND output_material_id=20)",
		planID, schema, planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_supply_gaps", fmt.Sprintf(
		"production_plan_id=%d AND item_type='material' AND item_id=30 AND status='unresolved'",
		planID,
	), 2)
	assertSharedManufacturingGraphCoverage(t, app, planID, 1_000, 300, 700, true)

	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusBadRequest || !strings.Contains(submit.Body.String(), "采购/备料缺口") {
		t.Fatalf("submit shared-purchase plan status=%d body=%s, want blocker", submit.Code, submit.Body.String())
	}
}

func TestMultilevelSchemaMigratesLegacyMaterialUniqueKeysToTypedKeys(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		DROP INDEX %[1]s.production_plan_item_dependencies_typed_uq;
		ALTER TABLE %[1]s.production_plan_item_dependencies
			ADD CONSTRAINT legacy_plan_dependency_material_uq UNIQUE(production_plan_item_id,depends_on_plan_item_id,material_id);
		DROP INDEX %[1]s.work_order_dependencies_typed_uq;
		ALTER TABLE %[1]s.work_order_dependencies
			ADD CONSTRAINT legacy_work_dependency_material_uq UNIQUE(work_order_id,depends_on_work_order_id,material_id);
		DROP INDEX %[1]s.work_order_material_reservation_batches_typed_uq;
		ALTER TABLE %[1]s.work_order_material_reservation_batches
			ADD CONSTRAINT legacy_reservation_material_batch_uq UNIQUE(reservation_id,material_batch_id);
	`, schema))
	if err := postgresproduction.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("migrate legacy multilevel unique keys: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.production_plan_item_dependencies(
			production_plan_id,production_plan_item_id,depends_on_plan_item_id,material_id,
			component_type,component_id,component_spec_g,required_g
		) VALUES
			(1,10,20,0,'product',5,227,227),
			(1,10,20,0,'product',5,454,454);
		INSERT INTO %[1]s.work_order_dependencies(
			work_order_id,depends_on_work_order_id,material_id,
			component_type,component_id,component_spec_g,required_g
		) VALUES
			(30,40,0,'product',5,227,227),
			(30,40,0,'product',5,454,454);
		INSERT INTO %[1]s.work_order_material_reservation_batches(
			reservation_id,work_order_id,material_id,component_type,component_id,component_spec_g,
			material_batch_id,stock_batch_id,batch_code,reserved_g
		) VALUES
			(50,30,0,'product',5,227,0,60,'FP-BATCH-A',100),
			(50,30,0,'product',5,227,0,61,'FP-BATCH-B',127);
	`, schema))
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", "component_type='product' AND component_id=5", 2)
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies", "component_type='product' AND component_id=5", 2)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "component_type='product' AND component_id=5 AND material_batch_id=0", 2)
}

func assertSharedManufacturingGraphCoverage(t *testing.T, app http.Handler, planID, requiredG, coveredG, shortageG int64, blocking bool) {
	t.Helper()
	detail := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", planID), nil)
	if detail.Code != http.StatusOK {
		t.Fatalf("load shared manufacturing graph status=%d body=%s", detail.Code, detail.Body.String())
	}
	var payload struct {
		ManufacturingPlan struct {
			Blocking bool `json:"blocking"`
			Nodes    []struct {
				OutputType       string `json:"output_type"`
				OutputMaterialID int64  `json:"output_material_id"`
				RequiredG        int64  `json:"required_g"`
				StockCoveredG    int64  `json:"stock_covered_g"`
				ShortageG        int64  `json:"shortage_g"`
			} `json:"nodes"`
		} `json:"manufacturing_plan"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode shared manufacturing graph: %v body=%s", err, detail.Body.String())
	}
	found := false
	for _, node := range payload.ManufacturingPlan.Nodes {
		if node.OutputType == "material" && node.OutputMaterialID == 30 &&
			node.RequiredG == requiredG && node.StockCoveredG == coveredG && node.ShortageG == shortageG {
			found = true
			break
		}
	}
	if payload.ManufacturingPlan.Blocking != blocking || !found {
		t.Fatalf("shared manufacturing graph=%+v, want blocking=%v material 30 required=%d covered=%d shortage=%d", payload.ManufacturingPlan, blocking, requiredG, coveredG, shortageG)
	}
}

func TestProductionPlanAPIPersistsNoBOMSupplyGapAndBlocksSubmit(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 2020, 20, "MB-BAG-NO-BOM", "227g包装袋", 100)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		DELETE FROM %s.production_bom_output_bindings WHERE output_type='material' AND output_id=10;
		DELETE FROM %s.production_bom_version_items WHERE version_id=200;
		DELETE FROM %s.production_bom_versions WHERE id=200;
		DELETE FROM %s.production_boms WHERE id=200;
	`, schema, schema, schema, schema))

	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31",
		"selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create no-BOM production plan status=%d body=%s", create.Code, create.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatalf("load no-BOM production plan id: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_supply_gaps", fmt.Sprintf(
		"production_plan_id=%d AND item_type='material' AND item_id=10 AND required_g=22700 AND required_units=0 AND reason='no_default_material_bom' AND status='unresolved'",
		planID,
	), 1)
	detail := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", planID), nil)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"supply_gaps"`) || !strings.Contains(detail.Body.String(), `"required_g":22700`) ||
		!strings.Contains(detail.Body.String(), `"manufacturing_plan"`) || !strings.Contains(detail.Body.String(), `"action":"purchase"`) ||
		!strings.Contains(detail.Body.String(), `"blocking":true`) {
		t.Fatalf("production plan supply gap detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusBadRequest || !strings.Contains(submit.Body.String(), "采购/备料缺口") {
		t.Fatalf("submit no-BOM production plan status=%d body=%s, want unresolved supply gap rejection", submit.Code, submit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", planID), 0)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", "1=1", 0)
}

func TestProductionPlanAPIFullStockCreatesNoUpstreamAndReservesOnceOnSubmit(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "MB-ROASTED-FULL", "在制熟豆", 22_700)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 3020, 20, "MB-BAG-FULL", "227g包装袋", 100)

	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31",
		"selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create full-stock production plan status=%d body=%s", create.Code, create.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatalf("load full-stock production plan id: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d", planID), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", fmt.Sprintf("production_plan_id=%d", planID), 0)
	assertProductionFlowCount(t, pool, schema, "production_plan_supply_gaps", fmt.Sprintf("production_plan_id=%d", planID), 0)
	detail := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", planID), nil)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"output_material_id":10`) ||
		!strings.Contains(detail.Body.String(), `"stock_covered_g":22700`) ||
		!strings.Contains(detail.Body.String(), `"shortage_g":0`) ||
		!strings.Contains(detail.Body.String(), `"action":"inventory"`) {
		t.Fatalf("full-stock manufacturing graph status=%d body=%s", detail.Code, detail.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,'包装',planned_g/1000.0,inventory_unit,15,1,planned_g/1000.0,planned_g,15
		FROM %s.production_plan_items WHERE production_plan_id=%d;
	`, schema, schema, planID))
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit full-stock production plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", planID), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies", "1=1", 0)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", "material_id=10 AND required_g=22700 AND reserved_g=22700 AND status='reserved'", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", "material_id=20 AND required_units=100 AND reserved_units=100 AND status='reserved'", 1)
}

func TestProductionPlanAPIConcurrentSubmitRechecksSharedMaterialAvailability(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 4010, 10, "MB-ROASTED-CONCURRENT", "在制熟豆", 10_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 4030, 30, "MB-GREEN-CONCURRENT", "生产生豆", 40_000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 4020, 20, "MB-BAG-CONCURRENT", "227g包装袋", 200)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (2,'第二款227g包装熟豆',50,true,'227g',227,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(2,'SO-ML-227-B','2026-08-10',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (2,1,'第二款227g包装熟豆',100,'袋','227g',2,50,5000);
		INSERT INTO %s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status)
		VALUES (101,'PBOM-PACK-227-B','第二款227g包装 BOM','product',2,0,'active');
		INSERT INTO %s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at)
		VALUES (101,101,'V001','published',1,0,1,'kg',31,now());
		INSERT INTO %s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES
			(101,10,'material','g_per_bag',227,0),
			(101,20,'material','unit_per_bag',1,0);
		INSERT INTO %s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		VALUES (2,101,101,'test');
		INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by)
		VALUES ('product',2,101,101,true,'test');
		INSERT INTO %s.product_production_configs(product_id,production_bom_id,production_bom_version_id,process_route_id,expected_loss_rate,created_by,updated_by)
		VALUES (2,101,101,31,0,'test','test');
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	app := newProductionFlowTestEcho(pool, schema)
	createPlan := func(productID int64) int64 {
		key := fmt.Sprintf("%d-227", productID)
		rec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
			"from": "2026-08-01", "to": "2026-08-31",
			"selected": []string{key}, "input_by_key": map[string]int64{key: 22_700},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("create concurrent plan product=%d status=%d body=%s", productID, rec.Code, rec.Body.String())
		}
		var planID int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
			t.Fatalf("load concurrent plan id: %v", err)
		}
		return planID
	}
	planA := createPlan(1)
	planB := createPlan(2)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,
		       CASE WHEN output_type='material' THEN '烘焙' ELSE '包装' END,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15
		FROM %s.production_plan_items WHERE production_plan_id IN (%d,%d);
	`, schema, schema, planA, planB))

	type submitResult struct {
		code int
		body string
	}
	start := make(chan struct{})
	results := make(chan submitResult, 2)
	for _, planID := range []int64{planA, planB} {
		go func(id int64) {
			<-start
			rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", id), nil)
			results <- submitResult{code: rec.Code, body: rec.Body.String()}
		}(planID)
	}
	close(start)
	first, second := <-results, <-results
	successes, staleFailures := 0, 0
	for _, result := range []submitResult{first, second} {
		if result.code == http.StatusOK {
			successes++
		}
		if result.code == http.StatusBadRequest && strings.Contains(result.body, "库存可用量已变化") {
			staleFailures++
		}
	}
	if successes != 1 || staleFailures != 1 {
		t.Fatalf("concurrent submit results=%+v/%+v, want one success and one stale-availability rejection", first, second)
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id IN (%d,%d) AND status='submitted'", planA, planB), 1)
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id IN (%d,%d) AND status='draft'", planA, planB), 1)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id IN (%d,%d)", planA, planB), 2)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", "material_id=10 AND required_g=22700 AND reserved_g=10000 AND status='reserved'", 1)
}

func TestReleasedTypedWorkOrderCancelReleasesSubmitReservation(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, workOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 5010)
	cancel := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/cancel", workOrderID), map[string]any{"note": "取消未开工工单"})
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel released typed work order status=%d body=%s", cancel.Code, cancel.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='cancelled'", workOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf(
		"work_order_id=%d AND material_id=10 AND status='released' AND returned_g=reserved_g AND returned_g=22700",
		workOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf(
		"work_order_id=%d AND material_id=20 AND status='released' AND returned_units=reserved_units AND returned_units=100",
		workOrderID,
	), 1)
}

func TestSubmittedUnstartedTypedPlanCancelReleasesAllReservations(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, planID, _ := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 6010)
	cancel := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/cancel", planID), map[string]any{"note": "取消未开工计划"})
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel submitted unstarted typed plan status=%d body=%s", cancel.Code, cancel.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id=%d AND status='cancelled'", planID), 1)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d AND status='cancelled'", planID), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", "status='reserved'", 0)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", "status='released' AND returned_g=reserved_g AND returned_units=reserved_units", 2)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='cancel'", planID), 1)
}

func TestDraftPlanItemTargetWarehouseUpdateFreezesOnSubmit(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 7010, 10, "MB-ROASTED-TARGET", "在制熟豆", 22_700)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 7020, 20, "MB-BAG-TARGET", "227g包装袋", 100)
	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31",
		"selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create target-warehouse plan status=%d body=%s", create.Code, create.Body.String())
	}
	var planID, itemID int64
	var defaultWarehouse string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT production_plan_id,id,target_warehouse
		FROM %s.production_plan_items ORDER BY id DESC LIMIT 1
	`, schema)).Scan(&planID, &itemID, &defaultWarehouse); err != nil {
		t.Fatalf("load target-warehouse plan item: %v", err)
	}
	if defaultWarehouse != "finished_goods" {
		t.Fatalf("product plan target warehouse=%q, want finished_goods", defaultWarehouse)
	}
	updatePath := fmt.Sprintf("/api/production-plans/%d/items/%d/target-warehouse", planID, itemID)
	update := serveMultilevelProductionJSON(t, app, http.MethodPatch, updatePath, map[string]any{"target_warehouse": "finished_shop"})
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), `"target_warehouse":"finished_shop"`) {
		t.Fatalf("update draft target warehouse status=%d body=%s", update.Code, update.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf(
		"entity_type='production_plan_item' AND entity_id=%d AND action='update_target_warehouse'", itemID,
	), 1)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		) SELECT %d,%d,1,'包装',planned_g/1000.0,inventory_unit,15,1,planned_g/1000.0,planned_g,15
		  FROM %s.production_plan_items WHERE id=%d;
		`, schema, planID, itemID, schema, itemID))
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit target-warehouse plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf(
		"production_plan_id=%d AND production_plan_item_id=%d AND target_warehouse='finished_shop'", planID, itemID,
	), 1)
	locked := serveMultilevelProductionJSON(t, app, http.MethodPatch, updatePath, map[string]any{"target_warehouse": "finished_goods"})
	if locked.Code != http.StatusBadRequest || !strings.Contains(locked.Body.String(), "草稿") {
		t.Fatalf("update submitted target warehouse status=%d body=%s, want draft-only rejection", locked.Code, locked.Body.String())
	}
}

func TestMaterialOutputCompletionHonorsFrozenPlanTargetWarehouse(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 8010, 10, "MB-ROASTED-TARGET-MATERIAL", "在制熟豆", 10_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 8030, 30, "MB-GREEN-TARGET-MATERIAL", "生产生豆", 20_000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 8020, 20, "MB-BAG-TARGET-MATERIAL", "227g包装袋", 100)
	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31",
		"selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create material target plan status=%d body=%s", create.Code, create.Body.String())
	}
	var planID, materialItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT production_plan_id,id FROM %s.production_plan_items
		WHERE output_type='material' AND output_material_id=10 ORDER BY id DESC LIMIT 1
	`, schema)).Scan(&planID, &materialItemID); err != nil {
		t.Fatalf("load material target plan item: %v", err)
	}
	update := serveMultilevelProductionJSON(t, app, http.MethodPatch,
		fmt.Sprintf("/api/production-plans/%d/items/%d/target-warehouse", planID, materialItemID),
		map[string]any{"target_warehouse": "finished_shop"})
	if update.Code != http.StatusOK {
		t.Fatalf("update material target warehouse status=%d body=%s", update.Code, update.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,
		       CASE WHEN output_type='material' THEN '烘焙' ELSE '包装' END,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,
		       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15
		FROM %s.production_plan_items WHERE production_plan_id=%d;
	`, schema, schema, planID))
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit material target plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	var workOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND output_type='material' AND output_material_id=10
	`, schema), planID).Scan(&workOrderID); err != nil {
		t.Fatalf("load material target work order: %v", err)
	}
	start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", workOrderID), nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start material target work order status=%d body=%s", start.Code, start.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),actual_input_qty=15.875,actual_output_qty=12.7
		WHERE work_order_id=%d;
	`, schema, workOrderID))
	wrong := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrderID), map[string]any{
		"finished_qty_g": 12_700, "consumed_input_g": 15_875, "warehouse": "wip",
	})
	if wrong.Code != http.StatusBadRequest || !strings.Contains(wrong.Body.String(), "frozen") {
		t.Fatalf("replace frozen material target status=%d body=%s, want rejection", wrong.Code, wrong.Body.String())
	}
	complete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrderID), map[string]any{
		"finished_qty_g": 12_700, "consumed_input_g": 15_875, "warehouse": "finished_shop",
	})
	if complete.Code != http.StatusOK {
		t.Fatalf("complete into frozen material target status=%d body=%s", complete.Code, complete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_id=10 AND warehouse='finished_shop' AND qty_g=12700", 1)
}

func TestMaterialOutputShortAndOverProductionKeepDownstreamCoverageExact(t *testing.T) {
	tests := []struct {
		name               string
		finishedG          int64
		wantReservedG      int64
		wantBoundG         int64
		wantDownstreamCode int
		wantOnhandG        int64
	}{
		{name: "short output remains blocked", finishedG: 12_000, wantReservedG: 22_000, wantBoundG: 12_000, wantDownstreamCode: http.StatusBadRequest, wantOnhandG: 22_000},
		{name: "over output leaves surplus free", finishedG: 13_000, wantReservedG: 22_700, wantBoundG: 12_700, wantDownstreamCode: http.StatusOK, wantOnhandG: 23_000},
	}
	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
			base := int64(9000 + index*100)
			seedProductionFlowWIPBatch(t, ctx, pool, schema, base+10, 10, fmt.Sprintf("MB-ROASTED-%d", base), "在制熟豆", 10_000)
			seedProductionFlowWIPBatch(t, ctx, pool, schema, base+30, 30, fmt.Sprintf("MB-GREEN-%d", base), "生产生豆", 20_000)
			seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, base+20, 20, fmt.Sprintf("MB-BAG-%d", base), "227g包装袋", 100)
			app := newProductionFlowTestEcho(pool, schema)
			create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
				"from": "2026-08-01", "to": "2026-08-31",
				"selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22_700},
			})
			if create.Code != http.StatusOK {
				t.Fatalf("create variance plan status=%d body=%s", create.Code, create.Body.String())
			}
			var planID int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
				t.Fatalf("load variance plan id: %v", err)
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
				INSERT INTO %s.production_plan_operation_splits(
					production_plan_id,production_plan_item_id,operation_seq,operation,
					batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
					planned_qty,planned_qty_g,planned_minutes
				)
				SELECT production_plan_id,id,1,
				       CASE WHEN output_type='material' THEN '烘焙' ELSE '包装' END,
				       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,
				       CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15
				FROM %s.production_plan_items WHERE production_plan_id=%d;
			`, schema, schema, planID))
			submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
			if submit.Code != http.StatusOK {
				t.Fatalf("submit variance plan status=%d body=%s", submit.Code, submit.Body.String())
			}
			var rootWorkOrderID, upstreamWorkOrderID int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1 AND output_type='product'`, schema), planID).Scan(&rootWorkOrderID); err != nil {
				t.Fatalf("load variance root work order: %v", err)
			}
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1 AND output_type='material'`, schema), planID).Scan(&upstreamWorkOrderID); err != nil {
				t.Fatalf("load variance upstream work order: %v", err)
			}
			start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", upstreamWorkOrderID), nil)
			if start.Code != http.StatusOK {
				t.Fatalf("start variance upstream status=%d body=%s", start.Code, start.Body.String())
			}
			consumedInputG := int64(math.Ceil(float64(tt.finishedG) * 1.25))
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
				UPDATE %s.job_cards
					SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),actual_input_qty=%g,actual_output_qty=%g
					WHERE work_order_id=%d;
				`, schema, float64(consumedInputG)/1000, float64(tt.finishedG)/1000, upstreamWorkOrderID))
			complete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", upstreamWorkOrderID), map[string]any{
				"finished_qty_g": tt.finishedG, "consumed_input_g": consumedInputG, "warehouse": "wip",
			})
			if complete.Code != http.StatusOK {
				t.Fatalf("complete variance upstream status=%d body=%s", complete.Code, complete.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf(
				"work_order_id=%d AND material_id=10 AND required_g=22700 AND reserved_g=%d", rootWorkOrderID, tt.wantReservedG,
			), 1)
			assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf(
				"work_order_id=%d AND material_id=10 AND reserved_g=%d", rootWorkOrderID, tt.wantBoundG,
			), 1)
			assertProductionFlowCount(t, pool, schema, "materials", fmt.Sprintf("id=10 AND onhand_g=%d", tt.wantOnhandG), 1)
			downstream := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
			if downstream.Code != tt.wantDownstreamCode {
				t.Fatalf("start downstream after variance status=%d body=%s, want %d", downstream.Code, downstream.Body.String(), tt.wantDownstreamCode)
			}
			if tt.finishedG > 12_700 {
				assertProductionFlowCount(t, pool, schema, "material_batches", fmt.Sprintf(
					"material_id=10 AND received_g=%d AND remaining_g=%d", tt.finishedG, tt.finishedG,
				), 1)
			}
		})
	}
}

func TestCountOnlyMaterialOutputWritesActualPerUnitBatchCost(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	const runningItemID int64 = 9901
	const workOrderID int64 = 9902
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES
			(40,'COUNT-OUTPUT','计件产出','other','unit','unit',0,0,0,0),
			(41,'COUNT-INPUT','计件投入','other','unit','unit',0,12,2,0);
		INSERT INTO %s.produce_running_items(
			id,batch_id,product_id,product_name,spec_g,need_g,status,started_by,started_at,
			input_g,planned_units,planned_loose_g,material_snapshot,
			output_type,output_product_id,output_material_id,output_name,output_qty,output_unit,target_warehouse
		) VALUES(
			%d,'RUN-COUNT',0,'计件产出',0,0,'running','test',now(),
			0,60,0,
			'[{
			  "material_id":41,"material_name":"计件投入","unit":"unit","source":"bom",
			  "component_type":"material","consume_unit":"unit","qty_per_unit":2,
			  "output_qty":10,"output_unit":"unit"
			}]'::jsonb,
			'material',0,40,'计件产出',60,'unit','wip'
		);
		INSERT INTO %s.work_orders(
			id,work_order_no,running_item_id,batch_id,product_id,product_name,status,
			material_snapshot,output_type,output_product_id,output_material_id,output_name,output_qty,output_unit,target_warehouse
		) SELECT %d,'WO-COUNT-OUTPUT',id,batch_id,0,'计件产出','running',material_snapshot,
		         'material',0,40,'计件产出',60,'unit','wip'
		  FROM %s.produce_running_items WHERE id=%d;
		INSERT INTO %s.job_cards(
			work_order_id,sequence_no,operation,status,planned_operation_cost,started_at,completed_at
		) VALUES(%d,1,'计件加工','completed',6,now(),now());
	`, schema, schema, runningItemID, schema, workOrderID, schema, runningItemID, schema, workOrderID))
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 9941, 41, "MB-COUNT-INPUT", "计件投入", 12)
	app := newProductionFlowTestEcho(pool, schema)
	complete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrderID), map[string]any{
		"finished_qty_units": 60,
		"warehouse":          "wip",
		"note":               "计件产出单位成本",
	})
	if complete.Code != http.StatusOK {
		t.Fatalf("complete count-only material output status=%d body=%s", complete.Code, complete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "materials", "id=40 AND onhand_units=60", 1)
	assertProductionFlowCount(t, pool, schema, "materials", "id=41 AND onhand_units=0", 1)
	assertProductionFlowCount(t, pool, schema, "material_batches", "material_id=40 AND qty_units=60 AND remaining_units=60 AND unit_cost=0.5", 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches", "item_type='material' AND item_id=40 AND qty_units=60 AND remaining_units=60 AND unit_cost=0.5", 1)
	assertProductionFlowCount(t, pool, schema, "production_batch_costs", fmt.Sprintf("running_item_id=%d AND material_cost=24 AND operation_cost=6 AND total_cost=30", runningItemID), 1)
}

func createSubmittedFullStockTypedPlan(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, batchID int64) (*echo.Echo, int64, int64) {
	t.Helper()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, batchID, 10, fmt.Sprintf("MB-ROASTED-%d", batchID), "在制熟豆", 22_700)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, batchID+1, 20, fmt.Sprintf("MB-BAG-%d", batchID), "227g包装袋", 100)
	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31",
		"selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create typed plan status=%d body=%s", create.Code, create.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatalf("load typed plan id: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,'包装',planned_g/1000.0,inventory_unit,15,1,planned_g/1000.0,planned_g,15
		FROM %s.production_plan_items WHERE production_plan_id=%d;
	`, schema, schema, planID))
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit typed plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	var workOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1`, schema), planID).Scan(&workOrderID); err != nil {
		t.Fatalf("load typed work order id: %v", err)
	}
	return app, planID, workOrderID
}

func serveMultilevelProductionJSON(t *testing.T, app http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload := []byte{}
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func seedMultilevelMaterialOutputFlow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		ALTER TABLE %s.customer_processing_production_demands
		ADD COLUMN IF NOT EXISTS request_item_id BIGINT NOT NULL DEFAULT 0;
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (1,'227g包装熟豆',50,true,'227g',227,'g','{"inventory_unit":"kg"}'::jsonb);
		INSERT INTO %s.order_process_statuses(name,sort,active) VALUES
			('待处理',10,true),
			('生产中',20,true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id) VALUES
			(1,'SO-ML-227','2026-08-10',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (1,1,'227g包装熟豆',100,'袋','227g',1,50,5000);

		INSERT INTO %s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES
			(10,'ROASTED-WIP','在制熟豆','bean','kg','kg',10000,0,80,0),
			(20,'BAG-227','227g包装袋','pack','unit','unit',0,100,1,0),
			(30,'GREEN-INPUT','生产生豆','bean','kg','kg',20000,0,50,0);

		INSERT INTO %s.process_routes(id,name,status,default_equipment,default_minutes) VALUES
			(31,'包装路线','active','包装机',15),
			(32,'烘焙路线','active','烘焙机',30);
		INSERT INTO %s.process_route_operations(route_id,seq,operation,workstation,default_equipment,default_minutes,records_loss) VALUES
			(31,1,'包装','包装台','包装机',15,false),
			(32,1,'烘焙','烘焙中心','烘焙机',30,true);

		INSERT INTO %s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status) VALUES
			(100,'PBOM-PACK-227','227g包装 BOM','product',1,0,'active'),
			(200,'PBOM-ROASTED','熟豆生产 BOM','material',0,10,'active');
		INSERT INTO %s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at) VALUES
			(100,100,'V001','published',1,0,1,'kg',31,now()),
			(200,200,'V001','published',1,0,1,'kg',32,now());
		INSERT INTO %s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES
			(100,10,'material','g_per_bag',227,0),
			(100,20,'material','unit_per_bag',1,0),
			(200,30,'material','g',1250,0);
		INSERT INTO %s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		VALUES (1,100,100,'test');
		INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by) VALUES
			('product',1,100,100,true,'test'),
			('material',10,200,200,true,'test');
		INSERT INTO %s.product_production_configs(product_id,production_bom_id,production_bom_version_id,process_route_id,expected_loss_rate,created_by,updated_by)
		VALUES (1,100,100,31,0,'test','test');
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))
}

func seedTypedProductComponentFlow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.products(
			id,name,default_price,active,spec_label,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES (2,'散装熟豆中间品',40,true,'1g',1,'g','{"inventory_unit":"g"}'::jsonb);

		DELETE FROM %s.production_bom_version_items WHERE version_id=100;
		INSERT INTO %s.production_bom_version_items(
			version_id,material_id,component_type,component_product_id,component_spec_g,
			consume_unit,qty_per_unit,ratio_pct
		) VALUES (100,0,'finished_product',2,1,'g_per_bag',227,0);

		INSERT INTO %s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status)
		VALUES (300,'PBOM-TYPED-ROASTED','散装熟豆中间品 BOM','product',2,0,'active');
		INSERT INTO %s.production_bom_versions(
			id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at
		) VALUES (300,300,'V001','published',1,0,1000,'g',32,now());
		INSERT INTO %s.production_bom_version_items(
			version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct
		) VALUES (300,30,'material','g',1000,0);
		INSERT INTO %s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
		VALUES (2,300,300,'test');
		INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by)
		VALUES ('product',2,300,300,true,'test');
		INSERT INTO %s.product_production_configs(
			product_id,production_bom_id,production_bom_version_id,process_route_id,expected_loss_rate,created_by,updated_by
		) VALUES (2,300,300,32,0,'test','test');

		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES (2,1,'finished_goods',10000,0);
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,
			qty_g,qty_units,operator,remaining_g,remaining_units,unit_cost,quality_status
		) VALUES ('FP-TYPED-ROASTED','finished_product',2,'散装熟豆中间品',1,'',0,'',10000,10000,'test',10000,10000,80,'pass');
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))
}

func seedSharedUpstreamMaterialFlow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, withSharedBOM bool) {
	t.Helper()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.materials SET code='INTERMEDIATE-A',name='半成品A',kind='other',unit='kg',cost_unit='kg',onhand_g=0,onhand_units=0 WHERE id=10;
		UPDATE %[1]s.materials SET code='INTERMEDIATE-B',name='半成品B',kind='other',unit='kg',cost_unit='kg',onhand_g=0,onhand_units=0 WHERE id=20;
		UPDATE %[1]s.materials SET code='SHARED-COMPONENT',name='共用组件',kind='other',unit='kg',cost_unit='kg',onhand_g=300,onhand_units=0 WHERE id=30;
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price)
		VALUES (40,'SHARED-RAW','共用原料','other','kg','kg',700,0,10,0);

		DELETE FROM %[1]s.production_bom_version_items WHERE version_id IN (100,200);
		UPDATE %[1]s.production_bom_versions SET output_qty=0.001,output_unit='kg',material_loss_rate=0 WHERE id=200;
		UPDATE %[1]s.production_boms SET name='半成品A BOM',output_type='material',output_product_id=0,output_material_id=10 WHERE id=200;
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES
			(100,10,'material','g_per_bag',1,0),
			(100,20,'material','g_per_bag',1,0),
			(200,30,'material','g',6,0);

		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status)
		VALUES (201,'PBOM-INTERMEDIATE-B','半成品B BOM','material',0,20,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at)
		VALUES (201,201,'V001','published',1,0,0.001,'kg',32,now());
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct)
		VALUES (201,30,'material','g',4,0);
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by)
		VALUES ('material',20,201,201,true,'test');
	`, schema))
	if !withSharedBOM {
		return
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status)
		VALUES (300,'PBOM-SHARED','共用组件 BOM','material',0,30,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at)
		VALUES (300,300,'V001','published',1,0,0.001,'kg',32,now());
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct)
		VALUES (300,40,'material','g',1,0);
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by)
		VALUES ('material',30,300,300,true,'test');
	`, schema))
}

func seedProductionFlowWIPUnitBatch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, batchID, materialID int64, batchCode, materialName string, qtyUnits int64) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batches(id,batch_code,material_id,material_name,received_g,qty_g,qty_units,remaining_g,remaining_units,unit_cost,status,quality_status)
		VALUES($1,$2,$3,$4,0,0,$5,0,$5,0,'active','pass')
	`, schema), batchID, batchCode, materialID, materialName, qtyUnits); err != nil {
		t.Fatalf("seed WIP unit material batch: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units)
		VALUES($1,$2,$3,'wip',0,$4)
	`, schema), batchID, batchCode, materialID, qtyUnits); err != nil {
		t.Fatalf("seed WIP unit material location: %v", err)
	}
}
