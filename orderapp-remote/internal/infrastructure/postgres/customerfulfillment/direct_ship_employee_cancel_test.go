package customerfulfillment

import (
	"context"
	"fmt"
	"testing"
)

func TestEmployeeCancelOrderRequestLookupRejectsCrossCustomer(t *testing.T) {
	ctx := context.Background()
	pool, schema := newMiniDirectShipTestDB(t)
	r := NewRepository(pool, schema)
	var customerID, otherID, orderID, requestID int64
	for _, row := range []struct {
		name string
		id   *int64
	}{{"取消客户", &customerID}, {"其他客户", &otherID}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES($1) RETURNING id`, schema), row.name).Scan(row.id); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id) VALUES('CANCEL-SCOPE',$1) RETURNING id`, schema), customerID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_requests(customer_id,request_no,status) VALUES($1,'DSR-CANCEL-SCOPE','reserved') RETURNING id`, schema), customerID).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_orders(request_id,order_id,order_no,status) VALUES($1,$2,'CANCEL-SCOPE','reserved')`, schema), requestID, orderID); err != nil {
		t.Fatal(err)
	}
	id, err := r.FindDirectShipRequestForOrder(ctx, customerID, orderID)
	if err != nil || id != requestID {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if _, err = r.FindDirectShipRequestForOrder(ctx, otherID, orderID); err == nil {
		t.Fatal("cross customer cancellation lookup allowed")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET customer_id=$2 WHERE id=$1`, schema), orderID, otherID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.FindDirectShipRequestForOrder(ctx, customerID, orderID); err == nil {
		t.Fatal("mismatched order ownership allowed")
	}
}
