package customerfulfillment

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

func TestIssuedBillDetailSnapshotAndHistoricFallback(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID, otherCustomer, orderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('测试客户') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('其他客户') RETURNING id`, schema)).Scan(&otherCustomer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled) VALUES($1,'settlement',true)`, schema), customerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS other_fee numeric(12,2) DEFAULT 0, ADD COLUMN IF NOT EXISTS other_fee_note text DEFAULT ''`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,grand_total,discount_amount,other_fee,other_fee_note) VALUES('SO-BILL-SNAPSHOT',$1,'2026-10-22',87.50,20,7.50,'贴标') RETURNING id`, schema), customerID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(order_id,line_no,item_name,spec,qty,unit,unit_price,line_total) VALUES($1,1,'原商品','454g',2,'袋',50,100)`, schema), orderID); err != nil {
		t.Fatal(err)
	}
	command := app.BillingPreviewCommand{CustomerID: customerID, Sources: []app.BillingSource{{ChargeType: "order_goods", SourceType: "order", SourceID: orderID}}}
	preview, err := repo.PreviewBilling(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := repo.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: command, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "snapshot-once"})
	if err != nil {
		t.Fatal(err)
	}
	if len(bill.Lines) != 1 || bill.Lines[0].Detail == nil || len(bill.Lines[0].Detail.Items) != 1 || bill.Lines[0].Detail.Items[0].Name != "原商品" || bill.Lines[0].Detail.InformationSource != "出账快照" {
		t.Fatalf("snapshot missing: %+v", bill.Lines)
	}
	if bill.TotalAmountCents != 8750 || bill.Lines[0].Detail.OtherFeeCents != 750 || bill.Lines[0].Detail.OtherFeeNote != "贴标" {
		t.Fatalf("issued fee detail: %+v", bill)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET other_fee_note='新说明' WHERE id=$1`, schema), orderID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET item_name='修改后商品' WHERE order_id=$1`, schema), orderID); err != nil {
		t.Fatal(err)
	}
	again, err := repo.GetIssuedBill(ctx, customerID, bill.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Lines[0].Detail.Items[0].Name != "原商品" || again.Lines[0].Detail.OtherFeeNote != "贴标" {
		t.Fatalf("snapshot changed: %+v", again.Lines[0].Detail)
	}
	if _, err = repo.GetIssuedBill(ctx, otherCustomer, bill.BatchID); err == nil {
		t.Fatal("other customer read bill")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_lines SET detail_snapshot=NULL WHERE settlement_batch_id=$1`, schema), bill.BatchID); err != nil {
		t.Fatal(err)
	}
	historic, err := repo.GetIssuedBill(ctx, customerID, bill.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if historic.Lines[0].Detail.OtherFeeCents != 0 || historic.Lines[0].Detail.OtherFeeNote != "" || historic.Lines[0].Detail.InformationSource != "原单当前信息" || historic.Lines[0].Detail.Items[0].Name != "修改后商品" || historic.TotalAmountCents != bill.TotalAmountCents {
		t.Fatalf("historic fallback: %+v", historic)
	}
}

