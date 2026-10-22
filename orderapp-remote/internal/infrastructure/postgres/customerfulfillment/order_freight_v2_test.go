package customerfulfillment

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	domain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	"testing"
	"time"
)

func TestOrderFreightV2SalesAndActual(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	r := NewRepository(pool, schema)
	fr := freightpg.NewRepository(pool, schema)
	sales := salespg.NewRepository(pool, schema)
	amount := int64(2000)
	override := &domain.Override{ManualAmountCents: &amount}
	qc := app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "sale", FreightProtocol: domain.OrderProtocol, EmployeeAdjustment: true, Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}, FreightOverride: override}
	quote, e := r.QuoteFulfillmentFees(ctx, qc)
	if e != nil {
		t.Fatal(e)
	}
	product := int64(910)
	price := 30.0
	cmd := salesapp.SaveOrderCommand{Actor: "test", CustomerID: 501, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: 1, ShipStatusID: 1, ShippingProvince: "云南省", FreightProtocol: domain.OrderProtocol, FreightQuoteToken: quote.Freight.Token, FreightOverride: override, ShippingAmount: 20, Items: []salesapp.OrderItemCommand{{ProductID: &product, BomSpecID: 7031, BomVariantID: 7032, Name: "手填运费测试", Units: 2, SalesUnit: "袋", ManualPrice: &price}}}
	result, e := sales.SaveOrder(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	check := func(want float64) {
		t.Helper()
		var total, shipping float64
		if e := pool.QueryRow(ctx, fmt.Sprintf(`SELECT grand_total::float8,shipping_amount::float8 FROM %s.orders WHERE id=$1`, schema), result.OrderID).Scan(&total, &shipping); e != nil || total != 60+want || shipping != want {
			t.Fatal(total, shipping, e)
		}
	}
	check(20)
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET shipping_amount=99 WHERE id=$1`, schema), result.OrderID); e == nil {
		t.Fatal("database allowed mismatch")
	}
	// An old open import must become stale when an unbilled order is adjusted.
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_tracking_no='MANUAL709',logistics_company_id=1 WHERE id=$1`, schema), result.OrderID); e != nil {
		t.Fatal(e)
	}
	stale, e := fr.PreviewImport(ctx, 1, "stale-manual", "manual.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "MANUAL709", ActualCents: 1200}}, "test")
	if e != nil {
		t.Fatal(e)
	}
	amount = 1500
	qc.OrderID = result.OrderID
	qc.ExpectedFreightToken = quote.Freight.Token
	quote, e = r.QuoteFulfillmentFees(ctx, qc)
	if e != nil {
		t.Fatal(e)
	}
	cmd.ShipTrackingNo = "MANUAL709"
	cmd.LogisticsCompanyID = 1
	cmd.EditID = result.OrderID
	cmd.ExpectedFreightToken = qc.ExpectedFreightToken
	cmd.FreightQuoteToken = quote.Freight.Token
	cmd.ShippingAmount = 15
	if _, e = sales.SaveOrder(ctx, cmd); e != nil {
		t.Fatal("edit", e)
	}
	check(15)
	if _, e = fr.ConfirmImport(ctx, stale.ID, "test"); e == nil {
		t.Fatal("stale import accepted")
	}
	var entry int64
	for index, cost := range []int64{1200, 1300} {
		b, e := fr.PreviewImport(ctx, 1, fmt.Sprintf("manual%d", index), "manual.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "MANUAL709", ActualCents: cost}}, "test")
		wantDelta := int64(-300)
		if index > 0 {
			wantDelta = 100
		}
		if e != nil || b.Rows[0].DeltaCents != wantDelta || b.Rows[0].FinalReceivableCents != cost {
			t.Fatal(b, e)
		}
		b, e = fr.ConfirmImport(ctx, b.ID, "test")
		if e != nil {
			t.Fatal(e)
		}
		entry = b.Rows[0].EntryID
	}
	if e = fr.ReverseEntry(ctx, entry, "manual-cost-reverse", "核对成本", "test"); e != nil {
		t.Fatal(e)
	}
	var fees int
	var net int64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),round(sum(amount)*100)::bigint FROM %s.customer_fee_items WHERE source_type='freight_actual'`, schema)).Scan(&fees, &net); e != nil || fees != 3 || net != -300 {
		t.Fatal("adjustment and reversal", fees, net, e)
	}
	check(15)
	qc.ExpectedFreightToken = quote.Freight.Token
	if _, e = r.QuoteFulfillmentFees(ctx, qc); e == nil {
		t.Fatal("confirmed freight editable")
	}
	// A client cannot masquerade as an employee even with the new protocol.
	qc.OrderID = 0
	qc.EmployeeAdjustment = false
	if _, e = r.QuoteFulfillmentFees(ctx, qc); e == nil {
		t.Fatal("customer explicit charge accepted")
	}
}

func TestOrderFreightV2PendingConfirmation(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	r := NewRepository(pool, schema)
	sales := salespg.NewRepository(pool, schema)
	amount := int64(2000)
	qc := app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "sale", FreightProtocol: domain.OrderProtocol, EmployeeAdjustment: true, Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}, FreightOverride: &domain.Override{ManualAmountCents: &amount}}
	quote, e := r.QuoteFulfillmentFees(ctx, qc)
	if e != nil {
		t.Fatal(e)
	}
	product := int64(910)
	price := 30.0
	cmd := salesapp.SaveOrderCommand{Actor: "test", CustomerID: 501, RequireConfirmation: true, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: 1, ShipStatusID: 1, ShippingProvince: "云南省", FreightProtocol: domain.OrderProtocol, FreightQuoteToken: quote.Freight.Token, FreightOverride: qc.FreightOverride, ShippingAmount: 20, Items: []salesapp.OrderItemCommand{{ProductID: &product, BomSpecID: 7031, BomVariantID: 7032, Name: "待确认运费测试", Units: 2, SalesUnit: "袋", ManualPrice: &price}}}
	result, e := sales.SaveOrder(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	var actual, ship float64
	var revision int64
	var snapshots int
	read := func() {
		t.Helper()
		if e := pool.QueryRow(ctx, fmt.Sprintf(`SELECT grand_total::float8,shipping_amount::float8,confirmation_revision FROM %s.orders WHERE id=$1`, schema), result.OrderID).Scan(&actual, &ship, &revision); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_freight_snapshots WHERE order_id=$1`, schema), result.OrderID).Scan(&snapshots); e != nil {
			t.Fatal(e)
		}
	}
	acceptedViews, e := freightpg.NewRepository(pool, schema).OrderViews(ctx, pool, []int64{result.OrderID})
	if e != nil || acceptedViews[result.OrderID] != nil {
		t.Fatalf("pending freight leaked into accepted accounting view: %+v %v", acceptedViews, e)
	}
	views, e := freightpg.NewRepository(pool, schema).OrderEditorViews(ctx, pool, []int64{result.OrderID})
	if e != nil || views[result.OrderID] == nil || views[result.OrderID].CurrentReceivableCents != 2000 || views[result.OrderID].EditToken != "" {
		t.Fatalf("pending editor: %+v %v", views, e)
	}
	read()
	if actual != 0 || ship != 0 || snapshots != 0 {
		t.Fatalf("pending charge leaked: %v %v %v", actual, ship, snapshots)
	}
	if _, e = sales.ReviewOrder(ctx, salesapp.ReviewOrderCommand{OrderID: result.OrderID, Revision: revision, Decision: "accepted", Admin: true, Actor: "test"}); e != nil {
		t.Fatal(e)
	}
	read()
	if actual != 80 || ship != 20 || snapshots != 1 {
		t.Fatalf("accepted mismatch: %v %v %v", actual, ship, snapshots)
	}
}

