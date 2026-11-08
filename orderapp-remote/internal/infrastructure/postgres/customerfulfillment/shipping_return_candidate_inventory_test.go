package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	saleshttp "orderapp/internal/interfaces/http/sales"
	"strings"
	"testing"

	d "orderapp/internal/domain/shippingreturn"
	sp "orderapp/internal/infrastructure/postgres/sales"
)

func TestShippingReturnMatchesPendingOrderCreatedAfterImport(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	r := sp.NewRepository(f.pool, f.schema)
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	// At import time the new order has not yet acquired its final recipient.
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET receiver_phone='13900000000' WHERE id=$1`, f.schema), f.request.OrderID); err != nil {
		t.Fatal(err)
	}
	b, err := r.ImportShippingReturn(ctx, "tester", "returns.xlsx", "pending-live", cfg, []d.Row{
		{RowNo: 2, TrackingNo: "SF-OCT-10", ActualCents: 1310, Receiver: "测*", Phone: "138****0000", Time: "2026-10-10 18:04:18", Status: "pending"},
		{RowNo: 3, TrackingNo: "SF-OCT-31", ActualCents: 1350, Receiver: "测*", Phone: "138****0000", Time: "2026-10-31 18:21:18", Status: "pending"},
	})
	if err != nil || b.Updated != 0 {
		t.Fatalf("import %+v: %v", b, err)
	}
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET receiver_phone='13800000000',order_date='2026-10-10' WHERE id=$1`, f.schema), f.request.OrderID); err != nil {
		t.Fatal(err)
	}
	choices, err := r.ShippingReturnCandidates(ctx, f.request.OrderID)
	if err != nil || len(choices) != 2 {
		t.Fatalf("recipient matches hidden by pending inventory or stale import: %+v %v", choices, err)
	}
	for _, c := range choices {
		raw, _ := json.Marshal(c)
		var view map[string]any
		_ = json.Unmarshal(raw, &view)
		if view["can_select"] != false || view["block_reason"] == "" || view["order_date"] != "2026-10-10" {
			t.Fatalf("missing readiness/date: %s", raw)
		}
		if _, err = r.SelectShippingReturn(ctx, "tester", c.OrderID, c.ID, c.Version); err == nil {
			t.Fatal("pending entitlement shipped as physical inventory")
		}
	}
	if _, err = r.RematchShippingReturn(ctx, "tester", b.ID, nil); err != nil {
		t.Fatal(err)
	}
	choices, err = r.ShippingReturnCandidates(ctx, f.request.OrderID)
	if err != nil || len(choices) != 2 {
		t.Fatalf("rematch lost blocked candidates: %+v %v", choices, err)
	}
}

