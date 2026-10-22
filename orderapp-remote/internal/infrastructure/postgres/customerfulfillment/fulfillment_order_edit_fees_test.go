package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fulfillmentEditFeeFixture struct {
	r                        *Repository
	pool                     *pgxpool.Pool
	schema                   string
	customer, product, order int64
	quote                    app.FulfillmentFeeQuote
}

func newFulfillmentEditFeeFixture(t *testing.T) fulfillmentEditFeeFixture {
	t.Helper()
	ctx := context.Background()
	pool, schema := newMiniDirectShipTestDB(t)
	f := fulfillmentEditFeeFixture{r: NewRepository(pool, schema), pool: pool, schema: schema}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('编辑费用测试') RETURNING id`, schema)).Scan(&f.customer); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.products(name) VALUES('270g物流规格') RETURNING id`, schema)).Scan(&f.product); err != nil {
		t.Fatal(err)
	}
	seedManualFreightWeight(t, pool, schema, f.product, 270)
	tm, err := f.r.SaveShippingFeeTemplate(ctx, app.SaveShippingFeeTemplateCommand{Actor: "fee-test", Template: domain.Template{Name: "原运费模板", Active: true, PackagingWeightG: 500, Rules: []domain.LegacyRule{{Province: "*", FirstWeightG: 1000, FirstFeeCents: 610, AdditionalWeightG: 1000, AdditionalFeeCents: 200}}}})
	if err != nil {
		t.Fatal(err)
	}
	fr := freightpg.NewRepository(pool, schema)
	grams, basis, err := fr.WeightTotals(ctx, pool, f.customer, []freightpg.WeightItem{{ProductID: f.product, Qty: 8}}, false)
	if err != nil {
		t.Fatal(err)
	}
	freight, err := domain.Quote(domain.Policy{CustomerID: f.customer, Scene: "stock", Revision: 1, TemplateID: tm.ID, SurchargeCents: 0}, tm, domain.Destination{Province: "北京市", City: "北京市", District: "朝阳区"}, grams)
	if err != nil {
		t.Fatal(err)
	}
	freight = freightpg.AttachWeightBasis(freight, basis)
	f.quote = app.FulfillmentFeeQuote{CustomerID: f.customer, FulfillmentMode: "stock", SettingsRevision: 1, ShippingTemplateID: tm.ID, ShippingTemplateRevision: tm.Revision, Province: "北京市", City: "北京市", District: "朝阳区", PackageQty: 8, NetWeightG: 2160, PackagingWeightG: 500, ChargeableWeightG: 2660, FixedFeeCents: 100, PerPackageFeeCents: 25, CalculatedServiceFeeCents: 300, ServiceFeeCents: 300, CalculatedShippingFeeCents: 1010, Freight: &freight}
	f.quote.FeeQuoteToken = app.FulfillmentFeeQuoteToken(f.quote)
	if err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,portal_service_code,fulfillment_mode,total_amount,grand_total,payment_goods_amount,payment_shipping_amount) VALUES('FEE-EDIT',$1,'direct_ship','stock',0,0,0,0) RETURNING id`, schema), f.customer).Scan(&f.order); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = f.r.SaveFulfillmentOrderFeesTx(ctx, tx, f.customer, f.order, 0, "stock", f.quote, "fee-test"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f fulfillmentEditFeeFixture) command(qty int64) app.FulfillmentFeeQuoteCommand {
	return app.FulfillmentFeeQuoteCommand{CustomerID: f.customer, OrderID: f.order, FulfillmentMode: "stock", ExpectedFreightToken: f.quote.Freight.Token, Province: "北京市", City: "北京市", District: "朝阳区", Items: []app.FulfillmentFeeItem{{ProductID: f.product, Qty: qty}}, EmployeeAdjustment: true}
}

func (f fulfillmentEditFeeFixture) transaction(t *testing.T, fn func(pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = freightpg.LockOrderFreight(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=$1 FOR UPDATE`, f.schema), f.order); err != nil {
		t.Fatal(err)
	}
	fn(tx)
}

