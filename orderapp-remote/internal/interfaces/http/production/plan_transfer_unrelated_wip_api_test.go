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

// A plan can have an unrelated packaging reservation which needs separate
// reconciliation. It must not block a valid raw-bean transfer posting.
func TestPlanTransferPostingIgnoresUnrelatedWIPReservation(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "RAW-VALID", "生产生豆", 150000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.materials SET onhand_g=150000 WHERE id=30;
 UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=1030;`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 52.307, "target_warehouse": "wip"}}})
	var plan app.ProductionPlanDetail
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &plan) != nil || len(plan.ComponentSources) != 1 {
		t.Fatalf("create plan %d %s", rec.Code, rec.Body.String())
	}
	src := plan.ComponentSources[0]
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "unrelated-wip", "items": []any{map[string]any{"source_id": src.ID, "warehouse": "raw_materials", "batches": []any{map[string]any{"batch_code": "RAW-VALID", "qty_g": 120000}}}}})
	var transfer app.PlanMaterialTransferResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &transfer) != nil || len(transfer.Documents) != 1 {
		t.Fatalf("create transfer %d %s", rec.Code, rec.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES(31,'UNRELATED-BAG','无关包装袋','packaging','袋','袋',0,423,1,0);
 INSERT INTO %[1]s.material_batches(id,batch_code,material_id,material_name,qty_units,remaining_units,unit_cost,status,quality_status) VALUES(2031,'UNRELATED-WIP',31,'无关包装袋',423,423,0,'active','pass');
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_units) VALUES(2031,'UNRELATED-WIP',31,'wip',423);
 INSERT INTO %[1]s.work_order_material_reservations(work_order_id,material_id,reserved_units,source_warehouse) VALUES(999,31,1,'wip');
 INSERT INTO %[1]s.production_plan_component_sources(production_plan_id,production_plan_item_id,component_id,component_name,unit,required_units) VALUES(%[2]d,%[3]d,31,'无关包装袋','袋',80);`, schema, plan.ID, src.ProductionPlanItemID))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", transfer.Documents[0].ID), map[string]any{})
	if rec.Code != 200 {
		t.Fatalf("unrelated WIP reservation blocked bean transfer: %d %s", rec.Code, rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=120000", 1)
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=2031 AND warehouse='wip' AND qty_units=423", 1)
}
