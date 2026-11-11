package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	pgprod "orderapp/internal/infrastructure/postgres/production"
	pgstock "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"testing"
)

func TestRecalculateDraftProtectsSemiFinishedSelectionAndRefreshesArchive(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedProductionPlanLifecycleData(t, ctx, pool, schema)
	e := newProductionFlowTestEcho(pool, schema)
	created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "erp_order", "selected": []string{"1-227"}})
	if created.Code != 200 {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	var p app.ProductionPlanDetail
	if err := json.Unmarshal(created.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g,qty_units) VALUES(%[2]d,'material:999',999,'wip',999,100,0)`, schema, p.ID))
	body := map[string]any{"selected": []string{"1-227"}, "revision": p.Revision, "request_id": "change-with-selection"}
	blocked := serveMultilevelProductionJSON(t, e, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d", p.ID), body)
	if blocked.Code == 200 || !strings.Contains(blocked.Body.String(), "半成品") {
		t.Fatalf("must require clearing selection: %d %s", blocked.Code, blocked.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_semi_finished_selections", fmt.Sprintf("production_plan_id=%d", p.ID), 1)
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id=%d AND revision=%d", p.ID, p.Revision), 1)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %[1]s.production_plan_semi_finished_selections WHERE production_plan_id=%[2]d;
 INSERT INTO %[1]s.production_plan_frozen_manufacturing_outputs(production_plan_id,item_key,item_snapshot) VALUES(%[2]d,'stale-snapshot','{}'::jsonb)`, schema, p.ID))
	body["request_id"] = "change-without-selection"
	updated := serveMultilevelProductionJSON(t, e, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d", p.ID), body)
	if updated.Code != 200 {
		t.Fatalf("recalculate without selection: %d %s", updated.Code, updated.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_frozen_manufacturing_outputs", fmt.Sprintf("production_plan_id=%d AND item_key='stale-snapshot'", p.ID), 0)
}

func TestRecalculateDraftRequiresPendingTransferHandled(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "PENDING-RECALC", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10; UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=1030`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	body := map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}}
	created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", body)
	if created.Code != 200 {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	var p app.ProductionPlanDetail
	if err := json.Unmarshal(created.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.ComponentSources) != 1 {
		t.Fatalf("sources %+v", p.ComponentSources)
	}
	transfer := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", p.ID), map[string]any{"draft_token": p.DraftToken, "idempotency_key": "recalc-transfer", "items": []any{map[string]any{"source_id": p.ComponentSources[0].ID, "warehouse": "raw_materials", "batches": []any{map[string]any{"batch_code": "PENDING-RECALC", "qty_g": 1000}}}}})
	if transfer.Code != 200 {
		t.Fatalf("transfer %d %s", transfer.Code, transfer.Body.String())
	}
	body["revision"] = p.Revision
	body["request_id"] = "recalc-pending"
	blocked := serveMultilevelProductionJSON(t, e, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d", p.ID), body)
	if blocked.Code == 200 || !strings.Contains(blocked.Body.String(), "调拨") {
		t.Fatalf("must reject pending transfer: %d %s", blocked.Code, blocked.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id=%d AND revision=%d", p.ID, p.Revision), 1)
}
