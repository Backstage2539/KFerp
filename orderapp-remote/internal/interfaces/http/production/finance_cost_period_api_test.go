package production

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	financeapp "orderapp/internal/application/finance"
	financepg "orderapp/internal/infrastructure/postgres/finance"
	"strings"
	"testing"
	"time"
)

func TestProductionWaitsForFinanceBeforeLockingMaterialBatch(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 99700)
	if err := financepg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, schema+":finance-period"); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"finished_units": 100, "consumed_input_g": 22700, "warehouse": "finished_goods", "note": "锁顺序验证"})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/produce/work-orders/%d/complete", wo), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		done <- rec
	}()
	select {
	case rec := <-done:
		t.Fatalf("production did not wait: %d %s", rec.Code, rec.Body.String())
	case <-time.After(300 * time.Millisecond):
	}
	observer, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	lockErr := observer.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.material_batches WHERE material_id=10 ORDER BY id LIMIT 1 FOR UPDATE NOWAIT`, schema)).Scan(&id)
	_ = observer.Rollback(ctx)
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case rec := <-done:
		if rec.Code != 200 {
			t.Errorf("production failed: %d %s", rec.Code, rec.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("production timed out")
	}
	if lockErr != nil {
		t.Fatalf("production locked material batch while waiting for finance: %v", lockErr)
	}
}

func TestProductionCostAndMonthCloseSerializeBothOrders(t *testing.T) {
	for _, closeFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("close_first_%v", closeFirst), func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 99500)
			if err := financepg.EnsureSchema(ctx, pool, schema); err != nil {
				t.Fatal(err)
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS grand_total numeric NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS discount_amount numeric NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS shipping_amount numeric NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS total_amount numeric NOT NULL DEFAULT 0`, schema))
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET cost_unavailable=true WHERE material_id=10`, schema))
			var month string
			if err := pool.QueryRow(ctx, `SELECT to_char(now(),'YYYY-MM')`).Scan(&month); err != nil {
				t.Fatal(err)
			}
			finance := financeapp.NewService(financepg.NewRepository(pool, schema))
			if closeFirst {
				if _, err := finance.CloseMonth(ctx, financeapp.CloseMonthCommand{Month: month, Actor: "test"}); err != nil {
					t.Fatalf("initial close: %v", err)
				}
			}
			lock, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback(ctx)
			if _, err = lock.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, schema+":finance-period"); err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(map[string]any{"finished_units": 100, "consumed_input_g": 22700, "warehouse": "finished_goods", "note": "隔离并发验证"})
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				req := httptest.NewRequest("POST", fmt.Sprintf("/api/produce/work-orders/%d/complete", wo), bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				done <- rec
			}()
			select {
			case r := <-done:
				t.Fatalf("production did not wait for finance lock: %d %s", r.Code, r.Body.String())
			case <-time.After(150 * time.Millisecond):
			}
			if err = lock.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case r := <-done:
				if r.Code != 200 {
					t.Fatalf("finish %d %s", r.Code, r.Body.String())
				}
			case <-time.After(10 * time.Second):
				t.Fatal("production timed out")
			}
			if !closeFirst {
				if _, err = finance.CloseMonth(ctx, financeapp.CloseMonthCommand{Month: month, Actor: "test"}); err == nil || !strings.Contains(err.Error(), "成本待核算") {
					t.Fatalf("close after unknown production: %v", err)
				}
			} else {
				dashboard, err := finance.Dashboard(ctx, month)
				if err != nil {
					t.Fatal(err)
				}
				if dashboard.Report.CostUnavailable {
					t.Fatal("late production changed closed snapshot")
				}
				found := false
				for _, ex := range dashboard.Exceptions {
					if ex.Code == "production_cost_after_close" {
						found = true
					}
				}
				if !found {
					t.Fatal("late production warning missing")
				}
			}
		})
	}
}

func TestMonthCloseWaitsForUncommittedProductionCost(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 99600)
	if err := financepg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS grand_total numeric NOT NULL DEFAULT 0;ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS discount_amount numeric NOT NULL DEFAULT 0;ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS shipping_amount numeric NOT NULL DEFAULT 0;ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS total_amount numeric NOT NULL DEFAULT 0;
 UPDATE %[1]s.material_batches SET cost_unavailable=true WHERE material_id=10;
 CREATE FUNCTION %[1]s.pause_cost_write() RETURNS trigger LANGUAGE plpgsql AS $f$ BEGIN PERFORM pg_advisory_xact_lock(hashtext('%[1]s:test-cost-writing')); PERFORM pg_sleep(0.8); RETURN NEW; END $f$;
 CREATE TRIGGER pause_cost_write BEFORE INSERT ON %[1]s.production_batch_costs FOR EACH ROW EXECUTE FUNCTION %[1]s.pause_cost_write();`, schema))
	var month string
	if err := pool.QueryRow(ctx, `SELECT to_char(now(),'YYYY-MM')`).Scan(&month); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"finished_units": 100, "consumed_input_g": 22700, "warehouse": "finished_goods", "note": "隔离并发验证"})
	produced := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/produce/work-orders/%d/complete", wo), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		produced <- rec
	}()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	deadline := time.Now().Add(8 * time.Second)
	observed := false
	for time.Now().Before(deadline) {
		var acquired bool
		if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, schema+":test-cost-writing").Scan(&acquired); err != nil {
			t.Fatal(err)
		}
		if !acquired {
			observed = true
			break
		}
		_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, schema+":test-cost-writing")
		time.Sleep(10 * time.Millisecond)
	}
	if !observed {
		t.Fatal("production never reached locked cost write")
	}
	finance := financeapp.NewService(financepg.NewRepository(pool, schema))
	closed := make(chan error, 1)
	go func() {
		_, err := finance.CloseMonth(ctx, financeapp.CloseMonthCommand{Month: month, Actor: "test"})
		closed <- err
	}()
	select {
	case err = <-closed:
		t.Fatalf("month close did not wait for production: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	select {
	case rec := <-produced:
		if rec.Code != 200 {
			t.Fatalf("production %d %s", rec.Code, rec.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("production timed out")
	}
	select {
	case err = <-closed:
		if err == nil || !strings.Contains(err.Error(), "成本待核算") {
			t.Fatalf("month close after unknown cost: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("month close timed out")
	}
}
