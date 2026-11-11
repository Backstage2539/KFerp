package customerfulfillment

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	salespg "orderapp/internal/infrastructure/postgres/sales"
)

func TestOperationsOrderBalanceIncludesResolutionAndReceipts(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	_, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.orders(order_no,customer_id,portal_service_code,total_amount,grand_total,pay_status_id,prepayment_amount,resolution_credit_cents,resolution_offset_cents,resolution_settled_out_cents,resolution_payment_tracked) VALUES
 ('BAL-CLOSED',501,'direct_ship',2160,2160,(SELECT id FROM %[1]s.pay_statuses WHERE name='未付款' LIMIT 1),0,216000,0,0,false),
 ('BAL-CREDIT',501,'direct_ship',100,100,(SELECT id FROM %[1]s.pay_statuses WHERE name='未付款' LIMIT 1),0,2500,0,0,false),
 ('BAL-PAID-PART',501,'direct_ship',100,100,(SELECT id FROM %[1]s.pay_statuses WHERE name='未付款' LIMIT 1),40,0,0,0,false),
 ('BAL-OFFSET',501,'direct_ship',100,100,(SELECT id FROM %[1]s.pay_statuses WHERE name='未付款' LIMIT 1),0,0,3000,0,false),
 ('BAL-TRANSFER',501,'direct_ship',100,100,(SELECT id FROM %[1]s.pay_statuses WHERE name='已付款' LIMIT 1),100,0,0,2500,true),
 ('BAL-LEGACY-PAID',501,'direct_ship',100,100,(SELECT id FROM %[1]s.pay_statuses WHERE name='已付款' LIMIT 1),0,0,0,0,false)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	overview, err := NewRepository(pool, schema).Overview(ctx, app.OverviewQuery{CustomerID: 501})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Operations.PendingPayment != 4 || overview.Operations.PendingOrderCents != 23000 {
		t.Errorf("overview pending count=%d cents=%d; want 4 and 23000", overview.Operations.PendingPayment, overview.Operations.PendingOrderCents)
	}
	list, err := salespg.NewRepository(pool, schema).ListOrders(ctx, salesapp.OrderListQuery{Scope: "fulfillment", CustomerID: 501, FulfillmentQueue: "pending_payment", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"BAL-CREDIT": true, "BAL-PAID-PART": true, "BAL-OFFSET": true, "BAL-TRANSFER": true}
	for _, row := range list.Rows {
		if !wanted[row.OrderNo] {
			t.Errorf("unexpected pending order %s", row.OrderNo)
		}
		delete(wanted, row.OrderNo)
	}
	if list.Summary.Orders != 4 || len(wanted) > 0 {
		t.Errorf("pending list count=%d missing=%v", list.Summary.Orders, wanted)
	}
}
