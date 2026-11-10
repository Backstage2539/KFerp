package customerportal

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerportal"
)

func TestCustomerHiddenOrderDoesNotAppearInListCountsOrDetails(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)
	// Create the contract first so this test exposes the missing query filtering
	// before the schema and implementation are added.
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %[1]s.customer_portal_hidden_orders (
		customer_id bigint NOT NULL, order_id bigint NOT NULL, reason text NOT NULL,
		hidden_by text NOT NULL, hidden_at timestamptz NOT NULL DEFAULT now(),
		restored_at timestamptz, PRIMARY KEY(customer_id,order_id));`, schema))
	if err != nil {
		t.Fatal(err)
	}
	var a, b, waiting, hiddenID, visibleID, otherID int64
	for _, pair := range []struct {
		name string
		id   *int64
	}{{"客户A", &a}, {"客户B", &b}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES($1) RETURNING id`, schema), pair.name).Scan(pair.id); err != nil {
			t.Fatal(err)
		}
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_process_statuses(name) VALUES('待生产') RETURNING id`, schema)).Scan(&waiting); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		no       string
		customer int64
		id       *int64
	}{{"HIDE-A", a, &hiddenID}, {"KEEP-A", a, &visibleID}, {"KEEP-B", b, &otherID}} {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,is_void,order_date,process_status_id,grand_total) VALUES($1,$2,false,CURRENT_DATE,$3,2160) RETURNING id`, schema), row.no, row.customer, waiting).Scan(row.id); err != nil {
			t.Fatal(err)
		}
	}
	// A customer-facing visibility choice must not delete accounting or
	// inventory history, including already-issued settlement records.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.customer_stock_entitlements (
		source_order_id bigint NOT NULL, promised_qty bigint NOT NULL, closed_qty bigint NOT NULL)`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_stock_entitlements(source_order_id,promised_qty,closed_qty) VALUES($1,24,24)`, schema), hiddenID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_batches(customer_id,settlement_no,status,total_amount)
		VALUES($1,'HIDE-BILL','confirmed',2160)`, schema), a); err != nil {
		t.Fatal(err)
	}
	var batchID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customer_settlement_batches WHERE settlement_no='HIDE-BILL'`, schema)).Scan(&batchID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_lines(customer_id,settlement_batch_id,charge_type,source_type,source_id,occurred_at,amount_cents,due_cents)
		VALUES($1,$2,'order_goods','order',$3,CURRENT_DATE,216000,216000)`, schema), a, batchID, hiddenID); err != nil {
		t.Fatal(err)
	}
	// A marker for a different customer cannot hide the owner's order.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_portal_hidden_orders(customer_id,order_id,reason,hidden_by) VALUES($1,$2,'wrong customer','test')`, schema), b, hiddenID); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.listCustomerOrders(ctx, app.ServicePageQuery{CustomerID: a}, 50, false)
	if err != nil || len(rows) != 2 {
		t.Fatalf("cross-customer marker changed visibility: rows=%+v err=%v", rows, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_portal_hidden_orders(customer_id,order_id,reason,hidden_by) VALUES($1,$2,'用户要求隐藏','test')`, schema), a, hiddenID); err != nil {
		t.Fatal(err)
	}
	rows, err = repo.listCustomerOrders(ctx, app.ServicePageQuery{CustomerID: a}, 50, false)
	if err != nil || len(rows) != 1 || rows[0].ID != visibleID {
		t.Fatalf("list must hide exactly one order: rows=%+v err=%v", rows, err)
	}
	counts, err := repo.customerOrderProgressCounts(ctx, a)
	if err != nil || counts.Accepted != 1 {
		t.Fatalf("overview must agree with list: counts=%+v err=%v", counts, err)
	}
	rows, err = repo.listCustomerOrders(ctx, app.ServicePageQuery{CustomerID: a, OrderID: hiddenID}, 1, false)
	if err != nil || len(rows) != 0 {
		t.Fatalf("direct ID revealed hidden order: rows=%+v err=%v", rows, err)
	}
	owned, err := repo.CustomerOwnsOrder(ctx, a, hiddenID)
	if err != nil || owned {
		t.Fatalf("hidden order accessible through customer detail: owned=%v err=%v", owned, err)
	}
	rows, err = repo.listCustomerOrders(ctx, app.ServicePageQuery{CustomerID: b}, 50, false)
	if err != nil || len(rows) != 1 || rows[0].ID != otherID {
		t.Fatalf("other customer changed: rows=%+v err=%v", rows, err)
	}
	var isVoid bool
	var total float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT is_void,grand_total FROM %s.orders WHERE id=$1`, schema), hiddenID).Scan(&isVoid, &total); err != nil || isVoid || total != 2160 {
		t.Fatalf("business record changed: is_void=%v total=%v err=%v", isVoid, total, err)
	}
	var promised, closed, lineAmount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT promised_qty,closed_qty FROM %s.customer_stock_entitlements WHERE source_order_id=$1`, schema), hiddenID).Scan(&promised, &closed); err != nil || promised != 24 || closed != 24 {
		t.Fatalf("customer stock entitlement changed: promised=%v closed=%v err=%v", promised, closed, err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT amount_cents FROM %s.customer_settlement_lines WHERE settlement_batch_id=$1 AND source_id=$2`, schema), batchID, hiddenID).Scan(&lineAmount); err != nil || lineAmount != 216000 {
		t.Fatalf("issued bill line changed: amount=%v err=%v", lineAmount, err)
	}
	settlement, err := repo.LoadServicePage(ctx, app.ServicePageQuery{Key: app.ServiceKeySettlement, CustomerID: a})
	if err != nil || len(settlement.SettlementBatches) != 1 || settlement.SettlementBatches[0].ID != batchID {
		t.Fatalf("issued bill became inaccessible: batches=%+v err=%v", settlement.SettlementBatches, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_portal_hidden_orders SET restored_at=now() WHERE customer_id=$1 AND order_id=$2`, schema), a, hiddenID); err != nil {
		t.Fatal(err)
	}
	rows, err = repo.listCustomerOrders(ctx, app.ServicePageQuery{CustomerID: a}, 50, false)
	if err != nil || len(rows) != 2 {
		t.Fatalf("restoration failed: rows=%+v err=%v", rows, err)
	}
}

func TestHiddenOrderVisibilityLegacySchemaAndLookupFailure(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID, orderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('旧客户') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,is_void,order_date) VALUES('LEGACY',$1,false,CURRENT_DATE) RETURNING id`, schema), customerID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP TABLE %s.customer_portal_hidden_orders`, schema)); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.listCustomerOrders(ctx, app.ServicePageQuery{CustomerID: customerID}, 20, false)
	if err != nil || len(rows) != 1 {
		t.Fatalf("legacy list: rows=%+v err=%v", rows, err)
	}
	owns, err := repo.CustomerOwnsOrder(ctx, customerID, orderID)
	if err != nil || !owns {
		t.Fatalf("legacy access: owns=%v err=%v", owns, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.visibleCustomerOrderSQL(cancelled, "o"); err == nil {
		t.Fatal("database failure must not silently bypass order visibility")
	}
}
