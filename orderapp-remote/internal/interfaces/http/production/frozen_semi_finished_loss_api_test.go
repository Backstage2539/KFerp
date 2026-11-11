package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	app "orderapp/internal/application/production"
	"testing"
)

func TestFrozenSemiFinishedLossRecalculatedFromOutput(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;UPDATE %[1]s.production_bom_version_items SET qty_per_unit=601.12 WHERE version_id=100 AND material_id=10;
 UPDATE %[1]s.production_bom_versions SET material_loss_rate=0.195 WHERE id=200;
 DELETE FROM %[1]s.production_bom_version_items WHERE version_id=200;
 INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES (31,'RAW31','原料31','bean','kg','kg',0,0,50,0),(32,'RAW32','原料32','bean','kg','kg',0,0,50,0),(33,'RAW33','原料33','bean','kg','kg',0,0,50,0);
 INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct,material_loss_rate) VALUES (200,30,'material','ratio_pct',0,15,0.195),(200,31,'material','ratio_pct',0,15,0.195),(200,32,'material','ratio_pct',0,20,0.195),(200,33,'material','ratio_pct',0,50,0.195)`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "SIXTY-KG-SEMI", "在制熟豆", 15000)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID)
	check := func(want float64) {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, http.MethodGet, url, nil)
		var got app.SemiFinishedStockResult
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &got) != nil || len(got.Items) != 1 {
			t.Fatalf("read: %s", r.Body.String())
		}
		var planned, output int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT planned_g,planned_output_g FROM %s.production_plan_items WHERE production_plan_id=$1 AND output_type='material' AND output_material_id=10`, schema), plan.ID).Scan(&planned, &output); err != nil {
			t.Fatal(err)
		}
		wantG := int64(math.Round(want * 1000))
		var correct int64
		for _, ratio := range []float64{.15, .15, .20, .50} {
			correct += int64(math.Ceil(float64(wantG) * ratio / .805))
		}
		if output != wantG || planned != correct {
			t.Fatalf("output=%d planned=%d want=%d", output, planned, correct)
		}
		if math.Abs(got.Items[0].RequiredQty-60.112) > 0.000001 || math.Abs(got.Items[0].ManufactureQty-want) > 0.000001 {
			t.Fatalf("expected 60.112 / %v: %s", want, r.Body.String())
		}
	}
	check(60.112)
	previous := 60.112
	for _, step := range []struct{ qty, want float64 }{{10, 50.112}, {15, 45.112}, {0, 60.112}} {
		choices := []any{}
		if step.qty > 0 {
			choices = append(choices, map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": step.qty})
		}
		body := map[string]any{"draft_token": plan.DraftToken, "check_only": true, "selections": choices}
		r := serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
		var preview app.SemiFinishedStockResult
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &preview) != nil || preview.PreviewToken == "" {
			t.Fatalf("preview: %d %s", r.Code, r.Body.String())
		}
		check(previous) // Preview is read-only and leaves the frozen draft unchanged.
		wantG := int64(math.Round(step.want * 1000))
		var correct int64
		for _, ratio := range []float64{.15, .15, .20, .50} {
			correct += int64(math.Ceil(float64(wantG) * ratio / .805))
		}
		if len(preview.Changes) != 1 || preview.Changes[0].AfterInputQty == nil || math.Abs(*preview.Changes[0].AfterInputQty-float64(correct)/1000) > 1e-8 {
			t.Fatalf("preview input inconsistent: %s; want %d g", r.Body.String(), correct)
		}
		body["check_only"] = false
		body["preview_token"] = preview.PreviewToken
		r = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
		var result app.SemiFinishedStockResult
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &result) != nil || result.Plan == nil {
			t.Fatalf("confirm: %d %s", r.Code, r.Body.String())
		}
		plan = *result.Plan
		check(step.want)
		previous = step.want
		for i := 0; i < 2; i++ {
			r = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), map[string]any{"draft_token": plan.DraftToken, "check_only": false})
			if r.Code != 200 {
				t.Fatalf("refresh: %d %s", r.Code, r.Body.String())
			}
			var refreshed app.ProductionPlanSupplySyncResult
			if err := json.Unmarshal(r.Body.Bytes(), &refreshed); err != nil {
				t.Fatal(err)
			}
			plan = refreshed.Plan
			check(step.want)
		}
	}
}
