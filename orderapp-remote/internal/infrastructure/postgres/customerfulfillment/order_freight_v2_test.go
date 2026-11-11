package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	domain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	"strings"
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

func TestOrderFreightAutoCurrentUsesGrossParcelWeightAndSavesSnapshot(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	configurePR703StockFees(t, pool, schema, 7031)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=NULL WHERE id=7032`, schema)); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(pool, schema)
	fr := freightpg.NewRepository(pool, schema)
	templates, err := r.ListShippingFeeTemplates(ctx)
	if err != nil || len(templates) == 0 {
		t.Fatalf("templates=%v err=%v", templates, err)
	}
	if _, err = fr.SavePolicy(ctx, domain.Policy{CustomerID: 501, Scene: "sale", TemplateID: templates[0].ID, SurchargeCents: 200}, 0, "employee:703"); err != nil {
		t.Fatal(err)
	}
	command := app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "sale", FreightProtocol: domain.OrderProtocol, EmployeeAdjustment: true, Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}, FreightOverride: &domain.Override{ChargeMode: "auto_current"}}
	quote, err := r.QuoteFulfillmentFees(ctx, command)
	if err != nil || quote.Freight == nil || quote.Freight.EstimateKnown || quote.Freight.CurrentReceivableCents != 0 || len(quote.Freight.ItemWeights) != 1 {
		t.Fatalf("unknown-weight preview=%+v err=%v", quote.Freight, err)
	}
	product, price := int64(910), 30.0
	base := salesapp.SaveOrderCommand{Actor: "mini-employee:7:销售甲", CustomerID: 501, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: 1, ShipStatusID: 1, ShippingProvince: "云南省", FreightProtocol: domain.OrderProtocol, FreightQuoteToken: quote.Freight.Token, FreightOverride: command.FreightOverride, PaymentShippingAmount: 1, Items: []salesapp.OrderItemCommand{{ProductID: &product, BomSpecID: 7031, BomVariantID: 7032, Name: "隔离运费规格", Units: 2, SalesUnit: "袋", ManualPrice: &price}}}
	if _, err = salespg.NewRepository(pool, schema).SaveOrder(ctx, base); err == nil || err.Error() != "商品缺少物流重量，请填写本单包裹总重量或手动填写运费" {
		t.Fatalf("unknown auto quote should require weight: %v", err)
	}
	parcel, templateID := int64(2300), templates[0].ID
	command.FreightOverride = &domain.Override{ChargeMode: "auto_current", ParcelWeightG: &parcel, TemplateID: &templateID}
	quote, err = r.QuoteFulfillmentFees(ctx, command)
	if err != nil || quote.Freight == nil || !quote.Freight.EstimateKnown || quote.Freight.Estimate.WeightG != 2300 || quote.Freight.ParcelWeightG != 2300 || quote.Freight.WeightSource != "order_override" || quote.Freight.CurrentReceivableCents != quote.Freight.EstimatedTotalCents {
		t.Fatalf("gross parcel quote=%+v err=%v", quote.Freight, err)
	}
	base.FreightOverride = command.FreightOverride
	base.FreightQuoteToken = quote.Freight.Token
	base.ShippingAmount = float64(quote.Freight.CurrentReceivableCents) / 100
	result, err := salespg.NewRepository(pool, schema).SaveOrder(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	var shipping, grand float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT shipping_amount::float8,grand_total::float8 FROM %s.orders WHERE id=$1`, schema), result.OrderID).Scan(&shipping, &grand); err != nil || shipping != base.ShippingAmount || grand != 60+base.ShippingAmount {
		t.Fatalf("saved order shipping=%v grand=%v want=%v err=%v", shipping, grand, base.ShippingAmount, err)
	}
	var snapshot domain.Snapshot
	var snapshotRaw []byte
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.order_freight_snapshots WHERE order_id=$1`, schema), result.OrderID).Scan(&snapshotRaw); err != nil || json.Unmarshal(snapshotRaw, &snapshot) != nil || snapshot.ParcelWeightG != 2300 || snapshot.CurrentReceivableCents != quote.Freight.CurrentReceivableCents || snapshot.WeightSource != "order_override" {
		t.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
	var requirement string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT meta->>'requirement' FROM %s.audit_logs WHERE entity_type='order_freight' AND entity_id=$1 ORDER BY id DESC LIMIT 1`, schema), result.OrderID).Scan(&requirement); err != nil || requirement != "PR-720" {
		t.Fatalf("freight audit requirement=%q err=%v", requirement, err)
	}
	orderSales := salespg.NewRepository(pool, schema)
	beforeForm, err := orderSales.OrderForm(ctx, result.OrderID)
	if err != nil || beforeForm.EditData == nil || len(beforeForm.EditData.Items) != 1 {
		t.Fatalf("read newly saved miniapp-style order: edit=%+v err=%v", beforeForm.EditData, err)
	}
	beforeItem := beforeForm.EditData.Items[0]
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET status='draft' WHERE id=7030`, schema)); err != nil {
		t.Fatal("retire original BOM version", err)
	}
	manualCents := int64(1000)
	manualOverride := &domain.Override{ManualAmountCents: &manualCents}
	command.OrderID = result.OrderID
	command.ExpectedFreightToken = quote.Freight.Token
	command.FreightOverride = manualOverride
	manualQuote, err := r.QuoteFulfillmentFees(ctx, command)
	if err != nil || manualQuote.Freight == nil || manualQuote.Freight.CurrentReceivableCents != 1000 {
		t.Fatalf("manual ten-yuan edit quote=%+v err=%v", manualQuote.Freight, err)
	}
	edit := base
	edit.EditID = result.OrderID
	edit.ExpectedFreightToken = quote.Freight.Token
	edit.FreightQuoteToken = manualQuote.Freight.Token
	edit.FreightOverride = manualOverride
	edit.ShippingAmount = 10
	edit.Items[0].ItemID = beforeItem.ItemID
	edit.Items[0].PreserveExistingSnapshot = true
	edit.Items[0].Unit = beforeItem.Unit
	edit.Items[0].BeanListPublicationID = beforeItem.BeanListPublicationID
	edit.Items[0].BeanListVersionNo = beforeItem.BeanListVersionNo
	edit.Items[0].PriceSourceJSON = beforeItem.PriceSourceJSON
	edit.Items[0].ProductKind = beforeItem.ProductKind
	edit.Items[0].SalesUnit = beforeItem.SalesUnit
	edit.Items[0].UnitBagCount = beforeItem.UnitBagCount
	edit.Items[0].UnitBeanG = 0
	if _, err = orderSales.SaveOrder(ctx, edit); err != nil {
		t.Fatalf("miniapp-style ten-yuan edit failed: %v", err)
	}
	afterForm, err := orderSales.OrderForm(ctx, result.OrderID)
	if err != nil || afterForm.EditData == nil || len(afterForm.EditData.Items) != 1 {
		t.Fatalf("read edited miniapp-style order: edit=%+v err=%v", afterForm.EditData, err)
	}
	afterItem := afterForm.EditData.Items[0]
	if afterItem.ProductID != beforeItem.ProductID || afterItem.BomSpecID != beforeItem.BomSpecID || afterItem.BomVariantID != beforeItem.BomVariantID || afterItem.Qty != beforeItem.Qty || afterItem.UnitPrice != beforeItem.UnitPrice || afterItem.LineTotal != beforeItem.LineTotal || afterItem.BeanListPublicationID != beforeItem.BeanListPublicationID {
		t.Fatalf("freight-only edit changed stored item: before=%+v after=%+v", beforeItem, afterItem)
	}
	var editedShipping, editedTotal, freightPaid float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT shipping_amount::float8,grand_total::float8,payment_shipping_amount::float8 FROM %s.orders WHERE id=$1`, schema), result.OrderID).Scan(&editedShipping, &editedTotal, &freightPaid); err != nil || editedShipping != 10 || editedTotal != 70 || freightPaid != 1 {
		t.Fatalf("miniapp-style save totals/receipt shipping=%v total=%v already_received=%v err=%v", editedShipping, editedTotal, freightPaid, err)
	}
	var operationCount int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_audit_logs WHERE order_id=$1 AND actor=$2`, schema), result.OrderID, base.Actor).Scan(&operationCount); err != nil || operationCount == 0 {
		t.Fatalf("miniapp order edit missing operation log count=%d err=%v", operationCount, err)
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

func TestNewOrderInitialReceiptAndReplay(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	r := NewRepository(pool, schema)
	sales := salespg.NewRepository(pool, schema)
	amount := int64(2000)
	override := &domain.Override{ManualAmountCents: &amount}
	quote, err := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "sale", FreightProtocol: domain.OrderProtocol, EmployeeAdjustment: true, Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}, FreightOverride: override})
	if err != nil {
		t.Fatal(err)
	}
	var partial int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name='预付款（付款未完成）' LIMIT 1`, schema)).Scan(&partial); err != nil {
		t.Fatal(err)
	}
	price, deposit := 30.0, 20.0
	product := int64(910)
	cmd := salesapp.SaveOrderCommand{Actor: "test", CustomerID: 501, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: partial, ShipStatusID: 1, ShippingProvince: "云南省", FreightProtocol: domain.OrderProtocol, FreightQuoteToken: quote.Freight.Token, FreightOverride: override, ShippingAmount: 20, PrepaymentAmount: &deposit, PaymentMethod: "现金", ReceiptDate: "2026-11-10", CustomerRequestID: "receipt-order-request-1", CustomerRequestHash: "same-hash", Items: []salesapp.OrderItemCommand{{ProductID: &product, BomSpecID: 7031, BomVariantID: 7032, Name: "测试", Units: 2, SalesUnit: "袋", ManualPrice: &price}}}
	cmd.PaymentGoodsAmount = 20
	var paidID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name='已付款' LIMIT 1`, schema)).Scan(&paidID); err != nil {
		t.Fatal(err)
	}
	zeroPaid := 0.0
	badPaid := cmd
	badPaid.CustomerRequestID = "receipt-order-bad-paid"
	badPaid.CustomerRequestHash = "bad-paid"
	badPaid.PayStatusID = paidID
	badPaid.PrepaymentAmount = &zeroPaid
	if _, err = sales.SaveOrder(ctx, badPaid); err == nil {
		t.Fatal("已付款 80 实收 20 incorrectly accepted")
	}
	badSplit := cmd
	badSplit.CustomerRequestID = "receipt-order-bad-split"
	badSplit.CustomerRequestHash = "bad-split"
	badSplit.PaymentGoodsAmount = 5
	if _, err = sales.SaveOrder(ctx, badSplit); err == nil {
		t.Fatal("部分收款累计与分项不符 incorrectly accepted")
	}
	fullPaid := 80.0
	goodFull := cmd
	goodFull.CustomerRequestID = "receipt-order-full-paid"
	goodFull.CustomerRequestHash = "full-paid"
	goodFull.PayStatusID = paidID
	goodFull.PrepaymentAmount = &fullPaid
	goodFull.PaymentGoodsAmount = 60
	goodFull.PaymentShippingAmount = 20
	full, err := sales.SaveOrder(ctx, goodFull)
	if err != nil {
		t.Fatalf("full receipt: %v", err)
	}
	var fullPre, fullGoods, fullShipping float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT prepayment_amount::float8,payment_goods_amount::float8,payment_shipping_amount::float8 FROM %s.orders WHERE id=$1`, schema), full.OrderID).Scan(&fullPre, &fullGoods, &fullShipping); err != nil || fullPre != 80 || fullGoods != 60 || fullShipping != 20 {
		t.Fatalf("full order amount=%v/%v/%v err=%v", fullPre, fullGoods, fullShipping, err)
	}
	legacyFull := goodFull
	legacyFull.CustomerRequestID = ""
	legacyFull.CustomerRequestHash = ""
	legacyFull.ReceiptDate = ""
	legacyFull.PrepaymentAmount = &zeroPaid
	legacyFullOrder, err := sales.SaveOrder(ctx, legacyFull)
	if err != nil {
		t.Fatalf("legacy full order: %v", err)
	}
	increasedCents := int64(4000)
	increasedOverride := &domain.Override{ManualAmountCents: &increasedCents}
	increasedQuote, err := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, OrderID: legacyFullOrder.OrderID, ExpectedFreightToken: quote.Freight.Token, FulfillmentMode: "sale", FreightProtocol: domain.OrderProtocol, EmployeeAdjustment: true, Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}, FreightOverride: increasedOverride})
	if err != nil {
		t.Fatal(err)
	}
	freightEdit := legacyFull
	freightEdit.EditID = legacyFullOrder.OrderID
	freightEdit.CustomerRequestID = ""
	freightEdit.CustomerRequestHash = ""
	freightEdit.ExpectedFreightToken = quote.Freight.Token
	freightEdit.FreightQuoteToken = increasedQuote.Freight.Token
	freightEdit.FreightOverride = increasedOverride
	freightEdit.ShippingAmount = 40
	if _, err = sales.SaveOrder(ctx, freightEdit); err != nil {
		t.Fatalf("increase freight: %v", err)
	}
	var newPayName string
	var newTotal, newPre float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT ps.name,o.grand_total::float8,o.prepayment_amount::float8 FROM %[1]s.orders o JOIN %[1]s.pay_statuses ps ON ps.id=o.pay_status_id WHERE o.id=$1`, schema), legacyFullOrder.OrderID).Scan(&newPayName, &newTotal, &newPre); err != nil || newPayName != "预付款（付款未完成）" || newTotal != 100 || newPre != 80 {
		t.Fatalf("increase freight result=%s %.2f %.2f err=%v", newPayName, newTotal, newPre, err)
	}
	bad := cmd
	bad.ReceiptDate = "2999-01-01"
	if _, err = sales.SaveOrder(ctx, bad); err == nil {
		t.Fatal("future date accepted")
	}
	first, err := sales.SaveOrder(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	again, err := sales.SaveOrder(ctx, cmd)
	if err != nil || again.OrderID != first.OrderID || !again.Replayed {
		t.Fatalf("replay=%+v err=%v", again, err)
	}
	inconsistent := cmd
	inconsistent.CustomerRequestID = "receipt-order-inconsistent"
	inconsistent.CustomerRequestHash = "inconsistent-hash"
	inconsistent.PaymentGoodsAmount = 5
	inconsistent.ReceiptDate = ""
	warned, err := sales.SaveOrder(ctx, inconsistent)
	if err != nil || warned.ReceiptWarning == "" {
		t.Fatalf("expected warning: %+v %v", warned, err)
	}
	warnedAgain, err := sales.SaveOrder(ctx, inconsistent)
	if err != nil || warnedAgain.ReceiptWarning != warned.ReceiptWarning {
		t.Fatalf("replayed warning lost: %+v %v", warnedAgain, err)
	}
	var count int
	var actual, total int64
	var date string
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),max(actual_cents),max(receivable_cents),max(receipt_date::text) FROM %s.cash_receipts WHERE source_type='initial_order' AND source_id=$1`, schema), first.OrderID).Scan(&count, &actual, &total, &date)
	if err != nil || count != 1 || actual != 2000 || total != 8000 || date != "2026-11-10" {
		t.Fatalf("initial receipt count=%d actual=%d total=%d date=%s err=%v", count, actual, total, date, err)
	}
	history, err := sales.OrderReceipts(ctx, first.OrderID)
	if err != nil || len(history) != 1 || history[0].ActualCents != 2000 {
		t.Fatalf("receipt history=%+v err=%v", history, err)
	}
	changed := cmd
	changed.CustomerRequestHash = "different-hash"
	if _, err = sales.SaveOrder(ctx, changed); err == nil {
		t.Fatal("changed replay accepted")
	}
	cross := cmd
	cross.CustomerID = 502
	if _, err = sales.SaveOrder(ctx, cross); err == nil || !strings.Contains(err.Error(), "请求标识") {
		t.Fatalf("cross-customer replay accepted or wrong error: %v", err)
	}
	cross = cmd
	cross.Actor = "another-employee"
	if _, err = sales.SaveOrder(ctx, cross); err == nil || !strings.Contains(err.Error(), "请求标识") {
		t.Fatalf("cross-actor replay accepted or wrong error: %v", err)
	}
	legacy := cmd
	legacy.CustomerRequestID = "receipt-order-request-legacy"
	legacy.ReceiptDate = ""
	legacy.CustomerRequestHash = "legacy-hash"
	old, err := sales.SaveOrder(ctx, legacy)
	if err != nil {
		t.Fatal(err)
	}
	var unknown bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT receipt_date IS NULL FROM %s.cash_receipts WHERE source_type='initial_order' AND source_id=$1`, schema), old.OrderID).Scan(&unknown); err != nil || !unknown {
		t.Fatalf("legacy date unknown=%v err=%v", unknown, err)
	}
	unkeyed := cmd
	unkeyed.CustomerRequestID = ""
	unkeyed.CustomerRequestHash = ""
	unkeyed.ReceiptDate = ""
	compat, err := sales.SaveOrder(ctx, unkeyed)
	if err != nil || compat.ReceiptWarning == "" {
		t.Fatalf("legacy warning=%+v err=%v", compat, err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.cash_receipts WHERE source_type='initial_order' AND source_id=$1`, schema), compat.OrderID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unkeyed receipt count=%d err=%v", count, err)
	}
	backfill := cmd
	backfill.BackfillMode = true
	backfill.CustomerRequestID = "receipt-order-backfill"
	backfill.CustomerRequestHash = "backfill-hash"
	historical, err := sales.SaveOrder(ctx, backfill)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.cash_receipts WHERE source_type='initial_order' AND source_id=$1`, schema), historical.OrderID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("historical receipt=%d err=%v", count, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rollback := cmd
	rollback.CustomerRequestID = "receipt-order-request-rollback"
	rollback.CustomerRequestHash = "rollback-hash"
	temporary, err := sales.WithOrderTransaction(tx).SaveOrder(ctx, rollback)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.cash_receipts WHERE source_type='initial_order' AND source_id=$1`, schema), temporary.OrderID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback retained receipt=%d err=%v", count, err)
	}
	pending := cmd
	pending.RequireConfirmation = true
	pending.CustomerRequestID = "receipt-order-request-pending"
	pending.CustomerRequestHash = "pending-hash"
	if _, err = sales.SaveOrder(ctx, pending); err == nil || !strings.Contains(err.Error(), "待首次确认") {
		t.Fatalf("pending receipt was accepted: %v", err)
	}
	zero := 0.0
	pending.PrepaymentAmount = &zero
	pending.PaymentGoodsAmount = 0
	pending.PaymentMethod = ""
	pending.PayStatusID = 1
	pending.ReceiptDate = ""
	pendingOrder, err := sales.SaveOrder(ctx, pending)
	if err != nil {
		t.Fatal(err)
	}
	var status string
	var orderPrepayment float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT confirmation_status,prepayment_amount::float8 FROM %s.orders WHERE id=$1`, schema), pendingOrder.OrderID).Scan(&status, &orderPrepayment); err != nil || status != "pending" || orderPrepayment != 0 {
		t.Fatalf("pending order=%s prepayment=%v err=%v", status, orderPrepayment, err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.cash_receipts WHERE source_type='initial_order' AND source_id=$1 AND actual_cents=2000`, schema), pendingOrder.OrderID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("pending receipt=%d err=%v", count, err)
	}
	badEdit := cmd
	badEdit.EditID = pendingOrder.OrderID
	badEdit.RequireConfirmation = true
	badEdit.CustomerRequestID = ""
	badEdit.CustomerRequestHash = ""
	pendingQuote, err := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, OrderID: pendingOrder.OrderID, FulfillmentMode: "sale", FreightProtocol: domain.OrderProtocol, EmployeeAdjustment: true, Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}, FreightOverride: override})
	if err != nil {
		t.Fatal(err)
	}
	badEdit.FreightQuoteToken = pendingQuote.Freight.Token
	editTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	badEdit.ExpectedEditRevision, err = salespg.CurrentOrderEditRevisionTx(ctx, editTx, schema, pendingOrder.OrderID)
	_ = editTx.Rollback(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sales.SaveOrder(ctx, badEdit); err == nil || !strings.Contains(err.Error(), "待首次确认") {
		t.Fatalf("pending edit with receipt accepted: %v", err)
	}
	var revision int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT confirmation_revision FROM %s.orders WHERE id=$1`, schema), pendingOrder.OrderID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if _, err = sales.ReviewOrder(ctx, salesapp.ReviewOrderCommand{OrderID: pendingOrder.OrderID, Revision: revision, Decision: "accepted", Admin: true, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT prepayment_amount::float8 FROM %s.orders WHERE id=$1`, schema), pendingOrder.OrderID).Scan(&orderPrepayment); err != nil || orderPrepayment != 0 {
		t.Fatalf("accepted prepayment=%v err=%v", orderPrepayment, err)
	}
	editTx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	acceptedRevision, err := salespg.CurrentOrderEditRevisionTx(ctx, editTx, schema, pendingOrder.OrderID)
	_ = editTx.Rollback(ctx)
	if err != nil {
		t.Fatal(err)
	}
	receiptCommand := salesapp.OrderReceiptCommand{OrderID: pendingOrder.OrderID, EditRevision: acceptedRevision, ActualCents: 2000, ExpectedDueCents: 8000, ReceiptDate: "2026-11-10", PaymentMethod: "现金", IdempotencyKey: "accepted-order-receipt", Actor: "test"}
	registered, err := sales.RegisterOrderReceipt(ctx, receiptCommand)
	if err != nil {
		t.Fatalf("accepted receipt: %v", err)
	}
	replayedReceipt, err := sales.RegisterOrderReceipt(ctx, receiptCommand)
	if err != nil || !replayedReceipt.Reused || replayedReceipt.ReceiptID != registered.ReceiptID {
		t.Fatalf("receipt replay=%+v err=%v", replayedReceipt, err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT prepayment_amount::float8 FROM %s.orders WHERE id=$1`, schema), pendingOrder.OrderID).Scan(&orderPrepayment); err != nil || orderPrepayment != 20 {
		t.Fatalf("accepted actual receipt prepayment=%v err=%v", orderPrepayment, err)
	}
	pending.CustomerRequestID = "receipt-order-request-rejected"
	pending.CustomerRequestHash = "rejected-hash"
	rejected, err := sales.SaveOrder(ctx, pending)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT confirmation_revision FROM %s.orders WHERE id=$1`, schema), rejected.OrderID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if _, err = sales.ReviewOrder(ctx, salesapp.ReviewOrderCommand{OrderID: rejected.OrderID, Revision: revision, Decision: "rejected", Reason: "测试", Admin: true, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	badEdit.EditID = rejected.OrderID
	pendingQuote, err = r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, OrderID: rejected.OrderID, FulfillmentMode: "sale", FreightProtocol: domain.OrderProtocol, EmployeeAdjustment: true, Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}, FreightOverride: override})
	if err != nil {
		t.Fatal(err)
	}
	badEdit.FreightQuoteToken = pendingQuote.Freight.Token
	editTx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	badEdit.ExpectedEditRevision, err = salespg.CurrentOrderEditRevisionTx(ctx, editTx, schema, rejected.OrderID)
	_ = editTx.Rollback(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sales.SaveOrder(ctx, badEdit); err == nil || !strings.Contains(err.Error(), "待首次确认") {
		t.Fatalf("rejected resubmit with receipt accepted: %v", err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.cash_receipts WHERE source_type='initial_order' AND source_id=$1`, schema), rejected.OrderID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rejected receipt=%d err=%v", count, err)
	}
	resolved, err := sales.CancelUncommittedOrderRequest(ctx, salesapp.OrderRequestCancelCommand{RequestID: cmd.CustomerRequestID, RequestActor: cmd.Actor, RequestHash: cmd.CustomerRequestHash, Actor: "admin", Reason: "核对已提交"})
	if err != nil || resolved.Status != "committed" || resolved.OrderID != first.OrderID {
		t.Fatalf("committed lookup: %+v %v", resolved, err)
	}
	cancelled := cmd
	cancelled.CustomerRequestID = "receipt-order-request-cancelled"
	cancelled.CustomerRequestHash = "cancelled-hash"
	resolved, err = sales.CancelUncommittedOrderRequest(ctx, salesapp.OrderRequestCancelCommand{RequestID: cancelled.CustomerRequestID, RequestActor: cancelled.Actor, RequestHash: cancelled.CustomerRequestHash, Actor: "admin", Reason: "核对无订单"})
	if err != nil || resolved.Status != "cancelled" {
		t.Fatalf("cancel: %+v %v", resolved, err)
	}
	if _, err = sales.SaveOrder(ctx, cancelled); err == nil || !strings.Contains(err.Error(), "已解除") {
		t.Fatalf("late request accepted: %v", err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='order_create_request' AND action='cancel_uncommitted' AND meta->>'request_id'=$1`, schema), cancelled.CustomerRequestID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cancel audit count=%d err=%v", count, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_create_idempotency_keys WHERE request_id=$1`, schema), legacy.CustomerRequestID); err != nil {
		t.Fatal(err)
	}
	legacyResult, err := sales.CancelUncommittedOrderRequest(ctx, salesapp.OrderRequestCancelCommand{RequestID: legacy.CustomerRequestID, RequestActor: "unknown", RequestHash: "unknown", Actor: "admin", Reason: "核对历史订单"})
	if err != nil || legacyResult.Status != "committed" || legacyResult.OrderID != old.OrderID {
		t.Fatalf("legacy lookup=%+v err=%v", legacyResult, err)
	}
	race := cmd
	race.CustomerRequestID = "receipt-order-request-race"
	race.CustomerRequestHash = "race-hash"
	lockTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lockTx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "order-create-global:"+race.CustomerRequestID); err != nil {
		t.Fatal(err)
	}
	type cancelResult struct {
		result salesapp.OrderRequestResolution
		err    error
	}
	type saveResult struct {
		result salesapp.SaveOrderResult
		err    error
	}
	cancelDone := make(chan cancelResult, 1)
	saveDone := make(chan saveResult, 1)
	go func() {
		v, e := sales.CancelUncommittedOrderRequest(ctx, salesapp.OrderRequestCancelCommand{RequestID: race.CustomerRequestID, RequestActor: race.Actor, RequestHash: race.CustomerRequestHash, Actor: "admin", Reason: "并发核对"})
		cancelDone <- cancelResult{v, e}
	}()
	go func() { v, e := sales.SaveOrder(ctx, race); saveDone <- saveResult{v, e} }()
	if err = lockTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	cr, sr := <-cancelDone, <-saveDone
	if cr.err != nil || !((cr.result.Status == "committed" && sr.err == nil && cr.result.OrderID == sr.result.OrderID) || (cr.result.Status == "cancelled" && sr.err != nil && strings.Contains(sr.err.Error(), "已解除"))) {
		t.Fatalf("racing cancellation=%+v/%v save=%+v/%v", cr.result, cr.err, sr.result, sr.err)
	}
	inflight := cmd
	inflight.CustomerRequestID = "receipt-order-request-inflight"
	inflight.CustomerRequestHash = "inflight-hash"
	inflightTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	inflightOrder, err := sales.WithOrderTransaction(inflightTx).SaveOrder(ctx, inflight)
	if err != nil {
		_ = inflightTx.Rollback(ctx)
		t.Fatal(err)
	}
	inflightDone := make(chan cancelResult, 1)
	go func() {
		v, e := sales.CancelUncommittedOrderRequest(ctx, salesapp.OrderRequestCancelCommand{RequestID: inflight.CustomerRequestID, RequestActor: inflight.Actor, RequestHash: inflight.CustomerRequestHash, Actor: "admin", Reason: "核对在途请求"})
		inflightDone <- cancelResult{v, e}
	}()
	select {
	case early := <-inflightDone:
		_ = inflightTx.Rollback(ctx)
		t.Fatalf("resolved while original transaction in flight: %+v", early)
	case <-time.After(100 * time.Millisecond):
	}
	if err = inflightTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	inflightResolution := <-inflightDone
	if inflightResolution.err != nil || inflightResolution.result.Status != "committed" || inflightResolution.result.OrderID != inflightOrder.OrderID {
		t.Fatalf("inflight result=%+v err=%v", inflightResolution.result, inflightResolution.err)
	}
}
