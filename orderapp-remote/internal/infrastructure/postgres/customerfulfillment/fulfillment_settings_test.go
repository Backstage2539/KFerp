package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerfulfillment"
	"testing"
)

func TestFulfillmentSettingsPersistValidateWarehouseQuoteAndBillOnce(t *testing.T) {
	ctx := context.Background()
	pool, schema := newMiniDirectShipTestDB(t)
	r := NewRepository(pool, schema)
	var customerID, productID, orderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('喜沁费率测试') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.warehouses(code,name,kind,customer_id) VALUES('fees-own','客户成品仓','customer_finished',$1),('fees-other','其他仓','customer_finished',999999)`, schema), customerID); err != nil {
		t.Fatal(err)
	}
	tm, err := r.SaveShippingFeeTemplate(ctx, app.SaveShippingFeeTemplateCommand{Actor: "tester", Template: app.ShippingFeeTemplate{Name: "云南模板", Active: true, PackagingWeightG: 200, Rules: []app.ShippingFeeRegionRule{{Province: "云南省", FirstWeightG: 1000, FirstFeeCents: 800, AdditionalWeightG: 1000, AdditionalFeeCents: 200}}}})
	if err != nil {
		t.Fatal(err)
	}
	settings := app.FulfillmentSettings{CustomerID: customerID, Modes: []app.FulfillmentModeSettings{{Mode: "stock", Enabled: true, WarehouseCode: "fees-other", FeeConfigured: true, FixedFeeCents: 300, PerPackageFeeCents: 50, ShippingTemplateID: tm.ID}}}
	if _, err = r.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Actor: "tester", Settings: settings}); err == nil {
		t.Fatal("foreign warehouse accepted")
	}
	settings.Modes[0].WarehouseCode = "fees-own"
	saved, err := r.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Actor: "tester", Settings: settings})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 1 {
		t.Fatalf("revision=%d", saved.Revision)
	}
	if _, err = r.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Actor: "tester", Settings: settings}); err == nil {
		t.Fatal("stale revision accepted")
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.products(name,net_content_qty,net_content_unit) VALUES('曜石454g',454,'g') RETURNING id`, schema)).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	seedManualFreightWeight(t, pool, schema, productID, 454)
	quote, err := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: customerID, FulfillmentMode: "stock", Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: productID, SpecG: 1, Qty: 12}}})
	if err != nil {
		t.Fatal(err)
	}
	if quote.NetWeightG != 5448 || quote.ServiceFeeCents != 900 || quote.ShippingFeeCents != 0 || quote.Freight == nil || quote.Freight.EstimatedTotalCents != 1800 {
		t.Fatalf("quote=%+v", quote)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,total_amount,grand_total) VALUES('FEE-ONCE',$1,0,0) RETURNING id`, schema), customerID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		e = r.SaveFulfillmentOrderFeesTx(ctx, tx, customerID, orderID, 0, "stock", quote, "tester")
		if e != nil {
			tx.Rollback(ctx)
			t.Fatal(e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
	}
	var count int
	var fee, ship, total float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),sum(amount)::float8 FROM %s.customer_fee_items WHERE customer_id=$1 AND source_type='order' AND source_id=$2 AND fee_type='direct_ship_service'`, schema), customerID, orderID).Scan(&count, &fee); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT shipping_amount::float8,grand_total::float8 FROM %s.orders WHERE id=$1`, schema), orderID).Scan(&ship, &total); err != nil {
		t.Fatal(err)
	}
	if count != 1 || fee != 9 || ship != 0 || total != 0 {
		t.Fatalf("count=%d fee=%v ship=%v total=%v", count, fee, ship, total)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET shipping_amount=99 WHERE id=$1`, schema), orderID); err == nil {
		t.Fatal("normal or inline edit bypassed the reason-required fee adjustment")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET notes='正常备注编辑',shipping_amount=0 WHERE id=$1`, schema), orderID); err != nil {
		t.Fatalf("unchanged freight blocked: %v", err)
	}
	var legacyOrderID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id) VALUES('LEGACY-FREIGHT',$1) RETURNING id`, schema), customerID).Scan(&legacyOrderID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET shipping_amount=99 WHERE id=$1`, schema), legacyOrderID); err != nil {
		t.Fatalf("legacy freight blocked: %v", err)
	}
	if _, err = r.AdjustFulfillmentOrderFees(ctx, app.AdjustFulfillmentOrderFeesCommand{CustomerID: customerID, OrderID: orderID, ExpectedQuoteToken: quote.FeeQuoteToken, ServiceFeeCents: 1000, ShippingFeeCents: 2000, Actor: "tester"}); err == nil {
		t.Fatal("no reason accepted")
	}
	adjusted, err := r.AdjustFulfillmentOrderFees(ctx, app.AdjustFulfillmentOrderFeesCommand{CustomerID: customerID, OrderID: orderID, ExpectedQuoteToken: quote.FeeQuoteToken, ServiceFeeCents: 1000, ShippingFeeCents: 0, Actor: "tester", Reason: "服务费核对，运费另行出账"})
	if err != nil {
		t.Fatal(err)
	}
	if adjusted.ServiceFeeCents != 1000 || adjusted.ShippingFeeCents != 0 || adjusted.AdjustmentReason == "" {
		t.Fatalf("adjusted=%+v", adjusted)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fee_items SET settlement_batch_id=777 WHERE customer_id=$1 AND source_id=$2 AND source_type='order'`, schema), customerID, orderID); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = r.ReverseFulfillmentOrderFeesTx(ctx, tx, customerID, orderID, "tester")
	tx.Rollback(ctx)
	if err == nil {
		t.Fatal("settled fee cancelled without reversal")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fee_items SET settlement_batch_id=0 WHERE customer_id=$1 AND source_id=$2 AND source_type='order'`, schema), customerID, orderID); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.ReverseFulfillmentOrderFeesTx(ctx, tx, customerID, orderID, "tester"); err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var reversedAmount float64
	var reversedStatus string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT amount::float8,status FROM %s.customer_fee_items WHERE customer_id=$1 AND source_id=$2 AND source_type='order'`, schema), customerID, orderID).Scan(&reversedAmount, &reversedStatus); err != nil || reversedAmount != 0 || reversedStatus != "reversed" {
		t.Fatalf("reversed=%v %s err=%v", reversedAmount, reversedStatus, err)
	}
	if _, err = r.AdjustFulfillmentOrderFees(ctx, app.AdjustFulfillmentOrderFeesCommand{CustomerID: customerID, OrderID: orderID, ExpectedQuoteToken: adjusted.FeeQuoteToken, ServiceFeeCents: 1000, ShippingFeeCents: 2000, Actor: "tester", Reason: "过期的编辑"}); err == nil {
		t.Fatal("stale quote edited")
	}
	var logs int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type IN ('customer_fulfillment_settings','shipping_fee_template','fulfillment_order_fees')`, schema)).Scan(&logs); err != nil || logs < 4 {
		t.Fatalf("logs=%d err=%v", logs, err)
	}
}

func TestFulfillmentStockWeightUsesManualLogisticsInsteadOfConversionOrPriceTable(t *testing.T) {
	ctx := context.Background()
	pool, schema := newMiniDirectShipTestDB(t)
	r := NewRepository(pool, schema)
	var productID, bomID, versionID, specID, variantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.products(name,net_content_qty,net_content_unit) VALUES('双规格商品',9999,'g') RETURNING id`, schema)).Scan(&productID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_boms(code,name,output_type,output_product_id,status) VALUES('FEE-WEIGHT','规格净重','product',$1,'active') RETURNING id`, schema), productID).Scan(&bomID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit) VALUES($1,'v1','published',1,'袋') RETURNING id`, schema), bomID).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_specs(bom_id,code,spec_key,name,inventory_unit) VALUES($1,'FEE-BAG','bag','不可猜重量的名称','袋') RETURNING id`, schema), bomID).Scan(&specID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g) VALUES($1,$2,'不可猜重量的名称','袋',true,250) RETURNING id`, schema), versionID, specID).Scan(&variantID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default) VALUES('product',$1,$2,$3,true)`, schema), productID, bomID, versionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.spec_conversion_groups(name,base_unit,members_json) VALUES('净重关系','kg',$1)`, schema), fmt.Sprintf(`[{"bom_spec_id":%d,"inventory_unit":"袋","base_quantity":"0.454"}]`, specID)); err != nil {
		t.Fatal(err)
	}
	weight, err := r.fulfillmentItemNetWeightG(ctx, pool, 1, app.FulfillmentFeeItem{ProductID: productID, BomSpecID: specID, BomVariantID: variantID, Qty: 12})
	if err != nil || weight != 250 {
		t.Fatalf("weight=%d err=%v", weight, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=NULL; DELETE FROM %[1]s.spec_conversion_groups`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.fulfillmentItemNetWeightG(ctx, pool, 1, app.FulfillmentFeeItem{ProductID: productID, BomSpecID: specID, BomVariantID: variantID, Qty: 12}); err == nil {
		t.Fatal("used parent weight or parsed spec display name")
	}
}

// Legacy freight fixtures now explicitly configure manual published spec weight.
func seedManualFreightWeight(t *testing.T, pool *pgxpool.Pool, schema string, productID int64, grams float64) {
	t.Helper()
	ctx := context.Background()
	var bom, version, spec int64
	if e := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_boms(code,name,output_type,output_product_id,specification_mode,status)VALUES($1,'手填物流重量','product',$2,'spec_group','active')RETURNING id`, schema), fmt.Sprintf("MANUAL-WEIGHT-%d", productID), productID).Scan(&bom); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_versions(bom_id,version_no,status,output_qty,output_unit)VALUES($1,'v1','published',1,'袋')RETURNING id`, schema), bom).Scan(&version); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_specs(bom_id,code,spec_key,name,inventory_unit)VALUES($1,$2,'manual','手填物流规格','袋')RETURNING id`, schema), bom, fmt.Sprintf("MANUAL-SPEC-%d", productID)).Scan(&spec); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g)VALUES($1,$2,'手填物流规格','袋',true,$3)`, schema), version, spec, grams); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)VALUES('product',$1,$2,$3,true)`, schema), productID, bom, version); e != nil {
		t.Fatal(e)
	}
}
