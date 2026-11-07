package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	resolutionhttp "orderapp/internal/interfaces/http/orderresolution"
	"orderapp/internal/interfaces/http/support"
	"testing"
	"time"
)

func TestOrderResolutionReplaceSupplyPreservesPackageAndHistory(t *testing.T) {
	t.Run("new target reservation", func(t *testing.T) { testResolutionReplaceSupply(t, false) })
	t.Run("existing target reservation", func(t *testing.T) { testResolutionReplaceSupply(t, true) })
}
func testResolutionReplaceSupply(t *testing.T, existing bool) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	sr := salespg.NewRepository(pool, schema)
	product, price := int64(910), 30.0
	sale := func(quantity int64) salesapp.SaveOrderResult {
		result, err := sr.SaveOrder(ctx, salesapp.SaveOrderCommand{CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(), DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Actor: "employee:703", Items: []salesapp.OrderItemCommand{{ProductID: &product, Name: "替换货源样本", Units: quantity, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}}})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	source, target := sale(3), sale(6)
	configurePR703StockFees(t, pool, schema, 7031)
	service := app.NewService(NewRepository(pool, schema))
	service.UseSalesOrderService(salesapp.NewService(sr))
	ship := app.MiniDirectShipCommand{FreightProtocol: "deferred_freight_v1", CustomerID: 501, FulfillmentMode: "stock", IdempotencyKey: "resolution-supply-ship", RecipientName: "隔离样本", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", DetailAddress: "测试路1号", Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 3}}}
	if existing {
		ship.Items[0].Qty = 6
	}
	preview, err := service.PreviewMiniDirectShip(ctx, ship)
	if err != nil {
		t.Fatal(err)
	}
	ship.PriceQuoteToken, ship.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	request, err := service.SubmitMiniDirectShip(ctx, ship)
	if err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			c.Set("operator_employee", "supply-api-actor")
			return next(c)
		}
	})
	e.Use(support.AuthorizationMiddleware(nil))
	resolutionhttp.RegisterRoutes(e, r, nil)
	call := func(path string, body any, want int) []byte {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s status=%d want=%d body=%s", path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	endpoint := fmt.Sprintf("/api/orders/%d/resolution", source.OrderID)
	current, err := r.Context(ctx, source.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	var dep domain.Dependency
	for _, d := range current.Dependencies {
		if d.Kind == "direct_ship" {
			dep = d
		}
	}
	if len(dep.SupplyReservations) != 1 {
		t.Fatalf("missing per-reservation source choices: %+v", dep)
	}
	s := dep.SupplyReservations[0]
	var targetID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_stock_entitlements WHERE source_order_id=$1`, schema), target.OrderID).Scan(&targetID); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidate := range s.Targets {
		if candidate.ID == targetID {
			found = true
		}
	}
	if !found {
		t.Fatalf("matching target unavailable: %+v", s)
	}
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: current.Version, IdempotencyKey: "replace-supply", Dispositions: []domain.Disposition{{Kind: "direct_ship", ID: request.ID, Choice: "replace_supply", Supplies: []domain.SupplyReplacement{{ReservationID: s.ID, TargetID: targetID, Quantity: 3}}}}}
	// Mixed physical and pending reservations need the separate stock disposition
	// flow; reject before creating a request instead of stranding warehouse approval.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_supply_reservations SET converted_qty=1,status='partially_converted' WHERE id=$1`, schema), s.ID); err != nil {
		t.Fatal(err)
	}
	partial, err := r.Context(ctx, source.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Version = partial.Version
	cmd.Dispositions[0].Supplies[0].Quantity = 2
	call(endpoint+"/preview", cmd, 400)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_supply_reservations SET converted_qty=0,status='reserved' WHERE id=$1`, schema), s.ID); err != nil {
		t.Fatal(err)
	}
	current, err = r.Context(ctx, source.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Version = current.Version
	cmd.Dispositions[0].Supplies[0].Quantity = 3
	invalid := cmd
	invalid.Dispositions = nil
	call(endpoint+"/preview", invalid, 400)
	call(endpoint+"/preview", cmd, 200)
	var result resolutionpg.Result
	if err = json.Unmarshal(call(endpoint+"/submit", cmd, 200), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) != 1 || result.Steps[0].Permission != "stock.resolve" {
		t.Fatalf("warehouse confirmation missing: %+v", result)
	}
	stepID := result.Steps[0].ID
	if !existing {
		replacement := sale(6)
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_stock_entitlements WHERE source_order_id=$1`, schema), replacement.OrderID).Scan(&targetID); err != nil {
			t.Fatal(err)
		}
		current, err = r.Context(ctx, source.OrderID)
		if err != nil {
			t.Fatal(err)
		}
		cmd.Version = current.Version
		cmd.IdempotencyKey = "change-replacement"
		cmd.Dispositions[0].Supplies[0].TargetID = targetID
		if err = json.Unmarshal(call(fmt.Sprintf("%s/%d/refresh", endpoint, result.ID), cmd, 200), &result); err != nil {
			t.Fatal(err)
		}
		activeStep := int64(0)
		for _, s := range result.Steps {
			if s.Status == "pending" {
				activeStep = s.ID
			}
		}
		if activeStep == 0 || activeStep == stepID {
			t.Fatal("changing the target reused the prior warehouse confirmation and label")
		}
		stepID = activeStep
	}
	confirm := resolutionpg.StepCommand{Choice: "replace_supply", Note: "明确保留代发并使用所选购货权益", IdempotencyKey: "replace-confirm"}
	// A target can lose capacity after the preview. Never release the original
	// reservation when the replacement is no longer available.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_entitlements SET promised_qty=2 WHERE id=$1`, schema), targetID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CompleteStep(ctx, source.OrderID, result.ID, stepID, confirm, "warehouse"); err == nil {
		t.Fatal("stale target capacity accepted")
	}
	var releasedBefore int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT released_qty FROM %s.customer_stock_supply_reservations WHERE id=$1`, schema), s.ID).Scan(&releasedBefore); err != nil || releasedBefore != 0 {
		t.Fatalf("failed replacement released original: %d %v", releasedBefore, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_entitlements SET promised_qty=6 WHERE id=$1`, schema), targetID); err != nil {
		t.Fatal(err)
	}
	// Audit and both reservation mutations must share the same transaction.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.fail_supply_audit() RETURNS trigger LANGUAGE plpgsql AS $f$ BEGIN IF NEW.action='resolution_replace_supply' THEN RAISE EXCEPTION 'isolated audit failure'; END IF; RETURN NEW; END $f$; CREATE TRIGGER fail_supply_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_supply_audit()`, schema)); err != nil {
		t.Fatal(err)
	}
	confirm.IdempotencyKey = "replace-audit-failure"
	if _, err = r.CompleteStep(ctx, source.OrderID, result.ID, stepID, confirm, "warehouse"); err == nil {
		t.Fatal("audit failure did not roll back")
	}
	var targetReserved int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(reserved_qty-converted_qty-released_qty),0) FROM %s.customer_stock_supply_reservations WHERE entitlement_id=$1`, schema), targetID).Scan(&targetReserved); err != nil || targetReserved != ship.Items[0].Qty-3 {
		t.Fatalf("failed audit changed target reservation: %d %v", targetReserved, err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT released_qty FROM %s.customer_stock_supply_reservations WHERE id=$1`, schema), s.ID).Scan(&releasedBefore); err != nil || releasedBefore != 0 {
		t.Fatalf("failed audit released original: %d %v", releasedBefore, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER fail_supply_audit ON %s.audit_logs`, schema)); err != nil {
		t.Fatal(err)
	}
	confirm.IdempotencyKey = "replace-retry-success"
	type completion struct {
		result  resolutionpg.Result
		command resolutionpg.StepCommand
		err     error
	}
	results := make(chan completion, 2)
	start := make(chan struct{})
	resolutionID := result.ID
	for n := 0; n < 2; n++ {
		go func(n int) {
			attempt := confirm
			attempt.IdempotencyKey = fmt.Sprintf("replace-concurrent-%d", n)
			<-start
			value, e := r.CompleteStep(ctx, source.OrderID, resolutionID, stepID, attempt, "warehouse")
			results <- completion{value, attempt, e}
		}(n)
	}
	close(start)
	var winner completion
	succeeded := 0
	for n := 0; n < 2; n++ {
		value := <-results
		if value.err == nil {
			succeeded++
			winner = value
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent replacements succeeded %d times", succeeded)
	}
	result, confirm = winner.result, winner.command
	if result.Status != "completed" || !result.Applied {
		t.Fatalf("replacement incomplete: %+v", result)
	}
	call(fmt.Sprintf("%s/%d/steps/%d", endpoint, result.ID, stepID), confirm, 200)
	var originalReleased, newReserved, stock, events int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT released_qty FROM %s.customer_stock_supply_reservations WHERE id=$1`, schema), s.ID).Scan(&originalReleased); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT reserved_qty-converted_qty-released_qty FROM %s.customer_stock_supply_reservations WHERE entitlement_id=$1 AND request_id=$2`, schema), targetID, request.ID).Scan(&newReserved); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(onhand_units),0) FROM %s.finished_inventory`, schema)).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE action='resolution_replace_supply'`, schema)).Scan(&events); err != nil {
		t.Fatal(err)
	}
	var packageStatus, sourceStatus string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.customer_direct_ship_request_orders WHERE request_id=$1`, schema), request.ID).Scan(&packageStatus); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT resolution_status FROM %s.orders WHERE id=$1`, schema), source.OrderID).Scan(&sourceStatus); err != nil {
		t.Fatal(err)
	}
	if originalReleased != 3 || newReserved != ship.Items[0].Qty || stock != 0 || events != 1 || packageStatus == "cancelled" || sourceStatus != "closed" {
		t.Fatalf("bad result released=%d new=%d stock=%d logs=%d package=%s source=%s", originalReleased, newReserved, stock, events, packageStatus, sourceStatus)
	}
}