func TestFulfillmentExplicitPhysicalReplenishment(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	// Transferred customer-owned stock is real stock, not a receipt of the
	// original purchase entitlement; replenishment must release that reservation.
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)
VALUES('EXPLICIT-TRANSFER','finished_product',910,'客户转入现货',0,7031,7032,501,4,4,'passed');
UPDATE %[1]s.finished_inventory SET onhand_units=onhand_units+4 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units)
VALUES('finished_product',910,'客户转入现货',501,0,7031,7032,'DEV-E2E-A1','stock_entry',9001,'EXPLICIT-TRANSFER',4,4);
INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)
VALUES(8001,'REPLENISH-SOURCE','finished_product',910,'来源批次',0,7031,7032,0,4,0,'passed');
INSERT INTO %[1]s.stock_entries(id,entry_no,entry_type,purpose,status) VALUES(9001,'EXPLICIT-MOVE','finished_transfer','material_transfer','submitted');
INSERT INTO %[1]s.stock_entry_items(id,stock_entry_id,item_type,product_id,bom_spec_id,bom_variant_id,owner_customer_id,target_owner_customer_id,spec_g,from_warehouse,to_warehouse,batch_code,qty_units)
VALUES(9001,9001,'finished_product',910,7031,7032,0,501,0,'finished_goods','DEV-E2E-A1','EXPLICIT-TRANSFER',4);
INSERT INTO %[1]s.stock_entry_finished_batch_moves(id,stock_entry_id,stock_entry_item_id,source_batch_id,source_batch_code,target_batch_id,target_batch_code,bom_spec_id,bom_variant_id,spec_g,qty_units)
SELECT 701,9001,9001,8001,'REPLENISH-SOURCE',id,batch_code,7031,7032,0,4 FROM %[1]s.stock_batches WHERE batch_code='EXPLICIT-TRANSFER';
UPDATE %[1]s.stock_batches SET source_doc_type='stock_entry_transfer',source_doc_id=701,source_batch_id='REPLENISH-SOURCE' WHERE batch_code='EXPLICIT-TRANSFER';
UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;
INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit) VALUES(7033,7030,'V2','published',1,'袋');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g) VALUES(7034,7033,7031,'454g袋','袋',true,454);
UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE output_type='product' AND output_id=910;
UPDATE %[1]s.order_items SET bom_variant_id=7034 WHERE order_id=%[2]d;
UPDATE %[1]s.customer_direct_ship_request_items SET bom_variant_id=7034 WHERE request_id=%[3]d;`, f.schema, f.request.OrderID, f.request.ID)); err != nil {
		t.Fatal(err)
	}
	if err := migration.EnsureAuthorityProjection(ctx, f.pool, f.schema); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(703))
			c.Set("actor", "employee:703")
			return next(c)
		}
	})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesapp.NewService(sp.NewRepository(f.pool, f.schema)), Authz: fulfillmentEditTestAuthz{}, FulfillmentEditor: f.svc})
	send := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		return w
	}
	path := fmt.Sprintf("/api/orders/%d/fulfillment", f.request.OrderID)
	shipping := sp.NewRepository(f.pool, f.schema)
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	batch, err := shipping.ImportShippingReturn(ctx, "tester", "transferred.xlsx", "transfer-return", cfg, []d.Row{
		{RowNo: 2, TrackingNo: "SF-EXPLICIT-A", ActualCents: 1310, Receiver: "测*", Phone: "138****0000", Status: "pending"},
		{RowNo: 3, TrackingNo: "SF-EXPLICIT-B", ActualCents: 1350, Receiver: "测*", Phone: "138****0000", Status: "pending"},
	})
	if err != nil || batch.Updated != 0 {
		t.Fatalf("pending import %+v %v", batch, err)
	}
	v, err := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	c := editCommand(v)
	p, err := f.svc.PreviewFulfillmentOrderEdit(ctx, c)
	if err != nil || p.StockReady {
		t.Fatalf("default changed original sources: %+v %v", p, err)
	}
	// JSON intentionally exercises the public opt-in contract, including RED
	// before the command acquires that field.
	if err = json.Unmarshal([]byte(`{"replace_pending_with_stock":true}`), &c); err != nil {
		t.Fatal(err)
	}
	p, err = f.svc.PreviewFulfillmentOrderEdit(ctx, c)
	if err != nil || !p.StockReady {
		t.Fatalf("explicit physical replenishment unavailable: %+v %v", p, err)
	}
	w := send("POST", path+"/preview", c)
	if w.Code != 200 {
		t.Fatalf("preview HTTP %d %s", w.Code, w.Body.String())
	}
	if err = json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.StockSources) != 1 || p.StockSources[0].BatchCode != "EXPLICIT-TRANSFER" || p.StockSources[0].BomVariantID != 7032 {
		t.Fatalf("source plan not reviewable: %+v", p.StockSources)
	}
	c.PreviewToken = p.PreviewToken
	// A physical balance change after preview must fail, releasing no rights.
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_units=3 WHERE batch_code='EXPLICIT-TRANSFER'`, f.schema)); err != nil {
		t.Fatal(err)
	}
	if w = send("PATCH", path, c); w.Code == 200 {
		t.Fatal("stale inventory accepted")
	}
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_units=4 WHERE batch_code='EXPLICIT-TRANSFER'`, f.schema)); err != nil {
		t.Fatal(err)
	}
	// Audit failure must roll back the source switch, including released rights.
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_replenish_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='fulfillment_edit' THEN RAISE EXCEPTION 'audit unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_replenish BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_replenish_audit()`, f.schema)); err != nil {
		t.Fatal(err)
	}
	if w = send("PATCH", path, c); w.Code == 200 || !strings.Contains(w.Body.String(), "audit unavailable") {
		t.Fatalf("audit failure not returned: %d %s", w.Code, w.Body.String())
	}
	var held int64
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT sum(reserved_qty-converted_qty-released_qty) FROM %s.customer_stock_supply_reservations WHERE order_id=$1`, f.schema), v.OrderID).Scan(&held); err != nil || held != 4 {
		t.Fatalf("failed switch released rights %d: %v", held, err)
	}
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_replenish ON %s.audit_logs`, f.schema)); err != nil {
		t.Fatal(err)
	}
	if w = send("PATCH", path, c); w.Code != 200 {
		t.Fatalf("save HTTP %d %s", w.Code, w.Body.String())
	}
	var reserved, allocated, remaining, received int64
	err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT
