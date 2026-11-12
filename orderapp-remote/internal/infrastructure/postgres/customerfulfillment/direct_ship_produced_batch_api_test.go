package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	portalhttp "orderapp/internal/interfaces/http/customerportal"
)

// A customer's physically received production remains usable after BOM
// republishing, even when a new dropship was not reserved before the receipt.
func TestMiniDirectShipProducedHistoricalBatchWithCurrentOrderVariantHTTP(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	seedPR706FrozenWeightMaterials(t, pool, schema)
	configurePR703StockFees(t, pool, schema, 7031)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.products(id,name,product_kind,active) VALUES(920,'第二种客户咖啡','roasted_bean',true);
 INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,status) VALUES(7040,'HIST-SECOND','第二种咖啡','product',920,'active');
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit) VALUES(7040,7040,'V1','published',1,'袋');
 INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES(7041,7040,'SECOND-454','454g','454g袋','袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g) VALUES(7042,7040,7041,'454g袋','袋',true,454);
 INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit) VALUES(7040,7042,70601,'material','fixed_qty',.454);
 INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default) VALUES('product',920,7040,7040,true);
 INSERT INTO %[1]s.produce_running_items(id,batch_id,product_id,bom_spec_id,bom_variant_id,product_name,spec_g,need_g,status) VALUES(8001,'PRODUCED-RUN',910,7031,7032,'历史制造批次',0,0,'running');
 INSERT INTO %[1]s.work_orders(id,work_order_no,running_item_id,batch_id,product_id,bom_spec_id,bom_variant_id,bom_version_id,inventory_unit,customer_id,target_warehouse,status) VALUES(8001,'WO-PRODUCED',8001,'PRODUCED-RUN',910,7031,7032,7030,'袋',501,'DEV-E2E-A1','running');
 INSERT INTO %[1]s.job_cards(id,work_order_id,operation,status) VALUES(8001,8001,'包装','completed');
 INSERT INTO %[1]s.production_operation_events(id,work_order_id,job_card_id,kind,status,request_id,expected_version,operator,input_unit,output_qty,output_unit,warehouse,receipt_enabled,final_output,payload,result) VALUES(8001,8001,8001,'complete','posted','PRODUCED-RECEIPT','1','test','袋',48,'袋','DEV-E2E-A1',true,true,'{}','{"stock_entry_ids":[9001]}');
 INSERT INTO %[1]s.stock_entries(id,entry_no,entry_type,purpose,status,work_order_id,job_card_id,running_item_id,source_type,source_id) VALUES(9001,'PRODUCED-RECEIPT','finished_receipt','manufacture','submitted',8001,8001,8001,'operation_completion',8001);
 INSERT INTO %[1]s.stock_entry_items(id,stock_entry_id,item_type,product_id,bom_spec_id,bom_variant_id,owner_customer_id,spec_g,to_warehouse,batch_code,qty_units) VALUES(9001,9001,'finished_product',910,7031,7032,501,0,'DEV-E2E-A1','PRODUCED-BATCH',48);
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status,source_doc_type,source_doc_id,source_batch_id) VALUES
 (8001,'PRODUCED-BATCH','finished_product',910,'历史制造批次',0,7031,7032,501,48,11,'unchecked','production_operation_event',8001,'PRODUCED-RUN'),
 (8002,'TRANSFER-SOURCE','finished_product',920,'历史转仓源',0,7041,7042,0,10,0,'passed','production_run',0,''),
 (8003,'TRANSFER-BATCH','finished_product',920,'历史转仓批次',0,7041,7042,501,10,9,'passed','stock_entry_transfer',701,'TRANSFER-SOURCE');
 INSERT INTO %[1]s.stock_entries(id,entry_no,entry_type,purpose,status) VALUES(9002,'TRANSFER-SECOND','finished_transfer','material_transfer','submitted');
 INSERT INTO %[1]s.stock_entry_items(id,stock_entry_id,item_type,product_id,bom_spec_id,bom_variant_id,owner_customer_id,target_owner_customer_id,spec_g,from_warehouse,to_warehouse,batch_code,qty_units) VALUES(9002,9002,'finished_product',920,7041,7042,0,501,0,'finished_goods','DEV-E2E-A1','TRANSFER-BATCH',10);
 INSERT INTO %[1]s.stock_entry_finished_batch_moves(id,stock_entry_id,stock_entry_item_id,source_batch_id,source_batch_code,target_batch_id,target_batch_code,bom_spec_id,bom_variant_id,spec_g,qty_units) VALUES(701,9002,9002,8002,'TRANSFER-SOURCE',8003,'TRANSFER-BATCH',7041,7042,0,10);
 UPDATE %[1]s.finished_inventory SET onhand_units=11,bom_variant_id=7032 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
 INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,owner_customer_id,onhand_units) VALUES(920,7041,7042,0,'DEV-E2E-A1',501,9);
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,source_batch_id,qty_change_units,qty_after_units) VALUES
 ('finished_product',910,'历史制造批次',501,0,7031,7032,'DEV-E2E-A1','production_operation_event',8001,'PRODUCED-BATCH','PRODUCED-RUN',48,48),
 ('finished_product',920,'历史转仓批次',501,0,7041,7042,'DEV-E2E-A1','stock_entry',9002,'TRANSFER-BATCH','TRANSFER-SECOND',10,10);
 UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id IN(7030,7040);
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit) VALUES(7033,7030,'V2','published',1,'袋'),(7043,7040,'V2','published',1,'袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g) VALUES(7034,7033,7031,'454g袋','袋',true,454),(7044,7043,7041,'454g袋','袋',true,454);
 INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit) VALUES(7033,7034,70601,'material','fixed_qty',.454),(7043,7044,70601,'material','fixed_qty',.454);
 UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=CASE output_id WHEN 910 THEN 7033 ELSE 7043 END WHERE output_type='product' AND output_id IN(910,920);
 `, schema)); err != nil {
		t.Fatal(err)
	}
	if err := migration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	svc := app.NewService(repo)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	e := echo.New()
	portalhttp.RegisterRoutes(e, portalhttp.Dependencies{CustomerPortal: &priceCatalogPrincipal{customerID: 501}, CustomerFulfillment: svc})
	call := func(path string, cmd app.MiniDirectShipCommand) *httptest.ResponseRecorder {
		t.Helper()
		raw, err := json.Marshal(cmd)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderAuthorization, "Bearer produced-batch-test")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	quote := func(cmd app.MiniDirectShipCommand) app.MiniDirectShipCommand {
		t.Helper()
		res := call("/api/mini/direct-ship/preview", cmd)
		if res.Code != 200 {
			t.Fatalf("preview: %d %s", res.Code, res.Body.String())
		}
		var preview app.MiniDirectShipPreview
		if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if !preview.CanSubmit || !preview.StockReady {
			t.Fatalf("owned physical stock unavailable: %+v", preview)
		}
		cmd.PriceQuoteToken, cmd.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
		return cmd
	}
	cmd := quote(app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", FulfillmentMode: "stock", IdempotencyKey: "PRODUCED-HISTORY", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南省", DetailAddress: "测试路", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}, {ProductID: 920, BomSpecID: 7041, BomVariantID: 7042, Qty: 2}}})
	res := call("/api/mini/direct-ship/requests", cmd)
	if res.Code != 201 {
		t.Fatalf("submit produced historical stock: %d %s", res.Code, res.Body.String())
	}
	var request app.MiniDirectShipRequest
	if err := json.Unmarshal(res.Body.Bytes(), &request); err != nil {
		t.Fatal(err)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_items", fmt.Sprintf("order_id=%d AND bom_variant_id IN(7034,7044)", request.OrderID), 2)
	assertMiniDirectShipCount(t, pool, schema, "customer_direct_ship_request_allocations", fmt.Sprintf("request_id=%d AND bom_variant_id IN(7032,7042) AND allocated_units=2", request.ID), 2)
	assertMiniDirectShipCount(t, pool, schema, "order_stock_batch_allocations", fmt.Sprintf("request_id=%d AND bom_variant_id IN(7032,7042) AND allocated_units=2", request.ID), 2)
	assertMiniDirectShipCount(t, pool, schema, "stock_batches", "(id=8001 AND bom_variant_id=7032 AND remaining_units=11) OR (id=8003 AND bom_variant_id=7042 AND remaining_units=9)", 2)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='customer_direct_ship_request' AND entity_id=%d AND action='submit'", request.ID), 1)
	for i := 0; i < 2; i++ {
		res = call("/api/mini/direct-ship/requests", cmd)
		var replay app.MiniDirectShipRequest
		if res.Code != 201 || json.Unmarshal(res.Body.Bytes(), &replay) != nil || replay.ID != request.ID {
			t.Fatalf("idempotent retry: %d %s", res.Code, res.Body.String())
		}
	}
	assertMiniDirectShipCount(t, pool, schema, "customer_direct_ship_requests", "idempotency_key='PRODUCED-HISTORY'", 1)

	state := func() string {
		t.Helper()
		var out string
		err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_array((SELECT count(*) FROM %[1]s.orders),(SELECT count(*) FROM %[1]s.customer_direct_ship_requests),(SELECT count(*) FROM %[1]s.customer_direct_ship_request_allocations),(SELECT count(*) FROM %[1]s.order_stock_batch_allocations),(SELECT count(*) FROM %[1]s.customer_fee_items),(SELECT count(*) FROM %[1]s.audit_logs),(SELECT sum(remaining_units) FROM %[1]s.stock_batches))::text`, schema)).Scan(&out)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	rollback := cmd
	rollback.IdempotencyKey = "PRODUCED-ROLLBACK"
	rollback = quote(rollback)
	before := state()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.fail_produced_order_allocation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced allocation failure'; END $$; CREATE TRIGGER fail_produced_order_allocation BEFORE INSERT ON %[1]s.order_stock_batch_allocations FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_produced_order_allocation()`, schema)); err != nil {
		t.Fatal(err)
	}
	failed := call("/api/mini/direct-ship/requests", rollback)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER fail_produced_order_allocation ON %s.order_stock_batch_allocations`, schema)); err != nil {
		t.Fatal(err)
	}
	if failed.Code != 500 {
		t.Fatalf("forced failure: %d %s", failed.Code, failed.Body.String())
	}
	if after := state(); after != before {
		t.Fatalf("failed request changed stock/orders/fees/audit: %s -> %s", before, after)
	}

	for _, tc := range []struct{ name, mutation string }{
		{"event missing", `UPDATE %[1]s.stock_batches SET source_doc_id=999999 WHERE id=8001`},
		{"event not posted", `UPDATE %[1]s.production_operation_events SET status='posting' WHERE id=8001`},
		{"event not completion", `UPDATE %[1]s.production_operation_events SET kind='start' WHERE id=8001`},
		{"event not final", `UPDATE %[1]s.production_operation_events SET final_output=false WHERE id=8001`},
		{"event receipt disabled", `UPDATE %[1]s.production_operation_events SET receipt_enabled=false WHERE id=8001`},
		{"event wrong warehouse", `UPDATE %[1]s.production_operation_events SET warehouse='DEV-E2E-B1' WHERE id=8001`},
		{"event wrong quantity", `UPDATE %[1]s.production_operation_events SET output_qty=1 WHERE id=8001`},
		{"different count unit", `UPDATE %[1]s.production_operation_events SET output_unit='箱' WHERE id=8001`},
		{"destination changed inventory unit", `UPDATE %[1]s.production_bom_version_variants SET inventory_unit='箱' WHERE id=7034`},
		{"request changed inventory unit", `UPDATE %[1]s.customer_direct_ship_request_items SET inventory_unit='箱' WHERE request_id=%[2]d AND product_id=910`},
		{"event wrong unit", `UPDATE %[1]s.production_operation_events SET output_unit='kg' WHERE id=8001`},
		{"work order missing", `UPDATE %[1]s.production_operation_events SET work_order_id=999999 WHERE id=8001`},
		{"work order wrong variant", `UPDATE %[1]s.work_orders SET bom_variant_id=7034 WHERE id=8001`},
		{"work order wrong version", `UPDATE %[1]s.work_orders SET bom_version_id=7033 WHERE id=8001`},
		{"work order wrong batch", `UPDATE %[1]s.work_orders SET batch_id='OTHER' WHERE id=8001`},
		{"run wrong identity", `UPDATE %[1]s.produce_running_items SET bom_variant_id=7034 WHERE id=8001`},
		{"job card wrong work order", `UPDATE %[1]s.job_cards SET work_order_id=999999 WHERE id=8001`},
		{"receipt not submitted", `UPDATE %[1]s.stock_entries SET status='draft' WHERE id=9001`},
		{"receipt not manufacture", `UPDATE %[1]s.stock_entries SET purpose='material_transfer' WHERE id=9001`},
		{"receipt wrong event", `UPDATE %[1]s.stock_entries SET source_id=999999 WHERE id=9001`},
		{"receipt wrong owner", `UPDATE %[1]s.stock_entry_items SET owner_customer_id=502,to_warehouse='DEV-E2E-B1' WHERE id=9001`},
		{"receipt wrong quantity", `UPDATE %[1]s.stock_entry_items SET qty_units=1 WHERE id=9001`},
		{"receipt wrong batch", `UPDATE %[1]s.stock_entry_items SET batch_code='OTHER' WHERE id=9001`},
		{"receipt wrong warehouse", `UPDATE %[1]s.stock_entry_items SET to_warehouse='DEV-E2E-A2' WHERE id=9001`},
		{"original ledger missing", `DELETE FROM %[1]s.stock_ledger_entries WHERE source_batch_code='PRODUCED-BATCH'`},
		{"original ledger wrong event", `UPDATE %[1]s.stock_ledger_entries SET source_doc_id=999999 WHERE source_batch_code='PRODUCED-BATCH'`},
		{"batch wrong owner", `UPDATE %[1]s.stock_batches SET owner_customer_id=502 WHERE id=8001`},
		{"batch held", `UPDATE %[1]s.stock_batches SET quality_status=' frozen ' WHERE id=8001`},
		{"unavailable quantity", `UPDATE %[1]s.stock_batches SET remaining_units=1 WHERE id=8001`},
		{"request wrong customer", `UPDATE %[1]s.customer_direct_ship_requests SET customer_id=502 WHERE id=%[2]d`},
		{"destination void", `UPDATE %[1]s.orders SET is_void=true WHERE id=%[3]d`},
		{"wrong request item", `UPDATE produced_allocation SET request_item_id=0`},
		{"wrong request order", `UPDATE produced_allocation SET request_order_id=0`},
		{"wrong quantity units", `UPDATE produced_allocation SET allocated_qty=3`},
		{"zero quantity", `UPDATE produced_allocation SET allocated_qty=0,allocated_units=0`},
		{"more than order line", `UPDATE produced_allocation SET allocated_qty=3,allocated_units=3`},
		{"weight substitution", `UPDATE produced_allocation SET allocated_g=1`},
		{"partial schema missing events", `ALTER TABLE %[1]s.production_operation_events RENAME TO hidden_operation_events`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, fmt.Sprintf(`CREATE TEMP TABLE produced_allocation ON COMMIT DROP AS SELECT * FROM %[1]s.customer_direct_ship_request_allocations WHERE request_id=%[2]d AND batch_id=8001; DELETE FROM %[1]s.order_stock_batch_allocations WHERE request_id=%[2]d AND batch_id=8001; DELETE FROM %[1]s.customer_direct_ship_request_allocations WHERE request_id=%[2]d AND batch_id=8001;`, schema, request.ID)); err != nil {
				t.Fatal(err)
			}
			mutation := tc.mutation
			if strings.Contains(mutation, "%[") {
				mutation = fmt.Sprintf(mutation, schema, request.ID, request.OrderID)
			}
			if _, err = tx.Exec(ctx, mutation); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_allocations(request_id,request_item_id,request_order_id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,allocated_qty,allocated_units,allocated_g,status) SELECT request_id,request_item_id,request_order_id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,allocated_qty,allocated_units,allocated_g,status FROM produced_allocation`, schema))
			if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
				t.Fatalf("forged historical production allocation accepted: %v", err)
			}
		})
	}
	for _, tc := range []struct{ name, mutation string }{
		{"missing request allocation", `DELETE FROM %[1]s.customer_direct_ship_request_allocations WHERE request_id=%[2]d AND batch_id=8001`},
		{"wrong request", `UPDATE produced_order_allocation SET request_id=0`},
		{"wrong order", `UPDATE produced_order_allocation SET order_id=0`},
		{"wrong item", `UPDATE produced_order_allocation SET order_item_id=0`},
		{"wrong batch", `UPDATE produced_order_allocation SET batch_id=0`},
		{"wrong batch code", `UPDATE produced_order_allocation SET batch_code='unrelated'`},
		{"wrong warehouse", `UPDATE produced_order_allocation SET warehouse='DEV-E2E-A2'`},
		{"different mirrored quantity", `UPDATE produced_order_allocation SET allocated_units=1`},
		{"zero quantity", `UPDATE produced_order_allocation SET allocated_units=0`},
		{"weight substitution", `UPDATE produced_order_allocation SET allocated_g=1`},
	} {
		t.Run("order allocation/"+tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, fmt.Sprintf(`CREATE TEMP TABLE produced_order_allocation ON COMMIT DROP AS SELECT * FROM %[1]s.order_stock_batch_allocations WHERE request_id=%[2]d AND batch_id=8001; DELETE FROM %[1]s.order_stock_batch_allocations WHERE request_id=%[2]d AND batch_id=8001`, schema, request.ID)); err != nil {
				t.Fatal(err)
			}
			mutation := tc.mutation
			if strings.Contains(mutation, "%[") {
				mutation = fmt.Sprintf(mutation, schema, request.ID)
			}
			if _, err = tx.Exec(ctx, mutation); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator) SELECT order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator FROM produced_order_allocation`, schema))
			if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
				t.Fatalf("forged historical order allocation accepted: %v", err)
			}
		})
	}
	for _, table := range []string{"customer_direct_ship_request_allocations", "order_stock_batch_allocations"} {
		t.Run("duplicate "+table, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			columns := "request_id,request_item_id,request_order_id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,allocated_qty,allocated_units,allocated_g,status"
			if table == "order_stock_batch_allocations" {
				columns = "order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator"
			}
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.%[2]s(%[3]s) SELECT %[3]s FROM %[1]s.%[2]s WHERE request_id=$1 AND batch_id=8001`, schema, table, columns), request.ID)
			if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
				t.Fatalf("duplicate historical allocation accepted: %v", err)
			}
		})
	}

}
