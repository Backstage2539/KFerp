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
	sp "orderapp/internal/infrastructure/postgres/sales"
	saleshttp "orderapp/internal/interfaces/http/sales"
)

func TestShippingReturnGroupShipsOnceHTTP(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	orderID := f.request.OrderID
	r := sp.NewRepository(f.pool, f.schema)
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	rows := []d.Row{
		{RowNo: 2, TrackingNo: "SF-GROUP-A", ActualCents: 1000, Receiver: "测*", Phone: "138****0000", SettlementWeightKG: "1.200", Status: "pending"},
		{RowNo: 3, TrackingNo: "SF-GROUP-B", ActualCents: 1500, Receiver: "测*", Phone: "138****0000", SettlementWeightKG: "2.300", Status: "pending"},
	}
	batch, err := r.ImportShippingReturn(ctx, "tester", "group.xlsx", "group-test", cfg, rows)
	if err != nil || batch.Updated != 0 || batch.Pending != 2 {
		t.Fatalf("ambiguous import %+v %v", batch, err)
	}
	extra := rows[0]
	extra.TrackingNo = "SF-OTHER-BATCH"
	other, err := r.ImportShippingReturn(ctx, "tester", "other.xlsx", "other-test", cfg, []d.Row{extra})
	if err != nil || other.Pending != 1 {
		t.Fatalf("other batch %+v %v", other, err)
	}
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)
VALUES('GROUP-RETURN','finished_product',910,'测试库存',0,7031,7032,501,4,4,'passed');
UPDATE %[1]s.finished_inventory SET onhand_units=4 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units)
VALUES('finished_product',910,'测试库存',501,0,7031,7032,'DEV-E2E-A1','production_run',703,'GROUP-RETURN',4,4);`, f.schema)); err != nil {
		t.Fatal(err)
	}
	edit, err := f.svc.FulfillmentOrderEdit(ctx, orderID)
	if err != nil {
		t.Fatal(err)
	}
	cmd := editCommand(edit)
	cmd.ReplacePendingWithStock = true
	f.save(t, cmd)
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`WITH exported AS (INSERT INTO %[1]s.order_shipments(shipment_no) VALUES('EXPORT-GROUP') RETURNING id)
		INSERT INTO %[1]s.order_shipment_orders(shipment_id,order_id) SELECT id,$1 FROM exported`, f.schema), orderID); err != nil {
		t.Fatal(err)
	}
	choices, err := r.ShippingReturnCandidates(ctx, orderID)
	if err != nil || len(choices) != 3 || !choices[0].CanSelect {
		t.Fatalf("choices %+v %v", choices, err)
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
	confirm := func(ids []int64, version string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"candidate_ids": ids, "version": version})
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/orders/%d/shipping-return-selection", orderID), strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		out := httptest.NewRecorder()
		e.ServeHTTP(out, req)
		return out
	}
	a, b := batch.Rows[0].ID, batch.Rows[1].ID
	version := choices[0].Version
	for _, bad := range []struct {
		ids     []int64
		version string
	}{
		{[]int64{a, other.Rows[0].ID}, version}, {[]int64{a, b}, "old"},
	} {
		if out := confirm(bad.ids, bad.version); out.Code != 409 {
			t.Fatalf("invalid group HTTP %d %s", out.Code, out.Body)
		}
	}
	// A late failure after the order, costs and stock have been written must roll all of them back.
	if _, err = f.pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_group() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id=%[2]d AND NEW.status='applied' THEN RAISE EXCEPTION 'reject-group'; END IF; RETURN NEW; END $$;
CREATE TRIGGER reject_group BEFORE UPDATE ON %[1]s.shipping_return_rows FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_group()`, f.schema, b)); err != nil {
		t.Fatal(err)
	}
	if out := confirm([]int64{a, b}, version); out.Code != 409 || !strings.Contains(out.Body.String(), "reject-group") {
		t.Fatalf("late failure HTTP %d %s", out.Code, out.Body)
	}
	var postings int
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.freight_actual_shipments WHERE order_id=$1)+(SELECT count(*) FROM %[1]s.order_shipping_trackings WHERE order_id=$1)+(SELECT count(*) FROM %[1]s.order_stock_deductions WHERE order_id=$1)+(SELECT count(*) FROM %[1]s.shipping_return_rows WHERE status='applied' AND selected_order_id=$1)`, f.schema), orderID).Scan(&postings); err != nil || postings != 0 {
		t.Fatalf("rollback left %d writes: %v", postings, err)
	}
	if _, err = f.pool.Exec(ctx, fmt.Sprintf("DROP TRIGGER reject_group ON %s.shipping_return_rows", f.schema)); err != nil {
		t.Fatal(err)
	}
	answers := make(chan *httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	for _, ids := range [][]int64{{b, a}, {a, b}} {
		go func(selected []int64) { <-start; answers <- confirm(selected, version) }(ids)
	}
	close(start)
	updated := 0
	for i := 0; i < 2; i++ {
		out := <-answers
		var response salesapp.FillShipmentTrackingResult
		if out.Code != 200 || json.Unmarshal(out.Body.Bytes(), &response) != nil {
			t.Fatalf("concurrent confirmation HTTP %d %s", out.Code, out.Body)
		}
		updated += response.Updated
	}
	if updated != 1 {
		t.Fatalf("concurrent requests updated %d orders", updated)
	}
	if out := confirm([]int64{a}, version); out.Code != 409 {
		t.Fatalf("partial replay HTTP %d", out.Code)
	}
	var tracks, exported string
	var applied, cost, deducted, audit int64
	err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT
(SELECT ship_tracking_no FROM %[1]s.orders WHERE id=$1),
(SELECT so.tracking_no FROM %[1]s.order_shipment_orders so JOIN %[1]s.order_shipments s ON s.id=so.shipment_id WHERE so.order_id=$1 AND s.shipment_no='EXPORT-GROUP' AND s.status='shipped'),
(SELECT count(*) FROM %[1]s.shipping_return_rows WHERE status='applied' AND selected_order_id=$1),
(SELECT sum(actual_cents) FROM %[1]s.freight_actual_shipments WHERE order_id=$1),
(SELECT sum(deducted_units) FROM %[1]s.order_stock_deductions WHERE order_id=$1),
(SELECT count(*) FROM %[1]s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND field='shipping_return')`, f.schema), orderID).Scan(&tracks, &exported, &applied, &cost, &deducted, &audit)
	if err != nil || tracks != "SF-GROUP-A\nSF-GROUP-B" || exported != tracks || applied != 2 || cost != 2500 || deducted != 4 || audit != 1 {
		t.Fatalf("posted tracks=%q applied=%d cost=%d stock=%d audit=%d err=%v", tracks, applied, cost, deducted, audit, err)
	}
}
