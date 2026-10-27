package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
	d "orderapp/internal/domain/shippingreturn"
	cs "orderapp/internal/infrastructure/postgres/customerstock"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	sp "orderapp/internal/infrastructure/postgres/sales"
	saleshttp "orderapp/internal/interfaces/http/sales"
)

// Exercise the real confirmation endpoint with the production identity guards:
// the order uses the new version, while its allocated stock retains V1.
func TestShippingReturnSelectionShipsFrozenVariantHTTP(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	r := sp.NewRepository(f.pool, f.schema)
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	b, err := r.ImportShippingReturn(ctx, "tester", "frozen.xlsx", "frozen-shipment", cfg, []d.Row{{RowNo: 3, TrackingNo: "SF-FROZEN", ActualCents: 1110, Receiver: "测*", Phone: "138****0000", Status: "pending"}})
	if err != nil || b.Updated != 0 {
		t.Fatalf("pending import %+v: %v", b, err)
	}
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = cs.LockFulfillmentCustomersTx(ctx, tx, f.schema, "EDIT-SOURCE"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)
VALUES('FROZEN-SHIPMENT','finished_product',910,'历史实物',0,7031,7032,501,4,4,'unchecked');
UPDATE %[1]s.finished_inventory SET onhand_units=onhand_units+4 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units)
VALUES('finished_product',910,'历史实物',501,0,7031,7032,'DEV-E2E-A1','production_run',703,'FROZEN-SHIPMENT',4,4);`, f.schema)); err != nil {
		t.Fatal(err)
	}
	if err = cs.ReceiveProductionTx(ctx, tx, f.schema, 703, 910, 7031, 7032, 0, 4, "FROZEN-SHIPMENT", "FROZEN-SHIPMENT", "DEV-E2E-A1", "EDIT-SOURCE", "tester"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`
UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;
INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7040,7030,'V2','published',1,'袋');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g)VALUES(7042,7040,7031,'454g袋','袋',true,454);
UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7040 WHERE output_type='product' AND output_id=910;
UPDATE %[1]s.order_items SET bom_variant_id=7042 WHERE order_id=%[2]d;`, f.schema, f.request.OrderID)); err != nil {
		t.Fatal(err)
	}
	if err = migration.EnsureAuthorityProjection(ctx, f.pool, f.schema); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		name, warehouse, batch, setup  string
		orderID, before, change, after int64
	}{
		{name: "wrong warehouse", warehouse: "other", batch: "FROZEN-SHIPMENT", orderID: f.request.OrderID, before: 8, change: -4, after: 4},
		{name: "wrong batch", warehouse: "DEV-E2E-A1", batch: "unallocated", orderID: f.request.OrderID, before: 8, change: -4, after: 4},
		{name: "wrong order", warehouse: "DEV-E2E-A1", batch: "FROZEN-SHIPMENT", orderID: 71401, before: 8, change: -4, after: 4},
		{name: "positive inbound", warehouse: "DEV-E2E-A1", batch: "FROZEN-SHIPMENT", orderID: f.request.OrderID, before: 0, change: 4, after: 4},
		{name: "exceeds allocation", warehouse: "DEV-E2E-A1", batch: "FROZEN-SHIPMENT", orderID: f.request.OrderID, before: 9, change: -5, after: 4},
		{name: "incorrect balance", warehouse: "DEV-E2E-A1", batch: "FROZEN-SHIPMENT", orderID: f.request.OrderID, before: 4, change: -4, after: 0},
		{name: "wrong owner", warehouse: "DEV-E2E-A1", batch: "FROZEN-SHIPMENT", orderID: f.request.OrderID, before: 8, change: -4, after: 4, setup: "UPDATE %s.stock_batches SET owner_customer_id=502 WHERE batch_code='FROZEN-SHIPMENT'"},
	} {
		t.Run(bad.name, func(t *testing.T) {
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if bad.setup != "" {
				if _, err = tx.Exec(ctx, fmt.Sprintf(bad.setup, f.schema)); err != nil {
					t.Fatal(err)
				}
			}
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_before_units,qty_change_units,qty_after_units)VALUES('finished_product',910,'历史实物',0,7031,7032,$1,'sales_order_shipment',$2,$3,$4,$5,$6)`, f.schema), bad.warehouse, bad.orderID, bad.batch, bad.before, bad.change, bad.after)
			if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
				t.Fatalf("invalid historical shipment accepted: %v", err)
			}
		})
	}
	// A deduction cannot claim historical stock without its matching outbound ledger.
	_, err = f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_stock_deductions(order_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,deducted_units,source_doc_type,source_doc_id)
