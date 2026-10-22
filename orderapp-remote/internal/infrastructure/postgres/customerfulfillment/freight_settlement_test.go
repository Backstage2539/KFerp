package customerfulfillment

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	domain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"strings"
	"testing"
	"time"
)

func TestFreightActualDeferredIdempotentAndDelta(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	r := freightpg.NewRepository(pool, schema)
	if err := r.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	var cid, oid int64
	if e := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name)VALUES('freight isolated')RETURNING id`, schema)).Scan(&cid); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,total_amount,grand_total,ship_tracking_no,logistics_company_id)VALUES('FREIGHT-ISOLATED',$1,300,300,'001234567890',1)RETURNING id`, schema), cid).Scan(&oid); e != nil {
		t.Fatal(e)
	}
	q := domain.Snapshot{Protocol: domain.Protocol, Policy: domain.Policy{CustomerID: cid, Scene: "stock", SurchargeCents: 1000}, SurchargeCents: 1000, Status: "pending_actual", EstimateKnown: true, EstimatedTotalCents: 2000}
	q.Token = domain.Token(q)
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.SaveSnapshotTx(ctx, tx, oid, q, "test"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	b, e := r.PreviewImport(ctx, 1, "file1", "test.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "001234567890", ActualCents: 1200}}, "test")
	if e != nil {
		t.Fatal(e)
	}
	if len(b.Rows) != 1 || b.Rows[0].DeltaCents != 2200 || b.Rows[0].OrderID != oid {
		t.Fatalf("preview %+v", b)
	}
	b, e = r.ConfirmImport(ctx, b.ID, "test")
	if e != nil {
		t.Fatal(e)
	}
	if b.Status != "confirmed" {
		t.Fatal(b)
	}
	if _, e = r.ConfirmImport(ctx, b.ID, "test"); e != nil {
		t.Fatal(e)
	}
	b, e = r.PreviewImport(ctx, 1, "file2", "test2.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "001234567890", ActualCents: 1300}}, "test")
	if e != nil || b.Rows[0].DeltaCents != 100 {
		t.Fatalf("correction %+v %v", b, e)
	}
	b, e = r.ConfirmImport(ctx, b.ID, "test")
	if e != nil {
		t.Fatal(e)
	}
	var cents, shipping int64
	var goods float64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(sum(amount)*100)::bigint FROM %s.customer_fee_items WHERE customer_id=$1 AND source_type='freight_actual'`, schema), cid).Scan(&cents); e != nil || cents != 2300 {
		t.Fatalf("billed=%d %v", cents, e)
	}
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(shipping_amount*100)::bigint,total_amount::float8 FROM %s.orders WHERE id=$1`, schema), oid).Scan(&shipping, &goods); e != nil || shipping != 0 || goods != 300 {
		t.Fatalf("order changed %d %f %v", shipping, goods, e)
	}
	account, e := NewRepository(pool, schema).CustomerAccount(ctx, app.AccountQuery{CustomerID: cid})
	if e != nil || account.Summary.FeeShippingCents != 2300 {
		t.Fatalf("account %+v %v", account.Summary, e)
	}
	if len(account.Fees) != 2 || account.Fees[0].OrderID != oid || account.Fees[0].IncludedInOrder {
		t.Fatalf("independent linked freight %+v", account.Fees)
	}
	b, e = r.PreviewImport(ctx, 1, "file3", "duplicate.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "001234567890", ActualCents: 1300}}, "test")
	if e != nil || b.Rows[0].DeltaCents != 0 {
		t.Fatal(b, e)
	}
	var otherCustomer int64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name)VALUES('freight cross customer')RETURNING id`, schema)).Scan(&otherCustomer); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_tracking_no='' WHERE id=$1`, schema), oid); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,ship_tracking_no,logistics_company_id)VALUES('OTHER-FREIGHT-ORDER',$1,'001234567890',1)`, schema), otherCustomer); e != nil {
		t.Fatal(e)
	}
	moved, e := r.PreviewImport(ctx, 1, "moved-tracking", "moved.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "001234567890", ActualCents: 1500}}, "test")
	if e != nil || moved.Rows[0].Error == "" {
		t.Fatalf("waybill moved across customer: %+v %v", moved, e)
	}

}

func TestFreightImportFreeLegacyConflictsReversalAndStalePreview(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	r := freightpg.NewRepository(pool, schema)
	var cid int64
	if e := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name)VALUES('freight edge isolated')RETURNING id`, schema)).Scan(&cid); e != nil {
		t.Fatal(e)
	}
	seed := func(no, track string, free, legacy bool) int64 {
		var oid int64
		if e := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,total_amount,grand_total,ship_tracking_no,logistics_company_id,shipping_amount)VALUES($1,$2,300,300,$3,1,$4)RETURNING id`, schema), no, cid, track, map[bool]int{true: 10, false: 0}[legacy]).Scan(&oid); e != nil {
			t.Fatal(e)
		}
		if !legacy {
			q := domain.Snapshot{Protocol: domain.Protocol, Policy: domain.Policy{CustomerID: cid, Scene: "stock", FreeShipping: free}, SurchargeCents: 1000, Status: "pending_actual"}
			if free {
				q.Status = "free"
			}
			q.Token = domain.Token(q)
			tx, e := pool.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if e = r.SaveSnapshotTx(ctx, tx, oid, q, "test"); e != nil {
				t.Fatal(e)
			}
			if e = tx.Commit(ctx); e != nil {
				t.Fatal(e)
			}
		}
		return oid
	}
	seed("FREIGHT-FREE", "FREE001", true, false)
	seed("FREIGHT-LEGACY", "LEGACY001", false, true)
	oid := seed("FREIGHT-RACE", "RACE001", false, false)
	b, e := r.PreviewImport(ctx, 1, "edges", "edges.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "FREE001", ActualCents: 1200}, {RowNo: 3, TrackingNo: "LEGACY001", ActualCents: 1200}, {RowNo: 4, TrackingNo: "UNKNOWN", ActualCents: 1200}, {RowNo: 5, TrackingNo: "DUP", ActualCents: 1200}, {RowNo: 6, TrackingNo: "DUP", ActualCents: 1300}}, "test")
	if e != nil {
		t.Fatal(e)
	}
	if b.Rows[0].DeltaCents != 0 || !b.Rows[0].FreeShipping || !b.Rows[1].Legacy || b.Rows[2].Error == "" || b.Rows[3].Error == "" || b.Rows[4].Error == "" {
		t.Fatal(b)
	}
	b, e = r.ConfirmImport(ctx, b.ID, "test")
	if e != nil {
		t.Fatal(e)
	}
	if b.Status != "partial" {
		t.Fatalf("unresolved rows must remain pending: %+v", b)
	}
	replay, e := r.PreviewImport(ctx, 1, "edges", "edges.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "FREE001", ActualCents: 1200}, {RowNo: 3, TrackingNo: "LEGACY001", ActualCents: 1200}, {RowNo: 4, TrackingNo: "UNKNOWN", ActualCents: 1200}, {RowNo: 5, TrackingNo: "DUP", ActualCents: 1200}, {RowNo: 6, TrackingNo: "DUP", ActualCents: 1300}}, "test")
	if e != nil || replay.Rows[0].EntryID != b.Rows[0].EntryID {
		t.Fatalf("processed rows were not preserved: %+v %v", replay, e)
	}
	preview := func(hash string, cents int64) freightpg.ImportBatch {
		v, e := r.PreviewImport(ctx, 1, hash, hash+".xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "RACE001", ActualCents: cents}}, "test")
		if e != nil {
			t.Fatal(e)
		}
		return v
	}

	b1, b2 := preview("race1", 1200), preview("race2", 1200)
	type outcome struct {
		batch freightpg.ImportBatch
		err   error
	}
	done := make(chan outcome, 2)
	for _, candidate := range []freightpg.ImportBatch{b1, b2} {
		go func(batch freightpg.ImportBatch) {
			v, e := r.ConfirmImport(ctx, batch.ID, "test")
			done <- outcome{v, e}
		}(candidate)
	}
	success, loserHash := 0, ""
	for n := 0; n < 2; n++ {
		result := <-done
		if result.err == nil {
			success++
		} else {
			if !strings.Contains(result.err.Error(), "变化") {
				t.Fatal(result.err)
			}
			if result.batch.ID == b1.ID {
				loserHash = "race1"
			} else {
				loserHash = "race2"
			}
		}
	}
	if success != 1 {
		t.Fatalf("concurrent bill confirmations=%d want1", success)
	}
	retry := preview(loserHash, 1200)
	if retry.Rows[0].DeltaCents != 0 {
		t.Fatal("re-preview stale", retry)
	}
	if _, e = r.ConfirmImport(ctx, retry.ID, "test"); e != nil {
		t.Fatal(e)
	}
	revision := preview("revision", 1300)
	revision, e = r.ConfirmImport(ctx, revision.ID, "test")
	if e != nil {
		t.Fatal(e)
	}
	entry := revision.Rows[0].EntryID
	if e = r.ReverseEntry(ctx, entry, "reverse-race", "账单核对撤销", "test"); e != nil {
		t.Fatal(e)
	}
	if e = r.ReverseEntry(ctx, entry, "reverse-race", "账单核对撤销", "test"); e != nil {
		t.Fatal(e)
	}
	var cents int64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(sum(amount)*100)::bigint FROM %s.customer_fee_items WHERE customer_id=$1`, schema), cid).Scan(&cents); e != nil || cents != 2400 {
		t.Fatalf("net bills %d %v", cents, e)
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fee_items SET status='settled' WHERE customer_id=$1`, schema), cid); e != nil {
		t.Fatal(e)
	}
	locked := preview("settled", 1500)
	if locked.Rows[0].Error != "" || locked.Rows[0].DeltaCents != 0 {
		t.Fatalf("unknown settled receipt must allow cost only: %+v", locked)
	}
	if _, e = r.ConfirmImport(ctx, locked.ID, "test"); e != nil {
		t.Fatal(e)
	}
	voidOrder := seed("FREIGHT-VOID", "VOID001", false, false)
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET is_void=true WHERE id=$1`, schema), voidOrder); e != nil {
		t.Fatal(e)
	}
	seed("FREIGHT-MULTI-A", "MULTI001", false, false)
	seed("FREIGHT-MULTI-B", "MULTI001", false, false)
	blocked, e := r.PreviewImport(ctx, 1, "void-ambiguous", "blocked.xlsx", []freightpg.ImportRow{{RowNo: 2, TrackingNo: "VOID001", ActualCents: 100}, {RowNo: 3, TrackingNo: "MULTI001", ActualCents: 100}}, "test")
	if e != nil || !strings.Contains(blocked.Rows[0].Error, "作废") || !strings.Contains(blocked.Rows[1].Error, "多个订单") {
		t.Fatal(blocked, e)
	}
	if _, e = r.ConfirmImport(ctx, blocked.ID, "test"); e == nil {
		t.Fatal("blocked rows posted")
	}
	var n int64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.freight_actual_shipments WHERE order_id=$1`, schema), oid).Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
}

func TestFreightOrdinarySalesFreezeEstimateAndRetainedHasNoFreight(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	configurePR703StockFees(t, pool, schema, 7031)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=454 WHERE id=7032`, schema)); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(pool, schema)
	fr := freightpg.NewRepository(pool, schema)
	templates, e := r.ListShippingFeeTemplates(ctx)
	if e != nil {
		t.Fatal(e)
	}
	p, e := fr.SavePolicy(ctx, domain.Policy{CustomerID: 501, Scene: "sale", TemplateID: templates[0].ID, SurchargeCents: 1000}, 0, "test")
	if e != nil {
		t.Fatal(e)
	}
	quote, e := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "sale", Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}})
	if e != nil || quote.Freight.EstimatedTotalCents != 2000 || quote.ServiceFeeCents != 0 {
		t.Fatal(quote, e)
	}
	product := int64(910)
	price := 30.0
	cmd := salesapp.SaveOrderCommand{CustomerID: 501, OrderDate: time.Now(), DocumentDate: time.Now(), PayStatusID: 1, ShipStatusID: 1, Actor: "test", ShippingProvince: "云南省", FreightQuoteToken: quote.Freight.Token, Items: []salesapp.OrderItemCommand{{ProductID: &product, BomSpecID: 7031, BomVariantID: 7032, Name: "独立运费测试", Units: 2, SalesUnit: "袋", ManualPrice: &price}}}
	sales := postgressales.NewRepository(pool, schema)
	if _, e = sales.SaveOrder(ctx, cmd); e == nil || !strings.Contains(e.Error(), "更新客户端") {
		t.Fatal("old ordinary client", e)
	}
	cmd.FreightProtocol = domain.Protocol
	result, e := sales.SaveOrder(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	var goods, ship, grand float64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT total_amount::float8,shipping_amount::float8,grand_total::float8 FROM %s.orders WHERE id=$1`, schema), result.OrderID).Scan(&goods, &ship, &grand); e != nil || goods != 60 || ship != 0 || grand != 60 {
		t.Fatal(goods, ship, grand, e)
	}
	var services int
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.customer_fee_items WHERE source_type='order' AND source_id=$1 AND fee_type='direct_ship_service'`, schema), result.OrderID).Scan(&services); e != nil || services != 0 {
		t.Fatal("ordinary created dropship service", services, e)
	}
	// Legacy published specs can remain missing: ordinary orders record unknown
	// estimates and never substitute the recipe/net-content weight.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=NULL WHERE id=7032`, schema)); err != nil {
		t.Fatal(err)
	}
	unknown, err := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "sale", Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}})
	if err != nil || unknown.Freight.EstimateKnown || !strings.Contains(unknown.Freight.CostPendingReason, "物流重量") {
		t.Fatalf("ordinary missing weight=%+v err=%v", unknown, err)
	}
	cmd.FreightQuoteToken = unknown.Freight.Token
	missingResult, err := sales.SaveOrder(ctx, cmd)
	if err != nil {
		t.Fatalf("ordinary missing weight must save: %v", err)
	}
	var missing *float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT logistics_item_weight_g FROM %s.order_items WHERE order_id=$1`, schema), missingResult.OrderID).Scan(&missing); err != nil || missing != nil {
		t.Fatalf("unknown weight replaced: %v %v", missing, err)
	}
	p.FreeShipping = true
	if _, e = fr.SavePolicy(ctx, p, p.Revision, "test"); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.spec_conversion_groups`, schema)); e != nil {
		t.Fatal(e)
	}
	free, e := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "sale", Province: "未提供地区", Items: []app.FulfillmentFeeItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 2}}})
	if e != nil || free.Freight.Status != "free" || free.Freight.EstimateKnown {
		t.Fatal("free unknown", free, e)
	}
	cmd.ShippingProvince = "未提供地区"
	cmd.FreightQuoteToken = free.Freight.Token
	if _, e = sales.SaveOrder(ctx, cmd); e != nil {
		t.Fatal("free save unknown cost", e)
	}
	cmd.DeliveryMode = "customer_stock"
	cmd.DeliveryWarehouseCode = "DEV-E2E-A1"
	cmd.FreightProtocol = ""
	cmd.FreightQuoteToken = ""
	retained, e := sales.SaveOrder(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	var n int
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_freight_snapshots WHERE order_id=$1`, schema), retained.OrderID).Scan(&n); e != nil || n != 0 {
		t.Fatal("retained charged freight", n, e)
	}
}
