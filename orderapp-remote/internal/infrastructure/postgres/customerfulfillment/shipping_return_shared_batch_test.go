package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	d "orderapp/internal/domain/shippingreturn"
	progress "orderapp/internal/infrastructure/postgres/orderprogress"
	sp "orderapp/internal/infrastructure/postgres/sales"
	saleshttp "orderapp/internal/interfaces/http/sales"
)

func TestShippingReturnSharedBatchExcludesAlreadyDeductedOrders(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)
VALUES('SHARED-RETURN','finished_product',910,'共享实际库存',0,7031,7032,501,8,8,'passed');
UPDATE %[1]s.finished_inventory SET onhand_units=8 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units)
VALUES('finished_product',910,'共享实际库存',501,0,7031,7032,'DEV-E2E-A1','production_run',703,'SHARED-RETURN',8,8);`, f.schema)); err != nil {
		t.Fatal(err)
	}
	v, err := f.svc.FulfillmentOrderEdit(ctx, f.request.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	edit := editCommand(v)
	edit.ReplacePendingWithStock = true
	f.save(t, edit)
	command := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", CustomerID: 501, FulfillmentMode: "stock", IdempotencyKey: "SHARED-RETURN-SECOND", RecipientName: "测试", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", DetailAddress: "测试路1号", Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 4}}}
	preview, err := f.svc.PreviewMiniDirectShip(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	command.PriceQuoteToken, command.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	second, err := f.svc.SubmitMiniDirectShip(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	shipping := sp.NewRepository(f.pool, f.schema)
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	batch, err := shipping.ImportShippingReturn(ctx, "tester", "shared.xlsx", "shared-return", cfg, []d.Row{
		{RowNo: 2, TrackingNo: "SF-SHARED-A", ActualCents: 1310, Receiver: "测*", Phone: "138****0000", Status: "pending"},
		{RowNo: 3, TrackingNo: "SF-SHARED-B", ActualCents: 1350, Receiver: "测*", Phone: "138****0000", Status: "pending"},
	})
	if err != nil || batch.Updated != 0 || batch.Pending != 2 {
		t.Fatalf("ambiguous import %+v %v", batch, err)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(703))
			c.Set("actor", "employee:703")
			return next(c)
		}
	})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesapp.NewService(shipping), Authz: fulfillmentEditTestAuthz{}, FulfillmentEditor: f.svc})
	ship := func(orderID int64, tracking string) {
		t.Helper()
		choices, err := shipping.ShippingReturnCandidates(ctx, orderID)
		if err != nil {
			t.Fatal(err)
		}
		var chosen d.Row
		for _, c := range choices {
			if c.TrackingNo == tracking {
				chosen = c
			}
		}
		if chosen.ID == 0 || !chosen.CanSelect {
			t.Fatalf("remaining shared batch blocked after another order shipped: %+v", choices)
		}
		body, _ := json.Marshal(map[string]any{"candidate_id": chosen.ID, "version": chosen.Version})
		for attempt := 0; attempt < 2; attempt++ {
			req := httptest.NewRequest("POST", fmt.Sprintf("/api/orders/%d/shipping-return-selection", orderID), strings.NewReader(string(body)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, req)
			if w.Code != 200 {
				t.Fatalf("shipment HTTP %d: %s", w.Code, w.Body.String())
			}
		}
	}
	checkReady := func(remaining int64, want bool) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_units=$1 WHERE batch_code='SHARED-RETURN'`, f.schema), remaining); err != nil {
			t.Fatal(err)
		}
		p, err := progress.Load(ctx, f.pool, f.schema, []int64{second.OrderID})
		if err != nil || p[second.OrderID] == nil || p[second.OrderID].CanShip != want {
			t.Fatalf("remaining=%d readiness want %v: %+v %v", remaining, want, p[second.OrderID], err)
		}
	}
	// Both outstanding orders still count: four remaining cannot cover eight held.
	checkReady(4, false)
	checkReady(8, true)
	ship(f.request.OrderID, "SF-SHARED-A")
	// The shipped allocation is retained as history but is no longer a hold.
	checkReady(4, true)
	checkReady(3, false)
	checkReady(4, true)
	ship(second.OrderID, "SF-SHARED-B")
	var remaining, deducted, applied, audit, historical int64
	err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT
(SELECT remaining_units FROM %[1]s.stock_batches WHERE batch_code='SHARED-RETURN'),
(SELECT SUM(deducted_units) FROM %[1]s.order_stock_deductions WHERE order_id IN($1,$2)),
(SELECT COUNT(*) FROM %[1]s.shipping_return_rows WHERE batch_id=$3 AND status='applied'),
(SELECT COUNT(*) FROM %[1]s.audit_logs WHERE entity_type='order' AND entity_id IN($1,$2) AND field='shipping_return'),
(SELECT COUNT(*) FROM %[1]s.customer_direct_ship_request_allocations WHERE order_id IN($1,$2))`, f.schema), f.request.OrderID, second.OrderID, batch.ID).Scan(&remaining, &deducted, &applied, &audit, &historical)
	if err != nil || remaining != 0 || deducted != 8 || applied != 2 || audit != 2 || historical != 2 {
		t.Fatalf("stock/duplicate/history mismatch: %d/%d/%d/%d/%d %v", remaining, deducted, applied, audit, historical, err)
	}
}
