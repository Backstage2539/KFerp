package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	fulfillmenthttp "orderapp/internal/interfaces/http/customerfulfillment"
)

func TestDirectShipHistoricalOwnedBatchesWithCurrentOrderVariantHTTP(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status,source_doc_type,source_doc_id) VALUES
 ('HIST-BATCH-A','finished_product',910,'历史实物',0,7031,7032,501,10,10,'passed','stock_entry_transfer',701),('HIST-BATCH-B','finished_product',910,'历史实物',0,7031,7032,501,7,7,'passed','stock_entry_transfer',702);
 UPDATE %[1]s.finished_inventory SET onhand_units=17,bom_variant_id=7032 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units) VALUES
 ('finished_product',910,'历史实物',501,0,7031,7032,'DEV-E2E-A1','stock_entry',1,'HIST-BATCH-A',10,10),('finished_product',910,'历史实物',501,0,7031,7032,'DEV-E2E-A1','stock_entry',2,'HIST-BATCH-B',7,17);
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status,source_doc_type,source_doc_id) VALUES
 (8001,'ORIGINAL-A','finished_product',910,'历史源',0,7031,7032,0,10,0,'passed','production_operation_event',8001),
 (8002,'ORIGINAL-B','finished_product',910,'历史源',0,7031,7032,0,7,0,'passed','production_operation_event',8002);
 INSERT INTO %[1]s.stock_entries(id,entry_no,entry_type,purpose,status) VALUES (9001,'TRANSFER-A','finished_transfer','material_transfer','submitted'),(9002,'TRANSFER-B','finished_transfer','material_transfer','submitted');
 INSERT INTO %[1]s.stock_entry_items(id,stock_entry_id,item_type,product_id,bom_spec_id,bom_variant_id,owner_customer_id,target_owner_customer_id,spec_g,from_warehouse,to_warehouse,batch_code,qty_units) VALUES
 (9001,9001,'finished_product',910,7031,7032,0,501,0,'finished_goods','DEV-E2E-A1','HIST-BATCH-A',10),(9002,9002,'finished_product',910,7031,7032,0,501,0,'finished_goods','DEV-E2E-A1','HIST-BATCH-B',7);
 INSERT INTO %[1]s.stock_entry_finished_batch_moves(id,stock_entry_id,stock_entry_item_id,source_batch_id,source_batch_code,target_batch_id,target_batch_code,bom_spec_id,bom_variant_id,spec_g,qty_units)
 SELECT 701,9001,9001,8001,'ORIGINAL-A',id,batch_code,7031,7032,0,10 FROM %[1]s.stock_batches WHERE batch_code='HIST-BATCH-A';
 INSERT INTO %[1]s.stock_entry_finished_batch_moves(id,stock_entry_id,stock_entry_item_id,source_batch_id,source_batch_code,target_batch_id,target_batch_code,bom_spec_id,bom_variant_id,spec_g,qty_units)
 SELECT 702,9002,9002,8002,'ORIGINAL-B',id,batch_code,7031,7032,0,7 FROM %[1]s.stock_batches WHERE batch_code='HIST-BATCH-B';
 UPDATE %[1]s.stock_batches SET source_batch_id=CASE batch_code WHEN 'HIST-BATCH-A' THEN 'ORIGINAL-A' ELSE 'ORIGINAL-B' END WHERE batch_code IN ('HIST-BATCH-A','HIST-BATCH-B');
 UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit) VALUES(7033,7030,'V2','published',1,'袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g) VALUES(7034,7033,7031,'454g袋','袋',true,454);
 UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE output_type='product' AND output_id=910;`, f.schema)); err != nil {
		t.Fatal(err)
	}
	if err := migration.EnsureAuthorityProjection(ctx, f.pool, f.schema); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(703))
			c.Set("operator_employee", "销售员")
			return next(c)
		}
	})
	fulfillmenthttp.RegisterRoutes(e, fulfillmenthttp.Dependencies{CustomerFulfillment: f.svc})
	cmd := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", FulfillmentMode: "stock", IdempotencyKey: "HIST-BATCH-REAL", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南省", DetailAddress: "测试路", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 12}}}
	call := func(path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	res := call("/api/customer-fulfillment/501/direct-ship/preview", cmd)
	if res.Code != 200 {
		t.Fatalf("preview: %d %s", res.Code, res.Body.String())
	}
	var p app.MiniDirectShipPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	cmd.PriceQuoteToken, cmd.FeeQuoteToken = p.PriceQuoteToken, p.FeeQuoteToken
	results := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- call("/api/customer-fulfillment/501/direct-ship/requests", cmd) }()
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result.Code != 201 {
			t.Fatalf("concurrent submit: %d %s", result.Code, result.Body.String())
		}
		res = result
	}
	var request app.MiniDirectShipRequest
	if err := json.Unmarshal(res.Body.Bytes(), &request); err != nil {
		t.Fatal(err)
	}
	var lineVariant int64
	if err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_variant_id FROM %s.order_items WHERE order_id=$1`, f.schema), request.OrderID).Scan(&lineVariant); err != nil || lineVariant != 7034 {
		t.Fatalf("order variant=%d err=%v", lineVariant, err)
	}
	assertMiniDirectShipCount(t, f.pool, f.schema, "customer_direct_ship_request_allocations", fmt.Sprintf("request_id=%d AND bom_variant_id=7032", request.ID), 2)
	assertMiniDirectShipCount(t, f.pool, f.schema, "order_stock_batch_allocations", fmt.Sprintf("request_id=%d AND bom_variant_id=7032", request.ID), 2)
	var quantities string
	if err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT string_agg(allocated_units::text,',' ORDER BY allocated_units DESC) FROM %s.customer_direct_ship_request_allocations WHERE request_id=$1`, f.schema), request.ID).Scan(&quantities); err != nil || quantities != "10,2" {
		t.Fatalf("batch allocation=%s err=%v", quantities, err)
	}
	assertMiniDirectShipCount(t, f.pool, f.schema, "customer_direct_ship_requests", "idempotency_key='HIST-BATCH-REAL'", 1)
	again := call("/api/customer-fulfillment/501/direct-ship/requests", cmd)
	if again.Code != 201 {
		t.Fatalf("retry: %d %s", again.Code, again.Body.String())
	}
	var replay app.MiniDirectShipRequest
	if err := json.Unmarshal(again.Body.Bytes(), &replay); err != nil || replay.ID != request.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	// A failure after the request allocation is inserted must roll the whole
	// request, order, fee and both allocations back without touching stock.
	rollbackCmd := cmd
	rollbackCmd.IdempotencyKey = "HIST-BATCH-ROLLBACK"
	rollbackCmd.Items = append([]app.MiniDirectShipItemCommand(nil), cmd.Items...)
	rollbackCmd.Items[0].Qty = 1
	previewRollback := call("/api/customer-fulfillment/501/direct-ship/preview", rollbackCmd)
	var rp app.MiniDirectShipPreview
	if previewRollback.Code != 200 {
		t.Fatalf("rollback preview: %s", previewRollback.Body.String())
	}
	if err := json.Unmarshal(previewRollback.Body.Bytes(), &rp); err != nil {
		t.Fatal(err)
	}
	rollbackCmd.PriceQuoteToken, rollbackCmd.FeeQuoteToken = rp.PriceQuoteToken, rp.FeeQuoteToken
	state := func() string {
		t.Helper()
		var value string
		err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_array((SELECT count(*) FROM %[1]s.orders),(SELECT count(*) FROM %[1]s.customer_direct_ship_requests),(SELECT count(*) FROM %[1]s.customer_direct_ship_request_allocations),(SELECT count(*) FROM %[1]s.order_stock_batch_allocations),(SELECT count(*) FROM %[1]s.customer_fee_items),(SELECT count(*) FROM %[1]s.audit_logs),(SELECT sum(remaining_units) FROM %[1]s.stock_batches WHERE batch_code IN ('HIST-BATCH-A','HIST-BATCH-B')))::text`, f.schema)).Scan(&value)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := state()
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.fail_second_allocation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced allocation failure'; END $$; CREATE TRIGGER fail_second_allocation BEFORE INSERT ON %[1]s.order_stock_batch_allocations FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_second_allocation()`, f.schema)); err != nil {
		t.Fatal(err)
	}
	failed := call("/api/customer-fulfillment/501/direct-ship/requests", rollbackCmd)
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER fail_second_allocation ON %s.order_stock_batch_allocations`, f.schema)); err != nil {
		t.Fatal(err)
	}
	if failed.Code == 201 || !strings.Contains(failed.Body.String(), "forced allocation failure") {
		t.Fatalf("forced failure: %d %s", failed.Code, failed.Body.String())
	}
	if after := state(); after != before {
		t.Fatalf("failed request changed state: before=%s after=%s", before, after)
	}

	// Every mutation is in a rolled-back transaction, with the genuine first
	// allocation as a fixture. An attacker cannot use a different provenance,
	// identity or quantity to bypass the historical-variant guard.
	for _, tc := range []struct{ name, mutation string }{
		{"wrong customer", `UPDATE %[1]s.customer_direct_ship_requests SET customer_id=502 WHERE id=%[2]d`},
		{"void order", `UPDATE %[1]s.orders SET is_void=true WHERE id=%[3]d`},
		{"wrong request item", `UPDATE negative_alloc SET request_item_id=0`},
		{"wrong request order", `UPDATE negative_alloc SET request_order_id=0`},
		{"partial schema missing request orders", `ALTER TABLE %[1]s.customer_direct_ship_request_orders RENAME TO temporarily_missing_request_orders`},
		{"partial schema missing request items", `ALTER TABLE %[1]s.customer_direct_ship_request_items RENAME TO temporarily_missing_request_items`},
		{"partial schema missing stock entries", `ALTER TABLE %[1]s.stock_entries RENAME TO temporarily_missing_stock_entries`},
		{"partial schema missing stock items", `ALTER TABLE %[1]s.stock_entry_items RENAME TO temporarily_missing_stock_items`},
		{"partial schema missing batches", `ALTER TABLE %[1]s.stock_batches RENAME TO temporarily_missing_batches`},
		{"partial schema missing ledger", `ALTER TABLE %[1]s.stock_ledger_entries RENAME TO temporarily_missing_ledger`},
		{"partial schema missing warehouses", `ALTER TABLE %[1]s.warehouses RENAME TO temporarily_missing_warehouses`},
		{"partial schema missing moves", `ALTER TABLE %[1]s.stock_entry_finished_batch_moves RENAME TO temporarily_missing_moves`},
		{"partial schema missing deductions", `ALTER TABLE %[1]s.order_stock_deductions RENAME TO temporarily_missing_deductions`},
		{"partial schema missing order allocations", `ALTER TABLE %[1]s.order_stock_batch_allocations RENAME TO temporarily_missing_order_allocations`},
		{"cancelled request order", `UPDATE %[1]s.customer_direct_ship_request_orders SET status='cancelled' WHERE request_id=%[2]d`},
		{"wrong order item", `UPDATE negative_alloc SET order_item_id=0`},
		{"wrong product", `UPDATE negative_alloc SET product_id=911`},
		{"wrong specification", `UPDATE negative_alloc SET bom_spec_id=999999`},
		{"wrong archived variant", `UPDATE negative_alloc SET bom_variant_id=999999`},
		{"wrong batch code", `UPDATE negative_alloc SET batch_code='unrelated'`},
		{"missing batch", `UPDATE negative_alloc SET batch_id=0`},
		{"wrong warehouse", `UPDATE negative_alloc SET warehouse_code='finished_goods'`},
		{"wrong warehouse kind", `UPDATE %[1]s.warehouses SET kind='raw' WHERE code='DEV-E2E-A1'`},
		{"wrong owner", `UPDATE %[1]s.stock_batches SET owner_customer_id=502 WHERE batch_code='HIST-BATCH-A'`},
		{"quality hold with spaces", `UPDATE %[1]s.stock_batches SET quality_status=' hold ' WHERE batch_code='HIST-BATCH-A'`},
		{"source mismatch", `UPDATE %[1]s.stock_entry_finished_batch_moves SET source_batch_id=8002 WHERE id=701`},
		{"wrong transfer target", `UPDATE %[1]s.stock_entry_items SET batch_code='ORIGINAL-A' WHERE id=9001`},
		{"wrong transfer owner", `UPDATE %[1]s.stock_entry_items SET target_owner_customer_id=502 WHERE id=9001`},
		{"transfer not submitted", `UPDATE %[1]s.stock_entries SET status='draft' WHERE id=9001`},
		{"wrong quantity", `UPDATE negative_alloc SET allocated_qty=11,allocated_units=11`},
		{"zero quantity", `UPDATE negative_alloc SET allocated_qty=0,allocated_units=0`},
		{"wrong weight", `UPDATE negative_alloc SET allocated_g=1`},
		{"foreign reserved stock", `INSERT INTO %[1]s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,batch_id,batch_code,warehouse,allocated_units) SELECT %[3]d,order_item_id,product_id,bom_spec_id,7034,batch_id,batch_code,warehouse_code,1 FROM negative_alloc`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			setup := fmt.Sprintf(`CREATE TEMP TABLE negative_alloc ON COMMIT DROP AS SELECT * FROM %[1]s.customer_direct_ship_request_allocations WHERE request_id=%[2]d ORDER BY allocated_units DESC LIMIT 1; DELETE FROM %[1]s.order_stock_batch_allocations WHERE request_id=%[2]d; DELETE FROM %[1]s.customer_direct_ship_request_allocations WHERE request_id=%[2]d;`, f.schema, request.ID)
			if _, err = tx.Exec(ctx, setup); err != nil {
				t.Fatal(err)
			}
			mutation := tc.mutation
			if strings.Contains(mutation, "%[") {
				mutation = fmt.Sprintf(mutation, f.schema, request.ID, request.OrderID)
			}
			if _, err = tx.Exec(ctx, mutation); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_allocations(request_id,request_item_id,request_order_id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,allocated_qty,allocated_units,allocated_g,status) SELECT request_id,request_item_id,request_order_id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,allocated_qty,allocated_units,allocated_g,status FROM negative_alloc`, f.schema))
			if err == nil || !(strings.Contains(err.Error(), "bom_variant_not_current") || strings.Contains(err.Error(), "bom_spec_not_published") || strings.Contains(err.Error(), "direct_product_identity_requires_zero_spec")) {
				t.Fatalf("identity guard did not reject forged allocation: %v", err)
			}
		})
	}
	for _, tc := range []struct{ name, mutation string }{
		{"missing request mirror", `DELETE FROM %[1]s.customer_direct_ship_request_allocations WHERE request_id=%[2]d`},
		{"wrong request", `UPDATE negative_order SET request_id=0`},
		{"wrong order", `UPDATE negative_order SET order_id=71401`},
		{"wrong item", `UPDATE negative_order SET order_item_id=0`},
		{"wrong batch", `UPDATE negative_order SET batch_id=0`},
		{"wrong code", `UPDATE negative_order SET batch_code='wrong'`},
		{"wrong warehouse", `UPDATE negative_order SET warehouse='finished_goods'`},
		{"different mirrored quantity", `UPDATE negative_order SET allocated_units=9`},
		{"zero quantity", `UPDATE negative_order SET allocated_units=0`},
		{"different weight", `UPDATE negative_order SET allocated_g=1`},
	} {
		t.Run("order allocation/"+tc.name, func(t *testing.T) {
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			setup := fmt.Sprintf(`CREATE TEMP TABLE negative_order ON COMMIT DROP AS SELECT * FROM %[1]s.order_stock_batch_allocations WHERE request_id=%[2]d ORDER BY allocated_units DESC LIMIT 1; DELETE FROM %[1]s.order_stock_batch_allocations WHERE request_id=%[2]d`, f.schema, request.ID)
			if _, err = tx.Exec(ctx, setup); err != nil {
				t.Fatal(err)
			}
			mutation := tc.mutation
			if strings.Contains(mutation, "%[") {
				mutation = fmt.Sprintf(mutation, f.schema, request.ID)
			}
			if _, err = tx.Exec(ctx, mutation); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator) SELECT order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator FROM negative_order`, f.schema))
			if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
				t.Fatalf("historical guard did not reject forged order allocation: %v", err)
			}
		})
	}
	t.Run("duplicate request allocation", func(t *testing.T) {
		tx, err := f.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.customer_direct_ship_request_allocations(request_id,request_item_id,request_order_id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,allocated_qty,allocated_units,allocated_g,status) SELECT request_id,request_item_id,request_order_id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,allocated_qty,allocated_units,allocated_g,status FROM %[1]s.customer_direct_ship_request_allocations WHERE request_id=$1 ORDER BY allocated_units LIMIT 1`, f.schema), request.ID)
		if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
			t.Fatalf("historical guard did not reject duplicate request allocation: %v", err)
		}
	})
	t.Run("duplicate mirrored order allocation", func(t *testing.T) {
		tx, err := f.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator) SELECT order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator FROM %[1]s.order_stock_batch_allocations WHERE request_id=$1 LIMIT 1`, f.schema), request.ID)
		if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
			t.Fatalf("historical guard did not reject duplicate order allocation: %v", err)
		}
	})
}
