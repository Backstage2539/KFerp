package customerportal

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerportal"
)

func TestOrderProgressCountsAndFilteringBeforePagination(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID, otherID, waitingID, producingID, doneID, shippedID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('overview-customer') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('overview-other') RETURNING id`, schema)).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	for name, dest := range map[string]*int64{"待生产": &waitingID, "生产中": &producingID, "生产完成": &doneID} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_process_statuses(name) VALUES($1) RETURNING id`, schema), name).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.order_shipment_orders(id BIGSERIAL PRIMARY KEY,order_id BIGINT,shipped_at TIMESTAMPTZ)`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,is_void,order_date,process_status_id) SELECT 'OV-'||n,$1,false,CURRENT_DATE,$2 FROM generate_series(1,25) n`, schema), customerID, waitingID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,is_void,order_date,process_status_id) VALUES('PRODUCING',$1,false,CURRENT_DATE,$2),('COMPLETE',$1,false,CURRENT_DATE,$3),('OTHER',$4,false,CURRENT_DATE,$2),('VOID',$1,true,CURRENT_DATE,$2)`, schema), customerID, producingID, doneID, otherID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,is_void,order_date,process_status_id) VALUES('PART-SHIPPED',$1,false,CURRENT_DATE,$2) RETURNING id`, schema), customerID, producingID).Scan(&shippedID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_shipment_orders(order_id,shipped_at) VALUES($1,now())`, schema), shippedID); err != nil {
		t.Fatal(err)
	}
	counts, err := repo.customerOrderProgressCounts(ctx, customerID)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Accepted != 25 || counts.InProduction != 1 || counts.Shipped != 1 {
		t.Fatalf("counts=%+v", counts)
	}
	rows, err := repo.listCustomerOrders(ctx, app.ServicePageQuery{CustomerID: customerID, Progress: "accepted", Offset: 20}, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("filtered second page=%d, want 5", len(rows))
	}
	for _, row := range rows {
		if row.ProcessStatus != "待生产" {
			t.Fatalf("wrong status: %+v", row)
		}
	}
}