(SELECT coalesce(sum(reserved_qty-converted_qty-released_qty),0) FROM %[1]s.customer_stock_supply_reservations WHERE order_id=$1),
(SELECT coalesce(sum(allocated_units),0) FROM %[1]s.order_stock_batch_allocations WHERE order_id=$1),
(SELECT remaining_units FROM %[1]s.stock_batches WHERE batch_code='EXPLICIT-TRANSFER'),
(SELECT received_qty FROM %[1]s.customer_stock_entitlements WHERE source_order_id=71401)`, f.schema), v.OrderID).Scan(&reserved, &allocated, &remaining, &received)
	if err != nil || reserved != 0 || allocated != 4 || remaining != 4 || received != 0 {
		t.Fatalf("replenishment changed facts: %d/%d/%d/%d %v", reserved, allocated, remaining, received, err)
	}
	if _, err = f.svc.UpdateFulfillmentOrder(ctx, c); err == nil {
		t.Fatal("stale repeat accepted")
	}
	choices, err := shipping.ShippingReturnCandidates(ctx, v.OrderID)
	if err != nil || len(choices) != 2 || !choices[0].CanSelect {
		t.Fatalf("replenishment not visible without rematch %+v %v", choices, err)
	}
	chosen := choices[0]
	body := map[string]any{"candidate_id": chosen.ID, "version": chosen.Version}
	selection := fmt.Sprintf("/api/orders/%d/shipping-return-selection", v.OrderID)
	for attempt := 0; attempt < 2; attempt++ {
		if w = send("POST", selection, body); w.Code != 200 {
			t.Fatalf("ship HTTP %d %s", w.Code, w.Body.String())
		}
	}
	var deducted, audit int64
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT coalesce(sum(deducted_units),0) FROM %[1]s.order_stock_deductions WHERE order_id=$1),(SELECT count(*) FROM %[1]s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND action='fulfillment_edit' AND new_value LIKE '%%EXPLICIT-TRANSFER%%')`, f.schema), v.OrderID).Scan(&deducted, &audit); err != nil || deducted != 4 || audit != 1 {
		t.Fatalf("wrong deductions/audit %d/%d %v", deducted, audit, err)
	}

}

func TestShippingReturnPendingMatchPreventsWrongAutomaticOlderOrder(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	r := sp.NewRepository(f.pool, f.schema)
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(order_no,customer_id,order_date,receiver_name,receiver_phone,process_status_id) VALUES('OLDER-READY',501,'2026-09-10','测试','13800000000',(SELECT id FROM %[1]s.order_process_statuses WHERE name='无需生产' LIMIT 1))`, f.schema)); err != nil {
		t.Fatal(err)
	}
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	b, err := r.ImportShippingReturn(ctx, "tester", "oct.xlsx", "older-ambiguity", cfg, []d.Row{{RowNo: 2, TrackingNo: "SF-AMBIGUOUS", ActualCents: 1310, Receiver: "测*", Phone: "138****0000", Status: "pending"}})
	if err != nil || b.Updated != 0 || b.Pending != 1 {
		t.Fatalf("incorrect auto-ship %+v %v", b, err)
	}
	choices, err := r.ShippingReturnCandidates(ctx, 0)
	if err != nil || len(choices) != 2 {
		t.Fatalf("missing ambiguity %+v %v", choices, err)
	}
}