SELECT $1,910,7031,7032,0,id,batch_code,4,'sales_order_shipment_inventory_sync_v2',$1 FROM %[1]s.stock_batches WHERE batch_code='FROZEN-SHIPMENT'`, f.schema), f.request.OrderID)
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("unbacked deduction accepted: %v", err)
	}
	if _, err = r.RematchShippingReturn(ctx, "tester", b.ID, nil); err != nil {
		t.Fatal(err)
	}
	choices, err := r.ShippingReturnCandidates(ctx, f.request.OrderID)
	if err != nil || len(choices) != 1 {
		t.Fatalf("frozen candidate %+v: %v", choices, err)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(703))
			c.Set("actor", "employee:703")
			return next(c)
		}
	})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesapp.NewService(r), Authz: fulfillmentEditTestAuthz{}})
	confirm := func() *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"candidate_id": choices[0].ID, "version": choices[0].Version})
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/orders/%d/shipping-return-selection", f.request.OrderID), strings.NewReader(string(raw)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		return w
	}
	snapshot := func() string {
		t.Helper()
		var state string
		err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object(
'order',(SELECT to_jsonb(o) FROM %[1]s.orders o WHERE id=$1),
'batches',(SELECT jsonb_agg(to_jsonb(b) ORDER BY id) FROM %[1]s.stock_batches b),
'inventory',(SELECT jsonb_agg(to_jsonb(i) ORDER BY product_id,bom_spec_id,warehouse) FROM %[1]s.finished_inventory i),
'ledger',(SELECT jsonb_agg(to_jsonb(l) ORDER BY id) FROM %[1]s.stock_ledger_entries l),
'deductions',(SELECT jsonb_agg(to_jsonb(d) ORDER BY id) FROM %[1]s.order_stock_deductions d),
'freight',(SELECT jsonb_agg(to_jsonb(f) ORDER BY id) FROM %[1]s.freight_actual_entries f),
'shipments',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM %[1]s.order_shipments s),
'rows',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM %[1]s.shipping_return_rows r),
'audit',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM %[1]s.order_audit_logs a))::text`, f.schema), f.request.OrderID).Scan(&state)
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	before := snapshot()
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_test_confirmation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.status='applied' THEN RAISE EXCEPTION 'test_late_confirmation_failure'; END IF; RETURN NEW; END $$;
CREATE TRIGGER reject_test_confirmation BEFORE UPDATE ON %[1]s.shipping_return_rows FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_test_confirmation();`, f.schema)); err != nil {
		t.Fatal(err)
	}
	w := confirm()
	if w.Code != 409 || !strings.Contains(w.Body.String(), "test_late_confirmation_failure") {
		t.Fatalf("late failure HTTP %d: %s", w.Code, w.Body.String())
	}
	if after := snapshot(); after != before {
		t.Fatal("failed confirmation changed order, stock, freight, row or audit")
	}
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_test_confirmation ON %s.shipping_return_rows`, f.schema)); err != nil {
		t.Fatal(err)
	}
	w = confirm()
	if w.Code != 200 {
		t.Fatalf("confirm frozen shipment HTTP %d: %s", w.Code, w.Body.String())
	}
	if w = confirm(); w.Code != 200 {
		t.Fatalf("idempotent confirmation HTTP %d: %s", w.Code, w.Body.String())
	}
	var remaining, onhand, deductions, units, variant, ledgers, costs int64
	err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT
(SELECT remaining_units FROM %[1]s.stock_batches WHERE batch_code='FROZEN-SHIPMENT'),
(SELECT onhand_units FROM %[1]s.finished_inventory WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'),
(SELECT count(*) FROM %[1]s.order_stock_deductions WHERE order_id=$1),
(SELECT coalesce(sum(deducted_units),0) FROM %[1]s.order_stock_deductions WHERE order_id=$1),
(SELECT coalesce(max(bom_variant_id),0) FROM %[1]s.order_stock_deductions WHERE order_id=$1),
(SELECT count(*) FROM %[1]s.stock_ledger_entries WHERE source_doc_type='sales_order_shipment' AND source_doc_id=$1 AND bom_variant_id=7032 AND qty_change_units=-4),
(SELECT count(*) FROM %[1]s.freight_actual_entries entry JOIN %[1]s.freight_actual_shipments shipment ON shipment.id=entry.shipment_id WHERE shipment.order_id=$1)`, f.schema), f.request.OrderID).Scan(&remaining, &onhand, &deductions, &units, &variant, &ledgers, &costs)
	if err != nil || remaining != 0 || onhand != 0 || deductions != 1 || units != 4 || variant != 7032 || ledgers != 1 || costs != 1 {
		t.Fatalf("shipment balances %d/%d deductions=%d units=%d variant=%d ledgers=%d costs=%d: %v", remaining, onhand, deductions, units, variant, ledgers, costs, err)
	}
}
