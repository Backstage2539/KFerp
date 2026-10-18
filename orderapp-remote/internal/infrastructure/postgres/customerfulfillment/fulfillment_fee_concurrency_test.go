package customerfulfillment

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerfulfillment"
)

func seedConcurrentFulfillmentFeeOrder(t *testing.T, pool *pgxpool.Pool, schema string, r *Repository, no string) (int64, int64, app.FulfillmentFeeQuote) {
	t.Helper()
	ctx := context.Background()
	var customerID, orderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES($1) RETURNING id`, schema), no).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,total_amount,grand_total) VALUES($1,$2,0,0) RETURNING id`, schema), no, customerID).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	quote := app.FulfillmentFeeQuote{CustomerID: customerID, FulfillmentMode: "stock", SettingsRevision: 1, ServiceFeeCents: 900, ShippingFeeCents: 1800}
	quote.FeeQuoteToken = app.FulfillmentFeeQuoteToken(quote)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.SaveFulfillmentOrderFeesTx(ctx, tx, customerID, orderID, 0, "stock", quote, "tester"); err != nil {
		tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return customerID, orderID, quote
}

func waitForFulfillmentDBLock(ctx context.Context, pool *pgxpool.Pool, pid int64) error {
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `SELECT COALESCE(wait_event_type='Lock',false) FROM pg_stat_activity WHERE pid=$1`, pid).Scan(&waiting); err != nil {
			return err
		}
		if waiting {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestFulfillmentFeesLockOrderAndConcurrentCancellation(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	r := NewRepository(pool, schema)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	customerID, orderID, _ := seedConcurrentFulfillmentFeeOrder(t, pool, schema, r, "FEE-LOCK-ORDER")
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if _, err = blocker.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=$1 FOR UPDATE`, schema), orderID); err != nil {
		t.Fatal(err)
	}
	worker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := int64(worker.Conn().PgConn().PID())
	done := make(chan error, 1)
	go func() {
		e := r.ReverseFulfillmentOrderFeesTx(ctx, worker, customerID, orderID, "取消员工")
		if e == nil {
			e = worker.Commit(ctx)
		} else {
			worker.Rollback(ctx)
		}
		done <- e
	}()
	waitErr := waitForFulfillmentDBLock(ctx, pool, pid)
	observer, err := pool.Begin(ctx)
	if err != nil {
		blocker.Rollback(ctx)
		<-done
		t.Fatal(err)
	}
	_, snapshotErr := observer.Exec(ctx, fmt.Sprintf(`SELECT order_id FROM %s.fulfillment_order_fee_snapshots WHERE order_id=$1 FOR UPDATE NOWAIT`, schema), orderID)
	observer.Rollback(ctx)
	blocker.Rollback(ctx)
	workerErr := <-done
	if waitErr != nil || workerErr != nil {
		t.Fatalf("wait=%v reverse=%v", waitErr, workerErr)
	}
	if snapshotErr != nil {
		t.Fatalf("reverse locked snapshot before waiting for order: %v", snapshotErr)
	}

	// Force cancellation to win the order lock, then race the real adjustment
	// transaction against it. The adjustment must return an ordinary conflict.
	customerID, orderID, quote := seedConcurrentFulfillmentFeeOrder(t, pool, schema, r, "FEE-CANCEL-ADJUST")
	cancelling, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelling.Rollback(context.Background())
	if _, err = cancelling.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=$1 FOR UPDATE`, schema), orderID); err != nil {
		t.Fatal(err)
	}
	adjusted := make(chan error, 1)
	go func() {
		_, e := r.AdjustFulfillmentOrderFees(ctx, app.AdjustFulfillmentOrderFeesCommand{CustomerID: customerID, OrderID: orderID, ExpectedQuoteToken: quote.FeeQuoteToken, ServiceFeeCents: 1200, ShippingFeeCents: 2100, Reason: "员工核对", Actor: "调整员工"})
		adjusted <- e
	}()
	for {
		var waiting bool
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid<>pg_backend_pid() AND wait_event_type='Lock' AND position($1 in query)>0 AND position('SELECT COALESCE(is_void,false)' in query)>0)`, schema+".orders").Scan(&waiting); err != nil {
			cancelling.Rollback(ctx)
			<-adjusted
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			cancelling.Rollback(context.Background())
			<-adjusted
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err = r.ReverseFulfillmentOrderFeesTx(ctx, cancelling, customerID, orderID, "取消员工"); err != nil {
		cancelling.Rollback(ctx)
		<-adjusted
		t.Fatal(err)
	}
	if _, err = cancelling.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET is_void=true WHERE id=$1`, schema), orderID); err != nil {
		cancelling.Rollback(ctx)
		<-adjusted
		t.Fatal(err)
	}
	if err = cancelling.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	adjustErr := <-adjusted
	if adjustErr == nil || (!strings.Contains(adjustErr.Error(), "作废") && !strings.Contains(adjustErr.Error(), "已变化")) {
		t.Fatalf("expected ordinary adjustment conflict, got %v", adjustErr)
	}
	var amount, shipping float64
	var status string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT fee.amount::float8,fee.status,sale.shipping_amount::float8 FROM %[1]s.customer_fee_items fee JOIN %[1]s.orders sale ON sale.id=fee.source_id WHERE fee.customer_id=$1 AND fee.source_type='order' AND fee.source_id=$2 AND fee.fee_type='direct_ship_service'`, schema), customerID, orderID).Scan(&amount, &status, &shipping); err != nil || amount != 0 || shipping != 0 || status != "reversed" {
		t.Fatalf("amount=%v freight=%v status=%s err=%v", amount, shipping, status, err)
	}
}
