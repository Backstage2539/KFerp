package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	pgprod "orderapp/internal/infrastructure/postgres/production"
	pgstock "orderapp/internal/infrastructure/postgres/stock"
)

func TestDirectTransferTwoOrdersShareSelectedBatch(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id) VALUES (2,'SO-ML-228','2026-08-10',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1));
 INSERT INTO %[1]s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total) VALUES (2,1,'227g包装熟豆',100,'袋','227g',1,50,5000);`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "SHARED-SEMI", "在制熟豆", 45400)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 1020, 20, "PACKAGING", "227g包装袋", 1000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=3010`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	var plan app.ProductionPlanDetail
	if created.Code != 200 || json.Unmarshal(created.Body.Bytes(), &plan) != nil {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	t.Logf("created plan items=%d", len(plan.Items))
	endpoint := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID)
	choices := []any{map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "raw_materials", "qty": 45.4}}
	check := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, map[string]any{"draft_token": plan.DraftToken, "check_only": true, "selections": choices})
	var checked app.SemiFinishedStockResult
	if check.Code != 200 || json.Unmarshal(check.Body.Bytes(), &checked) != nil || checked.PreviewToken == "" {
		t.Fatalf("check: %d %s", check.Code, check.Body.String())
	}
	saved := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, map[string]any{"draft_token": plan.DraftToken, "check_only": false, "preview_token": checked.PreviewToken, "selections": choices})
	if saved.Code != 200 {
		t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	get := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if get.Code != 200 || json.Unmarshal(get.Body.Bytes(), &plan) != nil {
		t.Fatalf("get: %d %s", get.Code, get.Body.String())
	}
	orders := map[int64]float64{}
	for _, item := range plan.Items {
		for _, demand := range item.DemandSources {
			orders[demand.OrderID] += demand.Quantity
		}
	}
	if orders[1] != 100 || orders[2] != 100 || len(orders) != 2 {
		t.Fatalf("missing or duplicated orders: %+v", orders)
	}
	transfer := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint+"/transfer-to-wip", map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "two-order-shared"})
	var tr app.PlanMaterialTransferResult
	if transfer.Code != 200 || json.Unmarshal(transfer.Body.Bytes(), &tr) != nil || len(tr.Documents) != 1 || tr.Plan == nil {
		t.Fatalf("transfer: %d %s", transfer.Code, transfer.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,CASE WHEN output_type='material' THEN '烘焙' ELSE '包装' END,CASE WHEN inventory_unit='kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,CASE WHEN inventory_unit='kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, plan.ID))
	submitted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submitted.Code != 200 {
		t.Fatalf("submit: %d %s", submitted.Code, submitted.Body.String())
	}
	var count, total int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*),COALESCE(SUM(reserved_g),0) FROM %s.work_order_material_reservation_batches WHERE material_batch_id=3010 AND warehouse='wip' AND status='reserved'`, schema)).Scan(&count, &total); err != nil {
		t.Fatal(err)
	}
	if count != 1 || total != 45400 {
		t.Fatalf("want one shared reservation totaling 45400 g, got %d/%d", count, total)
	}
}
