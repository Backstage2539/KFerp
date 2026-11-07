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
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	resolutionhttp "orderapp/internal/interfaces/http/orderresolution"
	"orderapp/internal/interfaces/http/support"
	"testing"
	"time"
)

func TestOrderResolutionReplaceSupplyPreservesPackageAndHistory(t *testing.T) {
	t.Run("new target reservation", func(t *testing.T) { testResolutionReplaceSupply(t, false, false) })
	t.Run("existing target reservation", func(t *testing.T) { testResolutionReplaceSupply(t, true, false) })
}
func TestOrderResolutionMixedSupplyPreservesPhysicalReservations(t *testing.T) {
	t.Run("new target", func(t *testing.T) { testResolutionReplaceSupply(t, false, true) })
	t.Run("existing target", func(t *testing.T) { testResolutionReplaceSupply(t, true, true) })
}
func testResolutionReplaceSupply(t *testing.T, existing, mixed bool) {
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
	if mixed {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status) VALUES('MIXED-RECEIPT','finished_product',910,'混合货源',0,7031,7032,501,1,1,'passed')`, schema)); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=1 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, schema)); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units) VALUES('finished_product',910,'混合货源',501,0,7031,7032,'DEV-E2E-A1','production_run',703,'MIXED-RECEIPT',1,1)`, schema)); err != nil {
			t.Fatal(err)
		}
		if err = customerstock.ReceiveProductionTx(ctx, tx, schema, 703, 910, 7031, 7032, 0, 1, "MIXED-RECEIPT", "MIXED-RECEIPT", "DEV-E2E-A1", source.OrderNo, "employee:703"); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	physicalSnapshot := func() string {
		t.Helper()
		var raw string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('batches',(SELECT jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text) FROM %[1]s.stock_batches x),'inventory',(SELECT jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text) FROM %[1]s.finished_inventory x),'allocations',(SELECT jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text) FROM %[1]s.order_stock_batch_allocations x),'direct_allocations',(SELECT jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text) FROM %[1]s.customer_direct_ship_request_allocations x),'receipts',(SELECT jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text) FROM %[1]s.customer_stock_entitlement_receipts x),'ledger',(SELECT jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text) FROM %[1]s.stock_ledger_entries x))::text`, schema)).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	physicalBefore := physicalSnapshot()
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
	cmd := domain.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: current.Version, IdempotencyKey: "replace-supply", Dispositions: []domain.Disposition{{Kind: "direct_ship", ID: request.ID, Choice: "replace_supply", Supplies: []domain.SupplyReplacement{{ReservationID: s.ID, TargetID: targetID, Quantity: s.Quantity}}}}}
	if mixed {
		if s.ConvertedQuantity != 1 || s.PhysicalVersion == "" || s.PhysicalIssue != "" {
			t.Fatalf("missing exact physical trace: %+v", s)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=0 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, schema)); err != nil {
			t.Fatal(err)
		}
		withoutStock, err := r.Context(ctx, source.OrderID)
		if err != nil {
			t.Fatal(err)
		}
		blocked := false
		for _, d := range withoutStock.Dependencies {
			if d.Kind == "direct_ship" && d.ID == request.ID {
				blocked = d.ReplacementIssue != ""
			}
		}
		if !blocked {
			t.Fatal("physical retention allowed without actual warehouse inventory")
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=1 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, schema)); err != nil {
			t.Fatal(err)
		}
		for _, d := range current.Dependencies {
			if d.Kind == "customer_stock" {
				cmd.Dispositions = append(cmd.Dispositions, domain.Disposition{Kind: d.Kind, ID: d.ID, Choice: "keep_sale"})
			}
		}
		call(endpoint+"/preview", cmd, 400)
		cmd.Dispositions[0].Supplies[0].PhysicalChoice = "keep_reserved"
		cmd.Dispositions[0].Supplies[0].PhysicalVersion = s.PhysicalVersion
	} else {
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
	}
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
	if mixed {
		var allocationID int64
		var oldActor string
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,operator FROM %s.order_stock_batch_allocations WHERE request_id=$1 ORDER BY id LIMIT 1`, schema), request.ID).Scan(&allocationID, &oldActor); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_stock_batch_allocations SET operator='changed-since-preview' WHERE id=$1`, schema), allocationID); err != nil {
			t.Fatal(err)
		}
		if _, err = r.CompleteStep(ctx, source.OrderID, result.ID, stepID, confirm, "warehouse"); err == nil {
			t.Fatal("physical reservations changed after preview but warehouse confirmation succeeded")
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_stock_batch_allocations SET operator=$2 WHERE id=$1`, schema), allocationID, oldActor); err != nil {
			t.Fatal(err)
		}
		if physicalSnapshot() != physicalBefore {
			t.Fatal("failed stale confirmation changed original physical facts")
		}
	}
	// A target can lose capacity after the preview. Never release the original
	// reservation when the replacement is no longer available.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_entitlements SET promised_qty=1 WHERE id=$1`, schema), targetID); err != nil {
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
	expectedStock, expectedPending := int64(0), int64(3)
	if mixed {
		expectedStock, expectedPending = 1, 2
	}
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
	if physicalSnapshot() != physicalBefore {
		t.Fatal("replacement changed physical inventory, batches, receipt history or existing allocations")
	}
	var packageStatus, sourceStatus string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.customer_direct_ship_request_orders WHERE request_id=$1`, schema), request.ID).Scan(&packageStatus); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT resolution_status FROM %s.orders WHERE id=$1`, schema), source.OrderID).Scan(&sourceStatus); err != nil {
		t.Fatal(err)
	}
	if originalReleased != expectedPending || newReserved != ship.Items[0].Qty-expectedStock || stock != expectedStock || events != 1 || packageStatus == "cancelled" || sourceStatus != "closed" {
		t.Fatalf("bad result released=%d new=%d stock=%d logs=%d package=%s source=%s", originalReleased, newReserved, stock, events, packageStatus, sourceStatus)
	}
	if mixed {
		latest, err := r.Context(ctx, source.OrderID)
		if err != nil {
			t.Fatal(err)
		}
		if latest.CreditCents != 6000 || latest.Lines[0].Delivered != 1 || latest.Lines[0].Closed != 2 || latest.Lines[0].Quantity != 3 {
			t.Fatalf("mixed close erased delivered obligation: %+v", latest)
		}
		var packageOrder int64
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT order_id FROM %s.customer_direct_ship_request_orders WHERE request_id=$1`, schema), request.ID).Scan(&packageOrder); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`SELECT %s.customer_stock_require_shippable($1)`, schema), packageOrder); err == nil {
			t.Fatal("pending replacement was treated as physical stock")
		}
		// Receipt against the new source converts only its pending quantity. The
		// original receipt and its converted reservation must not convert again.
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status) VALUES('REPLACEMENT-RECEIPT','finished_product',910,'混合货源',0,7031,7032,501,$1,$1,'passed')`, schema), newReserved); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=onhand_units+$1 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, schema), newReserved); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units) VALUES('finished_product',910,'混合货源',501,0,7031,7032,'DEV-E2E-A1','production_run',704,'REPLACEMENT-RECEIPT',$1::bigint,$1::bigint+1)`, schema), newReserved); err != nil {
			t.Fatal(err)
		}
		var targetNumber string
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT o.order_no FROM %[1]s.orders o JOIN %[1]s.customer_stock_entitlements e ON e.source_order_id=o.id WHERE e.id=$1`, schema), targetID).Scan(&targetNumber); err != nil {
			t.Fatal(err)
		}
		if err = customerstock.ReceiveProductionTx(ctx, tx, schema, 704, 910, 7031, 7032, 0, newReserved, "REPLACEMENT-RECEIPT", "REPLACEMENT-RECEIPT", "DEV-E2E-A1", targetNumber, "employee:703"); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`SELECT %s.customer_stock_require_shippable($1)`, schema), packageOrder); err != nil {
			t.Fatal(err)
		}
		var converted, allocated int64
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT sum(converted_qty) FROM %[1]s.customer_stock_supply_reservations WHERE request_id=$1),(SELECT sum(allocated_qty) FROM %[1]s.customer_direct_ship_request_allocations WHERE request_id=$1)`, schema), request.ID).Scan(&converted, &allocated); err != nil {
			t.Fatal(err)
		}
		if converted != ship.Items[0].Qty || allocated != ship.Items[0].Qty {
			t.Fatalf("mixed conversion duplicated physical stock: converted=%d allocated=%d", converted, allocated)
		}
	}
}
