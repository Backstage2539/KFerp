package customerfulfillment

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	"testing"
)

func TestOperationsWorkbenchHistoricalReadsAndUnpaginatedTodos(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(order_no,customer_id,portal_service_code,total_amount,grand_total,receiver_name,receiver_phone,receiver_address) SELECT 'PR718-'||n,501,'direct_ship',10,10,'收件人','13800000000','云南省' FROM generate_series(1,131) n;
 INSERT INTO %[1]s.orders(order_no,customer_id,portal_service_code,total_amount,grand_total) VALUES('PR718-OTHER',502,'direct_ship',90,90);
 UPDATE %[1]s.orders SET receiver_phone='' WHERE order_no='PR718-1';
 UPDATE %[1]s.orders SET is_void=true WHERE order_no='PR718-2';
 UPDATE %[1]s.customer_service_capabilities SET enabled=false WHERE customer_id=501;
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.customer_settlement_batches(id,customer_id,status,total_amount) VALUES(718,501,'draft',30); INSERT INTO %[1]s.customer_fee_items(customer_id,fee_type,amount,settlement_batch_id,status) VALUES(501,'direct_ship_service',30,718,'unsettled'),(501,'storage',20,0,'unsettled'),(502,'storage',90,0,'unsettled');`, schema))
	if err != nil {
		t.Fatal(err)
	}
	r := NewRepository(pool, schema)
	overview, err := r.Overview(ctx, app.OverviewQuery{CustomerID: 501})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Operations.UnsettledFeeCount != 1 || overview.Operations.UnsettledFeeCents != 2000 || overview.Operations.DraftSettlementCents != 3000 {
		t.Fatalf("fee counted twice: %+v", overview.Operations)
	}
	if overview.Operations.PendingShipment != 130 || overview.Operations.MissingRecipient != 1 || overview.Operations.PendingPayment != 130 {
		t.Fatalf("counts=%+v", overview.Operations)
	}
	if overview.Operations.PendingOrderCents != 130000 {
		t.Fatalf("order money=%+v", overview.Operations)
	}
	for queue, want := range map[string]int{"pending_shipment": 130, "missing_recipient": 1, "pending_payment": 130} {
		list, err := salespg.NewRepository(pool, schema).ListOrders(ctx, salesapp.OrderListQuery{Scope: "fulfillment", CustomerID: 501, FulfillmentQueue: queue, Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		if list.Summary.Orders != want || len(list.Rows) > 20 {
			t.Fatalf("queue %s: %+v", queue, list.Summary)
		}
		for _, row := range list.Rows {
			if row.OrderNo == "PR718-OTHER" {
				t.Fatal("foreign customer leaked")
			}
		}
	}

	// Retained sources keep financial/confirmation work but never enter shipping queues.
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,portal_service_code,total_amount,grand_total,delivery_mode,delivery_status,confirmation_status) VALUES('PR718-RETAINED',501,'product_order',120,120,'customer_stock','pending_receipt','pending')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	for queue, want := range map[string]int{"pending_shipment": 130, "missing_recipient": 1, "dispatch": 130, "pending_payment": 131, "pending_confirmation": 1, "stock_waiting": 0} {
		list, err := salespg.NewRepository(pool, schema).ListOrders(ctx, salesapp.OrderListQuery{Scope: "fulfillment", CustomerID: 501, FulfillmentQueue: queue, Limit: 200})
		if err != nil {
			t.Fatal(err)
		}
		// The list excludes void orders in all queues.
		if list.Summary.Orders != want {
			t.Fatalf("retained changed queue %s: got %d want %d", queue, list.Summary.Orders, want)
		}
	}
	overview, err = r.Overview(ctx, app.OverviewQuery{CustomerID: 501})
	if err != nil {
		t.Fatal(err)
	}
	if overview.Operations.PendingShipment != 130 || overview.Operations.MissingRecipient != 1 || overview.Operations.PendingPayment != 131 || overview.Operations.PendingConfirmation != 1 {
		t.Fatalf("retained counters=%+v", overview.Operations)
	}
}