func TestFulfillmentOrderEditFeesPreserveFrozenBasisAndSaveOnce(t *testing.T) {
	f := newFulfillmentEditFeeFixture(t)
	ctx := context.Background()
	// Neither the latest rate nor a changed live BOM weight may reprice an old order.
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.shipping_fee_templates SET revision=revision+1,packaging_weight_g=5000,rules_json='[{"province":"*","first_weight_g":1000,"first_fee_cents":9900,"additional_weight_g":1000,"additional_fee_cents":9900}]'; UPDATE %s.production_bom_version_variants SET logistics_weight_g=9000`, f.schema, f.schema)); err != nil {
		t.Fatal(err)
	}
	f.transaction(t, func(tx pgx.Tx) {
		q, err := f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, f.command(8))
		if err != nil {
			t.Fatal(err)
		}
		if q.FeeQuoteToken != f.quote.FeeQuoteToken || q.Freight.Token != f.quote.Freight.Token {
			t.Fatal("opening an unchanged order replaced its frozen quote")
		}
		q, err = f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, f.command(12))
		if err != nil {
			t.Fatal(err)
		}
		if q.NetWeightG != 3240 || q.ChargeableWeightG != 3740 || q.CalculatedShippingFeeCents != 1210 || q.ServiceFeeCents != 400 || q.ShippingTemplateRevision != f.quote.ShippingTemplateRevision || q.Freight.Policy != f.quote.Freight.Policy {
			t.Fatalf("changed quote must retain original settings: %+v", q)
		}
		if err = f.r.saveFulfillmentOrderEditFeesTx(ctx, tx, f.customer, f.order, f.quote.FeeQuoteToken, f.quote.Freight.Token, q, "fee-editor"); err != nil {
			t.Fatal(err)
		}
		var count int
		var amount, shipping, goodsPaid, freightPaid float64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),sum(amount)::float8 FROM %s.customer_fee_items WHERE source_type='order' AND source_id=$1 AND fee_type='direct_ship_service'`, f.schema), f.order).Scan(&count, &amount); err != nil {
			t.Fatal(err)
		}
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT shipping_amount::float8,payment_goods_amount::float8,payment_shipping_amount::float8 FROM %s.orders WHERE id=$1`, f.schema), f.order).Scan(&shipping, &goodsPaid, &freightPaid); err != nil {
			t.Fatal(err)
		}
		if count != 1 || amount != 4 || shipping != 0 || goodsPaid != 0 || freightPaid != 0 {
			t.Fatalf("duplicate/reclassified receivable: count=%d amount=%v ship=%v paid=%v/%v", count, amount, shipping, goodsPaid, freightPaid)
		}
		var savedFee, savedFreight []byte
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT quote_json FROM %s.fulfillment_order_fee_snapshots WHERE order_id=$1`, f.schema), f.order).Scan(&savedFee); err != nil {
			t.Fatal(err)
		}
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.order_freight_snapshots WHERE order_id=$1`, f.schema), f.order).Scan(&savedFreight); err != nil {
			t.Fatal(err)
		}
		var fq app.FulfillmentFeeQuote
		var fs domain.Snapshot
		json.Unmarshal(savedFee, &fq)
		json.Unmarshal(savedFreight, &fs)
		if fq.FeeQuoteToken != q.FeeQuoteToken || fs.Token != q.Freight.Token {
			t.Fatal("fee and freight snapshots diverged")
		}
		if err = f.r.saveFulfillmentOrderEditFeesTx(ctx, tx, f.customer, f.order, f.quote.FeeQuoteToken, f.quote.Freight.Token, q, "fee-editor"); err == nil {
			t.Fatal("stale save accepted")
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatalf("fee and freight deferred constraints failed: %v", err)
		}
	})
}

func TestFulfillmentOrderEditFeesSettlementLocksOnlyAffectedAmounts(t *testing.T) {
	f := newFulfillmentEditFeeFixture(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fee_items SET settlement_batch_id=777 WHERE source_type='order' AND source_id=$1 AND fee_type='direct_ship_service'`, f.schema), f.order); err != nil {
		t.Fatal(err)
	}
	f.transaction(t, func(tx pgx.Tx) {
		c := f.command(8)
		c.City = "北京市"
		c.District = "海淀区"
		q, err := f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, c)
		if err != nil {
			t.Fatal(err)
		}
		if err = f.r.saveFulfillmentOrderEditFeesTx(ctx, tx, f.customer, f.order, f.quote.FeeQuoteToken, f.quote.Freight.Token, q, "fee-editor"); err != nil {
			t.Fatalf("unchanged settled service blocked address edit: %v", err)
		}
	})
	f.transaction(t, func(tx pgx.Tx) {
		q, err := f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, f.command(12))
		if err == nil {
			err = f.r.saveFulfillmentOrderEditFeesTx(ctx, tx, f.customer, f.order, f.quote.FeeQuoteToken, f.quote.Freight.Token, q, "fee-editor")
		}
		if err == nil {
			t.Fatal("settled service charge was overwritten")
		}
	})
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_shipments(carrier_id,tracking_no,order_id,customer_id,revision) VALUES(1,'FEE-LOCK',$1,$2,1)`, f.schema), f.order, f.customer); err != nil {
		t.Fatal(err)
	}
	f.transaction(t, func(tx pgx.Tx) {
		q, err := f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, f.command(8))
		if err != nil {
			t.Fatalf("unchanged billed quote lost: %v", err)
		}
		if q.FeeQuoteToken != f.quote.FeeQuoteToken {
			t.Fatal("unchanged billed quote mutated")
		}
		c := f.command(8)
		c.District = "海淀区"
		_, err = f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, c)
		if err == nil {
			t.Fatal("billed freight destination edited")
		}
	})
}

func TestFulfillmentOrderEditFeesRejectOverridesAndKeepExplicitZero(t *testing.T) {
	f := newFulfillmentEditFeeFixture(t)
	ctx := context.Background()
	f.transaction(t, func(tx pgx.Tx) {
		zero := int64(0)
		c := f.command(8)
		c.ServiceOverrideCents = &zero
		if _, err := f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, c); err == nil {
			t.Fatal("service override without reason accepted")
		}
		c.AdjustmentReason = "本单免服务费"
		q, err := f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, c)
		if err != nil {
			t.Fatal(err)
		}
		if q.ServiceFeeCents != 0 || q.CalculatedServiceFeeCents != 300 || q.CalculatedShippingFeeCents != 1010 {
			t.Fatal("explicit service zero lost")
		}
		c = f.command(8)
		c.ShippingOverrideCents = &zero
		if _, err = f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, c); err == nil {
			t.Fatal("deferred freight turned into manual current charge")
		}
		c = f.command(8)
		c.ExpectedFreightToken = "stale"
		if _, err = f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, c); err == nil {
			t.Fatal("stale quote accepted")
		}
	})
}

func TestFulfillmentOrderEditFeesNewItemsFreePolicyAndHistoricalReceipts(t *testing.T) {
	f := newFulfillmentEditFeeFixture(t)
	ctx := context.Background()
	var addedProduct int64
	if err := f.pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.products(name) VALUES('新商品物流规格') RETURNING id`, f.schema)).Scan(&addedProduct); err != nil {
		t.Fatal(err)
	}
	seedManualFreightWeight(t, f.pool, f.schema, addedProduct, 400)
	// An on-demand order uses the same deferred engine. Preserve its historical
	// collection fields and original free-shipping/surcharge policy on edit.
	f.quote.FulfillmentMode = "on_demand"
	policy := f.quote.Freight.Policy
	policy.Scene, policy.FreeShipping, policy.SurchargeCents = "on_demand", true, 125
	var templateRaw []byte
	if err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json FROM %s.shipping_fee_template_versions WHERE template_id=$1 AND revision=$2`, f.schema), f.quote.ShippingTemplateID, f.quote.ShippingTemplateRevision).Scan(&templateRaw); err != nil {
		t.Fatal(err)
	}
	var template domain.Template
	if err := json.Unmarshal(templateRaw, &template); err != nil {
		t.Fatal(err)
	}
	freight, err := domain.Quote(policy, template, f.quote.Freight.Destination, f.quote.NetWeightG)
	if err != nil {
		t.Fatal(err)
	}
	freight = freightpg.AttachWeightBasis(freight, f.quote.Freight.ItemWeights)
	f.quote.Freight, f.quote.CalculatedShippingFeeCents = &freight, 0
	f.quote.FeeQuoteToken = app.FulfillmentFeeQuoteToken(f.quote)
	quoteRaw, _ := json.Marshal(f.quote)
	freightRaw, _ := json.Marshal(freight)
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.fulfillment_order_fee_snapshots SET mode='on_demand',quote_json=$2 WHERE order_id=$1`, f.schema), f.order, quoteRaw); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_freight_snapshots SET snapshot_json=$2 WHERE order_id=$1`, f.schema), f.order, freightRaw); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET fulfillment_mode='on_demand',total_amount=80,grand_total=80,payment_goods_amount=80,prepayment_amount=80,payment_voucher_asset_id=18 WHERE id=$1`, f.schema), f.order); err != nil {
		t.Fatal(err)
	}
	f.transaction(t, func(tx pgx.Tx) {
		c := f.command(8)
		c.FulfillmentMode = "on_demand"
		c.Items = append(c.Items, app.FulfillmentFeeItem{ProductID: addedProduct, Qty: 1})
		q, err := f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, c)
		if err != nil {
			t.Fatal(err)
		}
		if q.NetWeightG != 2560 || q.ChargeableWeightG != 3060 || q.ServiceFeeCents != 325 || q.Freight.Policy != policy || q.Freight.Status != "free" || q.Freight.SurchargeCents != 125 || q.Freight.WaiverCents != 1335 || q.CalculatedShippingFeeCents != 0 {
			t.Fatalf("new item/free policy mismatch: %+v", q)
		}
		if err := f.r.saveFulfillmentOrderEditFeesTx(ctx, tx, f.customer, f.order, f.quote.FeeQuoteToken, f.quote.Freight.Token, q, "fee-editor"); err != nil {
			t.Fatal(err)
		}
		var goods, prepay float64
		var voucher int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT payment_goods_amount::float8,prepayment_amount::float8,payment_voucher_asset_id FROM %s.orders WHERE id=$1`, f.schema), f.order).Scan(&goods, &prepay, &voucher); err != nil {
			t.Fatal(err)
		}
		if goods != 80 || prepay != 80 || voucher != 18 {
			t.Fatal("editing fees rewrote historical collections")
		}
		var rows int
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.customer_fee_items WHERE source_type='order' AND source_id=$1 AND fee_type='direct_ship_service'`, f.schema), f.order).Scan(&rows); err != nil || rows != 1 {
			t.Fatalf("service fee duplicated: %d %v", rows, err)
		}
	})
}