func TestOrderFreightV2LegacyAndConcurrentImport(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	r := NewRepository(pool, schema)
	fr := freightpg.NewRepository(pool, schema)
	sales := salespg.NewRepository(pool, schema)
	if _, e := fr.SavePolicy(ctx, domain.Policy{CustomerID: 501, Scene: "sale", FreeShipping: true}, 0, "test"); e != nil {
		t.Fatal(e)
	}
	qc := app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "sale", Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}}
	quote, e := r.QuoteFulfillmentFees(ctx, qc)
	if e != nil {
		t.Fatal(e)
	}
	product := int64(910)
	price := 30.0
	cmd := salesapp.SaveOrderCommand{Actor: "test", CustomerID: 501, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: 1, ShipStatusID: 1, ShippingProvince: "云南省", FreightProtocol: domain.Protocol, FreightQuoteToken: quote.Freight.Token, Items: []salesapp.OrderItemCommand{{ProductID: &product, BomSpecID: 7031, BomVariantID: 7032, Name: "历史运费", Units: 2, SalesUnit: "袋", ManualPrice: &price}}}
	result, e := sales.SaveOrder(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	originalToken := quote.Freight.Token
	qc.OrderID = result.OrderID
	qc.FreightProtocol = domain.OrderProtocol
	qc.EmployeeAdjustment = true
	qc.ExpectedFreightToken = originalToken
	quote, e = r.QuoteFulfillmentFees(ctx, qc)
	if e != nil {
		t.Fatal(e)
	}
	cmd.EditID = result.OrderID
	cmd.FreightProtocol = domain.OrderProtocol
	cmd.ExpectedFreightToken = originalToken
	cmd.FreightQuoteToken = quote.Freight.Token
	cmd.Notes = "只改备注"
	if _, e = sales.SaveOrder(ctx, cmd); e != nil {
		t.Fatal(e)
	}
	var protocol, token string
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json->>'protocol',snapshot_json->>'token' FROM %s.order_freight_snapshots WHERE order_id=$1`, schema), result.OrderID).Scan(&protocol, &token); e != nil || protocol != domain.Protocol || token != originalToken {
		t.Fatalf("unrelated edit migrated: %s %s %v", protocol, token, e)
	}
	amount := int64(2000)
	notFree := false
	qc.FreightOverride = &domain.Override{ManualAmountCents: &amount, FreeShipping: &notFree}
	quote, e = r.QuoteFulfillmentFees(ctx, qc)
	if e != nil {
		t.Fatal(e)
	}
	cmd.FreightOverride = qc.FreightOverride
	cmd.FreightQuoteToken = quote.Freight.Token
	cmd.ShippingAmount = 20
	cmd.ShipTrackingNo = "RACE709"
	cmd.LogisticsCompanyID = 1
	if _, e = sales.SaveOrder(ctx, cmd); e != nil {
		t.Fatal(e)
	}
	originalToken = quote.Freight.Token
	b, e := fr.PreviewImport(ctx, 1, "race709", "race.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "RACE709", ActualCents: 1200}}, "test")
	if e != nil {
		t.Fatal(e)
	}
	amount = 1500
	qc.ExpectedFreightToken = originalToken
	quote, e = r.QuoteFulfillmentFees(ctx, qc)
	if e != nil {
		t.Fatal(e)
	}
	cmd.ExpectedFreightToken = originalToken
	cmd.FreightQuoteToken = quote.Freight.Token
	cmd.ShippingAmount = 15
	start := make(chan struct{})
	done := make(chan error, 2)
	go func() { <-start; _, e := sales.SaveOrder(ctx, cmd); done <- e }()
	go func() { <-start; _, e := fr.ConfirmImport(ctx, b.ID, "test"); done <- e }()
	close(start)
	successes := 0
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err == nil {
				successes++
			}
		case <-time.After(10 * time.Second):
			t.Fatal("edit/import deadlock")
		}
	}
	if successes != 1 {
		t.Fatalf("successes=%d want1", successes)
	}
}
