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
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,grand_total,discount_amount) VALUES('SO-BILL-SNAPSHOT',$1,'2026-10-22',80,20) RETURNING id`, schema), customerID).Scan(&orderID); err != nil {
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
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET item_name='修改后商品' WHERE order_id=$1`, schema), orderID); err != nil {
		t.Fatal(err)
	}
	again, err := repo.GetIssuedBill(ctx, customerID, bill.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Lines[0].Detail.Items[0].Name != "原商品" {
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
	if historic.Lines[0].Detail.InformationSource != "原单当前信息" || historic.Lines[0].Detail.Items[0].Name != "修改后商品" || historic.TotalAmountCents != bill.TotalAmountCents {
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
}
