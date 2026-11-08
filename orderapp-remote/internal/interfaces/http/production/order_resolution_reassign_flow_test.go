package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	app "orderapp/internal/application/production"
	domain "orderapp/internal/domain/orderresolution"
	fulfillmentpg "orderapp/internal/infrastructure/postgres/customerfulfillment"
	portalpg "orderapp/internal/infrastructure/postgres/customerportal"
	customerstockpg "orderapp/internal/infrastructure/postgres/customerstock"
	financepg "orderapp/internal/infrastructure/postgres/finance"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	"testing"
)

func TestOrderResolutionReassignStartedProductionCompletesForTarget(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, work, run := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74920)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.customer_processing_production_demands ADD COLUMN IF NOT EXISTS request_id BIGINT NOT NULL DEFAULT 0; ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS source_warehouse TEXT NOT NULL DEFAULT 'finished_goods', ADD COLUMN IF NOT EXISTS total_amount NUMERIC NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS grand_total NUMERIC NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS pay_status_id BIGINT, ADD COLUMN IF NOT EXISTS shipping_amount NUMERIC NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS discount_amount NUMERIC NOT NULL DEFAULT 0; CREATE TABLE %s.pay_statuses(id BIGSERIAL PRIMARY KEY,name TEXT NOT NULL)`, schema, schema, schema))
	for _, ensure := range []func(context.Context, *pgxpool.Pool, string) error{portalpg.EnsureSchema, fulfillmentpg.EnsureSchema, customerstockpg.EnsureSchema, financepg.EnsureSchema, resolutionpg.EnsureSchema} {
		if err := ensure(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := `{"operations":[{"seq":1,"operation":"通用加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.order_process_statuses(name,sort,active)VALUES('生产完成',30,true)ON CONFLICT(name)DO NOTHING;
 INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)VALUES(900,100,'REASSIGN-SPEC','reassign','测试袋','袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)VALUES(901,100,900,'测试袋','袋',true);
 UPDATE %[1]s.products SET unit_rule_override_json='{"inventory_unit":"bag","default_sales_unit":"bag","unit_conversion_json":{"bag":{"bag":1}}}' WHERE id=1;
 UPDATE %[1]s.orders SET order_no='REASSIGN-SOURCE',total_amount=1000,grand_total=1000 WHERE id=1;
 UPDATE %[1]s.order_items SET bom_spec_id=900,bom_variant_id=901,qty=100,unit='袋',unit_price=10,line_total=1000 WHERE order_id=1;
 INSERT INTO %[1]s.orders(id,order_no,total_amount,grand_total)VALUES(74920,'REASSIGN-TARGET',1000,1000);
 INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,bom_spec_id,bom_variant_id,qty,unit,unit_price,line_total)VALUES(74920,74920,1,1,'测试袋',900,901,100,'袋',10,1000);
 UPDATE %[1]s.production_plan_items SET order_nos='REASSIGN-SOURCE',demand_sources_json='[]',bom_spec_id=900,bom_variant_id=901,spec_g=0,inventory_unit='袋',planned_inventory_qty=100,output_qty=100,output_unit='袋',customer_product_snapshot_json='[]',target_warehouse='finished_goods' WHERE id=(SELECT production_plan_item_id FROM %[1]s.work_orders WHERE id=%[2]d);
 UPDATE %[1]s.work_orders SET order_nos='REASSIGN-SOURCE',bom_spec_id=900,bom_variant_id=901,spec_g=0,material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,customer_product_snapshot_json='[]',target_warehouse='finished_goods' WHERE id=%[2]d;
 UPDATE %[1]s.produce_running_items SET order_nos='REASSIGN-SOURCE',bom_spec_id=900,bom_variant_id=901,spec_g=0 WHERE id=%[3]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%[2]d;
 UPDATE %[1]s.material_batches SET unit_cost=10 WHERE material_id IN(10,20);
 `, schema, work, run))
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb WHERE id=$1`, schema), work, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := migration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	var card, plan int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT j.id,w.production_plan_id FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE w.id=$1 ORDER BY j.id LIMIT 1`, schema), work).Scan(&card, &plan); err != nil {
		t.Fatal(err)
	}
	read := func(kind string) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/%s-preview", card, kind), nil)
		var p app.OperationCompletionPreview
		if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &p) != nil || p.BlockedReason != "" {
			t.Fatalf("%s preview: %d %s", kind, res.Code, res.Body.String())
		}
		return p
	}
	p := read("start")
	res := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "reassign-start"})
	if res.Code != 200 {
		t.Fatalf("start: %d %s", res.Code, res.Body.String())
	}
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "settle_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-08", Version: c.Version, IdempotencyKey: "reassign", Dispositions: []domain.Disposition{{Kind: "production", ID: plan, Choice: "reassign", Reassignments: []domain.ReassignSelection{{WorkOrderID: work, TargetItemID: 74920, Quantity: 100}}}}}
	result, err := r.Submit(ctx, 1, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range result.Steps {
		if _, err = r.CompleteStep(ctx, 1, result.ID, step.ID, resolutionpg.StepCommand{Choice: step.Choice, Note: "原投入全部转供目标需求", Reference: "TEST-APPROVAL", IdempotencyKey: fmt.Sprint(step.ID)}, "production"); err != nil {
			t.Fatal(err)
		}
	}
	p = read("completion")
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "actual_output_qty": 100, "output_unit": "袋", "expected_version": p.Version, "request_id": "reassign-finish"})
	if res.Code != 200 {
		t.Fatalf("complete after transfer: %d %s", res.Code, res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("work_order_id=%d AND remaining_qty=0", work), 1)
	assertProductionFlowCount(t, pool, schema, "production_logs", fmt.Sprintf("running_item_id=%d AND order_nos='REASSIGN-TARGET'", run), 1)
	assertProductionFlowCount(t, pool, schema, "orders", "id=1 AND resolution_status='closed'", 1)
	assertProductionFlowCount(t, pool, schema, "orders", fmt.Sprintf("id=74920 AND process_status_id=(SELECT id FROM %s.order_process_statuses WHERE name='生产完成')", schema), 1)
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=1 AND bom_spec_id=900 AND bom_variant_id=901 AND warehouse='finished_goods' AND onhand_units=100", 1)
	assertProductionFlowCount(t, pool, schema, "production_batch_costs", fmt.Sprintf("running_item_id=%d", run), 1)

}
