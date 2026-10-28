package customerfulfillment

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

func TestAccountAllUnpaidMatchesBillingAcrossIssuedAndUnbilled(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID, otherID, partialID, paidID, partialOrder, paidOrder int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('未付客户') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('其他客户') RETURNING id`, schema)).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled) VALUES($1,'settlement',true)`, schema), customerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS prepayment_amount NUMERIC NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS payment_goods_amount NUMERIC NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS payment_shipping_amount NUMERIC NOT NULL DEFAULT 0`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name='预付款（付款未完成）' LIMIT 1`, schema)).Scan(&partialID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name='已付款' LIMIT 1`, schema)).Scan(&paidID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,pay_status_id,grand_total,shipping_amount,prepayment_amount,payment_goods_amount,is_void) VALUES('PARTIAL-OLD',$1,'2026-08-10',$2,100,0,40,40,false) RETURNING id`, schema), customerID, partialID).Scan(&partialOrder); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,pay_status_id,grand_total,shipping_amount,prepayment_amount,payment_goods_amount,is_void) VALUES('PAID-ORDER',$1,'2026-10-01',$2,50,0,50,50,false) RETURNING id`, schema), customerID, paidID).Scan(&paidOrder); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status,note) VALUES($1,'order',$2,'direct_ship_service',3,'2026-10-22','unsettled','服务费'),($1,'freight_actual',999,'shipping',9.10,'2026-10-26','unsettled','实际运费'),($3,'order',999,'direct_ship_service',999,'2026-10-26','unsettled','其他客户费用')`, schema), customerID, paidOrder, otherID); err != nil {
		t.Fatal(err)
	}
	var paidFeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status,note) VALUES($1,'order',$2,'processing',5,'2026-10-20','unsettled','已付加工费') RETURNING id`, schema), customerID, paidOrder).Scan(&paidFeeID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status,note) VALUES($1,'adjustment',987,'adjustment',-2,'2026-10-24','unsettled','退款调整')`, schema), customerID); err != nil {
		t.Fatal(err)
	}
	paidCmd := app.BillingPreviewCommand{CustomerID: customerID, Sources: []app.BillingSource{{ChargeType: "processing", SourceType: "customer_fee_item", SourceID: paidFeeID}}}
	paidPreview, err := repo.PreviewBilling(ctx, paidCmd)
	if err != nil {
		t.Fatal(err)
	}
	paidBill, err := repo.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: paidCmd, PreviewVersion: paidPreview.PreviewVersion, IdempotencyKey: "paid-fee-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_batches SET status='paid',paid_at=now() WHERE id=$1`, schema), paidBill.BatchID); err != nil {
		t.Fatal(err)
	}
	cmd := app.BillingPreviewCommand{CustomerID: customerID, Sources: []app.BillingSource{{ChargeType: "order_goods", SourceType: "order", SourceID: partialOrder}}}
	preview, err := repo.PreviewBilling(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: cmd, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "unpaid-fixture"}); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET prepayment_amount=60,payment_goods_amount=60 WHERE id=$1`, schema), partialOrder); err != nil {
		t.Fatal(err)
	}
	account, err := repo.CustomerAccount(ctx, app.AccountQuery{CustomerID: customerID, View: "order_fees", UnpaidOnly: true, Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	totals, err := repo.BillingSummary(ctx, customerID)
	if err != nil {
		t.Fatal(err)
	}
	if totals.TotalUnpaidCents != 5210 || account.FeeSummary.DueCents != totals.TotalUnpaidCents || account.FeeSummary.GoodsCents != 4000 || account.FeeSummary.DirectShipServiceCents != 300 || account.FeeSummary.ShippingCents != 910 {
		t.Fatalf("account=%+v billing=%+v", account.FeeSummary, totals)
	}
	for _, group := range account.OrderFeeGroups {
		if group.Order.OrderNo == "PAID-ORDER" && (len(group.FeeLines) != 1 || group.FeeLines[0].Category != "direct_ship_service") {
			t.Fatalf("paid order independent fee=%+v", group)
		}
	}
}