func TestFulfillmentOrderEditFeesConcurrentActualBillWins(t *testing.T) {
	f := newFulfillmentEditFeeFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	fr := freightpg.NewRepository(f.pool, f.schema)
	var priorPreview app.FulfillmentFeeQuote
	f.transaction(t, func(tx pgx.Tx) {
		var err error
		priorPreview, err = f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, f.command(12))
		if err != nil {
			t.Fatal(err)
		}
	})
	tracking := "EDIT-CONCURRENT-BILL"
	if _, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET logistics_company_id=1,ship_tracking_no=$2 WHERE id=$1`, f.schema), f.order, tracking); err != nil {
		t.Fatal(err)
	}
	batch, err := fr.PreviewImport(ctx, 1, "fulfillment-edit-bill-first", "test.csv", []freightpg.ImportRow{{RowNo: 1, TrackingNo: tracking, ActualCents: 1234}}, "bill-test")
	if err != nil || len(batch.Rows) != 1 || batch.Rows[0].Error != "" {
		t.Fatalf("prepare actual bill: %+v %v", batch, err)
	}
	// Block the real bill writer on its order row, after it has taken the shared
	// freight lock. The editor must wait behind that writer, then see its bill.
	barrier, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer barrier.Rollback(ctx)
	var barrierPID int
	if err = barrier.QueryRow(ctx, fmt.Sprintf(`SELECT pg_backend_pid() FROM %s.orders WHERE id=$1 FOR UPDATE`, f.schema), f.order).Scan(&barrierPID); err != nil {
		t.Fatal(err)
	}
	billDone := make(chan error, 1)
	go func() { _, err := fr.ConfirmImport(ctx, batch.ID, "bill-test"); billDone <- err }()
	waitFulfillmentEditLock(t, ctx, f.pool, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, barrierPID)

	editPID := make(chan int, 1)
	editDone := make(chan error, 1)
	go func() {
		tx, err := f.pool.Begin(ctx)
		if err != nil {
			editDone <- err
			return
		}
		defer tx.Rollback(ctx)
		var pid int
		if err = tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			editDone <- err
			return
		}
		editPID <- pid
		if err = freightpg.LockOrderFreight(ctx, tx); err == nil {
			_, err = tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=$1 FOR UPDATE`, f.schema), f.order)
		}
		if err == nil {
			var q app.FulfillmentFeeQuote
			q, err = f.r.previewFulfillmentOrderEditFeesTx(ctx, tx, f.command(12))
			if err == nil {
				err = f.r.saveFulfillmentOrderEditFeesTx(ctx, tx, f.customer, f.order, f.quote.FeeQuoteToken, f.quote.Freight.Token, q, "fee-editor")
			}
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		editDone <- err
	}()
	var pid int
	select {
	case pid = <-editPID:
	case err = <-editDone:
		t.Fatalf("editor did not reach freight lock: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	waitFulfillmentEditLock(t, ctx, f.pool, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND NOT granted)`, pid)
	if err = barrier.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-billDone; err != nil {
		t.Fatalf("real actual bill confirmation failed: %v", err)
	}
	if err = <-editDone; err == nil || !strings.Contains(err.Error(), "运费已出账") {
		t.Fatalf("editor crossed an actual bill: %v", err)
	}
	// The final save guard independently rejects a quote obtained before posting,
	// even if a future caller accidentally carries that quote past re-preview.
	f.transaction(t, func(tx pgx.Tx) {
		err := f.r.saveFulfillmentOrderEditFeesTx(ctx, tx, f.customer, f.order, f.quote.FeeQuoteToken, f.quote.Freight.Token, priorPreview, "fee-editor")
		if err == nil || !strings.Contains(err.Error(), "运费已出账") {
			t.Fatalf("stale preview overwrote an actual bill during save: %v", err)
		}
	})
	var saved app.FulfillmentFeeQuote
	var raw []byte
	var actual, billed, revision, actualRows, serviceRows int64
	var service float64
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT quote_json FROM %s.fulfillment_order_fee_snapshots WHERE order_id=$1`, f.schema), f.order).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT actual_cents,billed_cents,revision FROM %s.freight_actual_shipments WHERE order_id=$1`, f.schema), f.order).Scan(&actual, &billed, &revision); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FILTER(WHERE source_type='freight_actual'),count(*) FILTER(WHERE source_type='order'),sum(amount) FILTER(WHERE source_type='order')::float8 FROM %s.customer_fee_items WHERE customer_id=$1`, f.schema), f.customer).Scan(&actualRows, &serviceRows, &service); err != nil {
		t.Fatal(err)
	}
	if saved.FeeQuoteToken != f.quote.FeeQuoteToken || actual != 1234 || billed != 1234 || revision != 1 || actualRows != 1 || serviceRows != 1 || service != 3 {
		t.Fatalf("concurrent edit altered billed/history state: fee=%+v actual=%d billed=%d revision=%d actualRows=%d serviceRows=%d service=%v", saved, actual, billed, revision, actualRows, serviceRows, service)
	}
}

func waitFulfillmentEditLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, pid int) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := pool.QueryRow(ctx, query, pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("database operation did not reach expected lock: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
