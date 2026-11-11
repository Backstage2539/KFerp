package sales

import (
	"context"
	"fmt"
	app "orderapp/internal/application/sales"
	"testing"
	"time"
)

func TestOrderReceiptPartialAndReplay(t *testing.T) {
	ctx := context.Background()
	repo, revision := combinedPaymentFixture(t)
	cmd := app.OrderReceiptCommand{OrderID: 11, EditRevision: revision(11), ActualCents: 2000, ExpectedDueCents: 7000, ReceiptDate: "2026-11-10", PaymentMethod: "现金", Actor: "tester", IdempotencyKey: "single-1"}
	first, err := repo.RegisterOrderReceipt(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := repo.RegisterOrderReceipt(ctx, cmd)
	if err != nil || retry.ReceiptID != first.ReceiptID || !retry.Reused {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	var count int
	var actual, receivable int64
	var date string
	err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),max(actual_cents),max(receivable_cents),max(receipt_date::text) FROM %s.cash_receipts`, repo.schema)).Scan(&count, &actual, &receivable, &date)
	if err != nil || count != 1 || actual != 2000 || receivable != 10000 || date != "2026-11-10" {
		t.Fatalf("%d %d %d %q %v", count, actual, receivable, date, err)
	}
	cmd.ActualCents = 2001
	if _, err = repo.RegisterOrderReceipt(ctx, cmd); err == nil {
		t.Fatal("changed replay accepted")
	}
	cmd.IdempotencyKey = "single-2"
	cmd.EditRevision = revision(11)
	cmd.ActualCents = 5000
	cmd.ExpectedDueCents = 5000
	if _, err = repo.RegisterOrderReceipt(ctx, cmd); err != nil {
		t.Fatalf("tail payment: %v", err)
	}
	var paid int64
	if err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(prepayment_amount*100)::bigint FROM %s.orders WHERE id=11`, repo.schema)).Scan(&paid); err != nil || paid != 10000 {
		t.Fatalf("tail payment paid=%d err=%v", paid, err)
	}
	if err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.cash_receipts`, repo.schema)).Scan(&count); err != nil || count != 2 {
		t.Fatalf("receipt count=%d err=%v", count, err)
	}
}

func TestOrderReceiptRejectsStaleAndRollsBack(t *testing.T) {
	ctx := context.Background()
	repo, revision := combinedPaymentFixture(t)
	cmd := app.OrderReceiptCommand{OrderID: 11, EditRevision: revision(11), ActualCents: 2000, ExpectedDueCents: 7000, ReceiptDate: "2026-11-10", PaymentMethod: "现金", Actor: "tester", IdempotencyKey: "stale"}
	cmd.EditRevision = "stale"
	if _, err := repo.RegisterOrderReceipt(ctx, cmd); err == nil {
		t.Fatal("stale receipt accepted")
	}
	var paid int64
	var receipts, requests, events int
	err := repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT round(prepayment_amount*100)::bigint FROM %[1]s.orders WHERE id=11),(SELECT count(*) FROM %[1]s.cash_receipts),(SELECT count(*) FROM %[1]s.order_single_receipt_requests),(SELECT count(*) FROM %[1]s.message_business_events)`, repo.schema)).Scan(&paid, &receipts, &requests, &events)
	if err != nil || paid != 3000 || receipts != 0 || requests != 0 || events != 0 {
		t.Fatalf("paid=%d receipts=%d requests=%d events=%d err=%v", paid, receipts, requests, events, err)
	}
	cmd.EditRevision = revision(11)
	cmd.ReceiptDate = ""
	if _, err = repo.RegisterOrderReceipt(ctx, cmd); err == nil {
		t.Fatal("missing actual receipt date accepted")
	}
}

func TestOrderReceiptRejectsClosedAccountingMonth(t *testing.T) {
	ctx := context.Background()
	repo, revision := combinedPaymentFixture(t)
	_, err := repo.pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.finance_settings(id INT PRIMARY KEY,closing_mode TEXT); INSERT INTO %[1]s.finance_settings VALUES(1,'strong_lock'); CREATE TABLE %[1]s.finance_monthly_reports(month TEXT,status TEXT); INSERT INTO %[1]s.finance_monthly_reports VALUES('2026-10','closed');`, repo.schema))
	if err != nil {
		t.Fatal(err)
	}
	cmd := app.OrderReceiptCommand{OrderID: 11, EditRevision: revision(11), ActualCents: 2000, ExpectedDueCents: 7000, ReceiptDate: "2026-10-10", PaymentMethod: "现金", Actor: "tester", IdempotencyKey: "closed"}
	if _, err = repo.RegisterOrderReceipt(ctx, cmd); err == nil {
		t.Fatal("closed accounting month accepted")
	}
	var count int
	if err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.cash_receipts`, repo.schema)).Scan(&count); err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestReceiptsWaitForFinanceLockBeforeLockingOrders(t *testing.T) {
	for _, mode := range []string{"single", "combined"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			repo, revision := combinedPaymentFixture(t)
			lock, err := repo.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback(ctx)
			if _, err = lock.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, repo.schema+":finance-period"); err != nil {
				t.Fatal(err)
			}
			single := app.OrderReceiptCommand{OrderID: 11, EditRevision: revision(11), ActualCents: 2000, ExpectedDueCents: 7000, ReceiptDate: "2026-11-10", PaymentMethod: "现金", Actor: "tester", IdempotencyKey: "period-single"}
			combined := combinedFixtureCommand(revision)
			combined.IdempotencyKey = "period-combined"
			finished := make(chan error, 1)
			go func() {
				if mode == "single" {
					_, e := repo.RegisterOrderReceipt(ctx, single)
					finished <- e
				} else {
					_, e := repo.CombinedOrderPayment(ctx, combined)
					finished <- e
				}
			}()
			deadline := time.Now().Add(3 * time.Second)
			waiting := false
			for time.Now().Before(deadline) {
				if err = repo.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%pg_advisory_xact_lock(hashtext($1))%')`).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case e := <-finished:
					t.Fatalf("operation finished before finance lock released: %v", e)
				default:
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !waiting {
				t.Fatal("payment did not wait for finance period lock")
			}
			probe, err := repo.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = probe.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.orders WHERE id=11 FOR UPDATE NOWAIT`, repo.schema))
			_ = probe.Rollback(ctx)
			if err != nil {
				t.Fatalf("order locked before finance lock: %v", err)
			}
			if err = lock.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("payment failed to resume")
			}
		})
	}
}
