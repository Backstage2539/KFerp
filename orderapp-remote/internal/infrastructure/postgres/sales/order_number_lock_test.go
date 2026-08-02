package sales

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNextOrderNoWithAdvisoryLockSerializesConcurrentCreates(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.orders (order_no TEXT NOT NULL)`, schema)); err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first, err := nextOrderNoWithAdvisoryLock(ctx, tx1, schema, date)
	if err != nil {
		_ = tx1.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := tx1.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no) VALUES($1)`, schema), first); err != nil {
		_ = tx1.Rollback(ctx)
		t.Fatal(err)
	}

	result := make(chan struct {
		orderNo string
		err     error
	}, 1)
	started := make(chan struct{})
	go func() {
		tx2, beginErr := pool.Begin(ctx)
		close(started)
		if beginErr != nil {
			result <- struct {
				orderNo string
				err     error
			}{err: beginErr}
			return
		}
		second, nextErr := nextOrderNoWithAdvisoryLock(ctx, tx2, schema, date)
		if nextErr == nil {
			_, nextErr = tx2.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no) VALUES($1)`, schema), second)
		}
		if nextErr == nil {
			nextErr = tx2.Commit(ctx)
		} else {
			_ = tx2.Rollback(ctx)
		}
		result <- struct {
			orderNo string
			err     error
		}{orderNo: second, err: nextErr}
	}()
	<-started

	select {
	case premature := <-result:
		_ = tx1.Rollback(ctx)
		t.Fatalf("second create bypassed advisory lock: %+v", premature)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case second := <-result:
		if second.err != nil {
			t.Fatal(second.err)
		}
		if first != "SO-20260802-0001" || second.orderNo != "SO-20260802-0002" {
			t.Fatalf("serialized order numbers first=%q second=%q", first, second.orderNo)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second create did not resume after first transaction committed")
	}
}
