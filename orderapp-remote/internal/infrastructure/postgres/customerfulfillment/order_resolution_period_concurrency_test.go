package customerfulfillment

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	domain "orderapp/internal/domain/orderresolution"
	financepg "orderapp/internal/infrastructure/postgres/finance"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	"testing"
	"time"
)

func TestOrderResolutionMonthLockCoversLegacySourceWrites(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	if err := financepg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(id,order_no,customer_id,order_date,total_amount)VALUES(78270,'MONTH-LOCK',501,'2026-10-01',90)`, schema)); err != nil {
		t.Fatal(err)
	}
	lock, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(ctx)
	if _, err = lock.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, schema+":finance-period"); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`UPDATE %s.orders SET total_amount=60 WHERE id=78270`,
		`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at)VALUES(501,'order',78270,'direct_ship_service',10,'2026-10-01')`,
		`INSERT INTO %s.finance_expenses(expense_date,month,category,amount)VALUES('2026-10-01','2026-10','测试费用',10)`,
		`INSERT INTO %s.finance_tax_ledger(month,kind,total_amount)VALUES('2026-10','input',10)`,
	} {
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(ctx, `SET LOCAL lock_timeout='150ms'`); e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(ctx, fmt.Sprintf(query, schema))
		tx.Rollback(ctx)
		var pg *pgconn.PgError
		if !errors.As(e, &pg) || pg.Code != "55P03" {
			t.Errorf("source writer bypassed monthly close lock: %s: %v", query, e)
		}
	}
	policyCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	_, policyErr := resolutionpg.NewRepository(pool, schema).SavePolicy(policyCtx, 1, domain.Policy{}, "admin")
	cancel()
	if !errors.Is(policyErr, context.DeadlineExceeded) {
		t.Errorf("policy changed during protected order execution: %v", policyErr)
	}
	if err = lock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finance_monthly_reports(month,status)VALUES('2026-10','closed')`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finance_expenses(expense_date,month,category,amount)VALUES('2026-10-01','2026-10','过期提交',10)`, schema)); err == nil {
		t.Fatal("old expense submit passed after month became strongly locked")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at)VALUES(501,'order',78270,'direct_ship_service',10,'2026-10-01')`, schema)); err == nil {
		t.Fatal("fee writer changed closed period")
	}

	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finance_expenses(expense_date,month,category,amount)VALUES('2026-11-01','2026-11','开放账期',10)`, schema)); err != nil {
		t.Fatalf("open month blocked: %v", err)
	}
}