func TestBillingDetailUsesOnlyFreightSourceWaybill(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID, orderID, feeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('多运单客户') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,grand_total) VALUES('SO-TWO-WAYBILLS',$1,'2026-10-22',0) RETURNING id`, schema), customerID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.freight_actual_shipments(id bigint PRIMARY KEY,order_id bigint,customer_id bigint,tracking_no text)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.freight_actual_entries(id bigint PRIMARY KEY,shipment_id bigint)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_shipment_orders(id bigint PRIMARY KEY,order_id bigint,tracking_no text,shipped_at timestamptz)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.shipping_return_rows(id bigint PRIMARY KEY,status text,selected_order_id bigint,payload jsonb)`, schema),
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_shipments(id,carrier_id,order_id,customer_id,tracking_no) VALUES(11,1,$1,$2,'SF-A')`, schema), orderID, customerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_entries(id,shipment_id,request_key,actual_cents,target_cents,delta_cents,previous_actual_cents,previous_billed_cents,reason,created_by) VALUES(21,11,'test',1110,1110,1110,0,0,'test','test')`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_shipment_orders(id,order_id,tracking_no,shipped_at) VALUES(1,$1,'SF-A','2026-10-26'),(2,$1,'SF-B','2026-10-27')`, schema), orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_return_rows(id,status,selected_order_id,payload) VALUES(1,'applied',$1,'{"tracking_no":"SF-A","pickup_time":"2026-10-26 10:00","settlement_weight_kg":"1.2"}')`, schema), orderID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status) VALUES($1,'freight_actual',21,'shipping',11.10,'2026-10-26','unsettled') RETURNING id`, schema), customerID).Scan(&feeID); err != nil {
		t.Fatal(err)
	}
	line := app.SettlementLineSummary{ChargeType: "shipping", SourceType: "customer_fee_item", SourceID: feeID, AmountCents: 1110}
	detail, err := repo.billingLineDetail(ctx, pool, customerID, line, "原单当前信息")
	if err != nil {
		t.Fatal(err)
	}
	if detail.OrderNo != "SO-TWO-WAYBILLS" || len(detail.Shipments) != 1 || detail.Shipments[0].TrackingNo != "SF-A" || detail.Shipments[0].SettlementWeightKG != "1.2" || detail.Shipments[0].ShippedAt != "2026-10-26 10:00" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	// A single fee adjustment covering a multi-parcel return must show exactly its source parcels.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.freight_actual_entries ADD COLUMN IF NOT EXISTS shipping_return_group_id bigint;
UPDATE %[1]s.freight_actual_entries SET shipping_return_group_id=12 WHERE id=21`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_shipments(id,carrier_id,order_id,customer_id,tracking_no) VALUES(12,1,$1,$2,'SF-B')`, schema), orderID, customerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.freight_actual_entries(id,shipment_id,request_key,actual_cents,target_cents,delta_cents,previous_actual_cents,previous_billed_cents,reason,created_by,shipping_return_group_id) VALUES(22,12,'group-p2',550,0,0,0,0,'test','test',12);
UPDATE %[1]s.order_shipment_orders SET tracking_no=E'SF-A\nSF-B' WHERE id=1;
UPDATE %[1]s.order_shipment_orders SET tracking_no='SF-OTHER' WHERE id=2;`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.shipping_return_rows(id,status,selected_order_id,payload) VALUES(2,'applied',$1,'{"tracking_no":"SF-B","pickup_time":"2026-10-26 10:01","settlement_weight_kg":"2.3"}')`, schema), orderID); err != nil {
		t.Fatal(err)
	}
	detail, err = repo.billingLineDetail(ctx, pool, customerID, line, "出账快照")
	if err != nil || len(detail.Shipments) != 2 || detail.Shipments[0].TrackingNo != "SF-A" || detail.Shipments[1].TrackingNo != "SF-B" || detail.Shipments[1].SettlementWeightKG != "2.3" {
		t.Fatalf("group fee detail %+v: %v", detail, err)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled) VALUES($1,'settlement',true)`, schema), customerID); err != nil {
		t.Fatal(err)
	}
	query := app.BillingQuery{CustomerID: customerID, FeeType: "shipping"}
	unbilled, err := repo.ListUnbilledCharges(ctx, query)
	if err != nil || len(unbilled.Rows) != 1 || unbilled.Rows[0].SourceNo != "SF-A\nSF-B" {
		t.Fatalf("unbilled group %+v %v", unbilled, err)
	}
	command := app.BillingPreviewCommand{CustomerID: customerID, Sources: []app.BillingSource{{ChargeType: "shipping", SourceType: "customer_fee_item", SourceID: feeID}}}
	preview, err := repo.PreviewBilling(ctx, command)
	if err != nil || len(preview.Lines) != 1 || preview.Lines[0].SourceNo != "SF-A\nSF-B" {
		t.Fatalf("group preview %+v %v", preview, err)
	}
	bill, err := repo.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: command, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "two-tracking-snapshot"})
	if err != nil || bill.TotalAmountCents != 1110 || len(bill.Lines) != 1 || bill.Lines[0].SourceNo != "SF-A\nSF-B" || len(bill.Lines[0].Detail.Shipments) != 2 {
		t.Fatalf("group snapshot %+v %v", bill, err)
	}

}

func TestBillingDefaultSealMatchesPreviewAndRespectsSavedEmptyLayout(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID, billID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('默认公章测试') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_batches(customer_id,settlement_no,status,total_amount,created_by) VALUES($1,'BILL-DEFAULT-SEAL','confirmed',10,'test') RETURNING id`, schema), customerID).Scan(&billID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.sales_order_settings(id bigint primary key,seal_asset_id bigint); INSERT INTO %s.sales_order_settings VALUES(1,9)`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	initial, err := repo.GetBillingPDFSettings(ctx, customerID, billID)
	if err != nil {
		t.Fatal(err)
	}
	if initial.SealAssetID != 9 || len(initial.Placements) != 1 || initial.Placements[0].PageNumber != 1 {
		t.Fatalf("default=%+v", initial)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_bill_pdf_settings(customer_id,settlement_id,seal_asset_id,placements) VALUES($1,$2,9,'[]'::jsonb)`, schema), customerID, billID); err != nil {
		t.Fatal(err)
	}
	saved, err := repo.GetBillingPDFSettings(ctx, customerID, billID)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Placements) != 0 {
		t.Fatalf("saved empty changed: %+v", saved)
	}
}
