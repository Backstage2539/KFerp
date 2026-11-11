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

// Two orders reserve distinct portions of the same physical WIP batch.
// The shared stock must cover demand exactly once across the orders.
func TestBatchReplenishmentSharedProductWIP(t *testing.T) {
	for _, bound := range []bool{true, false} {
		t.Run(fmt.Sprint("wip_reserved_", bound), func(t *testing.T) { testSharedProductWIP(t, bound) })
	}
}
func testSharedProductWIP(t *testing.T, bound bool) {
	f, ids := batchIssueFixture(t, true)
	ctx := context.Background()
	seedTypedProductComponentFlow(t, ctx, f.pool, f.schema)
	mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(`
 UPDATE %[1]s.stock_batches SET qty_g=120000,qty_units=120000,remaining_g=120000,remaining_units=120000 WHERE batch_code='FP-TYPED-ROASTED';
 UPDATE %[1]s.finished_inventory SET onhand_units=120000 WHERE product_id=2 AND warehouse='finished_goods';
 INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,qty_g,qty_units,remaining_g,remaining_units,quality_status) VALUES('PRODUCT-SHARED-FREE','finished_product',2,'散装熟豆中间品',1,10000,10000,10000,10000,'pass');
 INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(2,1,'wip',10000,0);
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,spec_g,warehouse,source_batch_code) VALUES('finished_product',2,1,'wip','PRODUCT-SHARED-FREE');
 DELETE FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id IN (SELECT work_order_id FROM %[1]s.job_cards WHERE id IN (%[2]d,%[3]d)) AND material_id=30;
 UPDATE %[1]s.work_order_material_reservations SET component_type='product',material_id=0,component_id=2,component_spec_g=1,material_name='散装熟豆中间品',source_warehouse='finished_goods' WHERE work_order_id IN (SELECT work_order_id FROM %[1]s.job_cards WHERE id IN (%[2]d,%[3]d)) AND material_id=30;
 INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,component_spec_g,stock_batch_id,batch_code,warehouse,reserved_g,reserved_units)
 SELECT r.id,r.work_order_id,0,'product',2,1,s.id,s.batch_code,'finished_goods',r.reserved_g,r.reserved_units FROM %[1]s.work_order_material_reservations r CROSS JOIN %[1]s.stock_batches s WHERE r.work_order_id IN (SELECT work_order_id FROM %[1]s.job_cards WHERE id IN (%[2]d,%[3]d)) AND r.component_type='product' AND r.component_id=2 AND s.batch_code='FP-TYPED-ROASTED';
 UPDATE %[1]s.work_orders SET material_snapshot='[{"material_id":0,"component_type":"product","component_product_id":2,"component_spec_g":1,"material_name":"散装熟豆中间品","unit":"kg","ratio_pct":100}]'::jsonb WHERE id IN (SELECT work_order_id FROM %[1]s.job_cards WHERE id IN (%[2]d,%[3]d));
 `, f.schema, ids[0], ids[1]))
	wantAvailable, wantShortage := 0.0, 25.57
	if bound {
		wantAvailable, wantShortage = 5, 15.57
		mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(` UPDATE %[1]s.work_order_material_reservation_batches SET reserved_g=reserved_g-5000 WHERE work_order_id IN (SELECT work_order_id FROM %[1]s.job_cards WHERE id IN (%[2]d,%[3]d)) AND component_type='product' AND warehouse='finished_goods';
 INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,component_spec_g,stock_batch_id,batch_code,warehouse,reserved_g)
 SELECT r.id,r.work_order_id,0,'product',2,1,s.id,s.batch_code,'wip',5000 FROM %[1]s.work_order_material_reservations r CROSS JOIN %[1]s.stock_batches s WHERE r.work_order_id IN (SELECT work_order_id FROM %[1]s.job_cards WHERE id IN (%[2]d,%[3]d)) AND r.component_type='product' AND r.component_id=2 AND s.batch_code='PRODUCT-SHARED-FREE';
`, f.schema, ids[0], ids[1]))
	}
	for _, id := range ids {
		res := serveMultilevelProductionJSON(t, f.app, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", id), nil)
		if res.Code != 200 {
			t.Fatalf("preview: %s", res.Body.String())
		}
		var preview app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if len(preview.Materials) != 1 || math.Abs(preview.Materials[0].AvailableQty-wantAvailable) > 1e-8 {
			t.Fatalf("wrong WIP availability: %+v", preview.Materials)
		}
	}
	p := batchPreview(t, f, ids)
	if len(p.Groups) != 1 {
		t.Fatalf("expected one shared product group: %+v", p)
	}
	excess := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", batchCommand(p, "product-shared-excess", wantShortage+0.001))
	if excess.Code == http.StatusOK {
		t.Fatalf("overfilled shared WIP shortage: %s", excess.Body.String())
	}
	exact := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", batchCommand(p, "product-shared-exact", wantShortage))
	if exact.Code != http.StatusOK {
		t.Fatalf("actual shared product WIP shortage should be reflected: %d %s", exact.Code, exact.Body.String())
	}
	confirm := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "product-shared-exact"})
	if confirm.Code != http.StatusOK {
		t.Fatalf("product confirm: %d %s", confirm.Code, confirm.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "return_source='workstation_replenishment' AND status='submitted'", 2)
}
