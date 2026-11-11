package customerfulfillment

import (
	"context"
	"fmt"
	d "orderapp/internal/domain/shippingreturn"
	fp "orderapp/internal/infrastructure/postgres/freight"
	sp "orderapp/internal/infrastructure/postgres/sales"
	"sync"
	"testing"
)

func TestShippingReturnRepeatAfterReceiptRematchesWithoutShipping(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	if err := fp.NewRepository(pool, schema).EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	r := sp.NewRepository(pool, schema)
	var oid int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(order_no,receiver_name,receiver_phone,process_status_id) VALUES('REMATCH','汝汀汀','13600006539',(SELECT id FROM %[1]s.order_process_statuses WHERE name='待生产' LIMIT 1)) RETURNING id`, schema)).Scan(&oid); err != nil {
		t.Fatal(err)
	}
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	rows := []d.Row{{RowNo: 3, TrackingNo: "SF5144416745654", ActualCents: 910, Receiver: "汝**", Phone: "136****6539", Status: "pending"}}
	before, err := r.ImportShippingReturn(ctx, "tester", "return.xlsx", "rematch-hash", cfg, rows)
	if err != nil || before.Rows[0].Status != "pending" || before.Updated != 0 {
		t.Fatalf("initial %+v %v", before, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.orders SET process_status_id=(SELECT id FROM %[1]s.order_process_statuses WHERE name='无需生产' LIMIT 1) WHERE id=%d`, schema, oid)); err != nil {
		t.Fatal(err)
	}
	after, err := r.ImportShippingReturn(ctx, "tester", "return.xlsx", "rematch-hash", cfg, rows)
	if err != nil || after.ID != before.ID || after.Pending != 1 || len(after.OrderIDs) != 0 {
		t.Fatalf("same-file rematch must refresh candidates without shipping: %+v %v", after, err)
	}
	choices, err := r.ShippingReturnCandidates(ctx, oid)
	if err != nil || len(choices) != 1 || choices[0].ID != before.Rows[0].ID {
		t.Fatalf("choices %+v %v", choices, err)
	}
	var writes int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT COUNT(*) FROM %[1]s.order_shipping_trackings)+(SELECT COUNT(*) FROM %[1]s.freight_actual_entries)+(SELECT COUNT(*) FROM %[1]s.order_stock_deductions)`, schema)).Scan(&writes); err != nil || writes != 0 {
		t.Fatalf("rematch business writes=%d %v", writes, err)
	}
	// Invalid scope and audit failure must leave the existing candidate untouched.
	if _, err = r.RematchShippingReturn(ctx, "tester", before.ID, []int64{999999}); err == nil {
		t.Fatal("foreign row accepted")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_rematch_audit() RETURNS trigger LANGUAGE plpgsql AS $fn$ BEGIN IF NEW.action='rematch' THEN RAISE EXCEPTION 'audit unavailable'; END IF; RETURN NEW; END $fn$; CREATE TRIGGER reject_rematch BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_rematch_audit(); UPDATE %[1]s.orders SET receiver_name='其他人' WHERE id=%d`, schema, oid)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.RematchShippingReturn(ctx, "tester", before.ID, nil); err == nil {
		t.Fatal("audit failure committed")
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.shipping_return_candidates WHERE row_id=$1`, schema), before.Rows[0].ID).Scan(&writes); err != nil || writes != 1 {
		t.Fatalf("rollback candidates %d %v", writes, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_rematch ON %[1]s.audit_logs; UPDATE %[1]s.orders SET receiver_name='汝汀汀' WHERE id=%d`, schema, oid)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := r.RematchShippingReturn(ctx, "tester", before.ID, []int64{before.Rows[0].ID})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	choices, err = r.ShippingReturnCandidates(ctx, oid)
	if err != nil || len(choices) != 1 {
		t.Fatalf("concurrent %+v %v", choices, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET receiver_name='其他人' WHERE id=%d`, schema, oid)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.RematchShippingReturn(ctx, "tester", before.ID, nil); err != nil {
		t.Fatal(err)
	}
	choices, err = r.ShippingReturnCandidates(ctx, oid)
	if err != nil || len(choices) != 0 {
		t.Fatalf("stale candidates %+v %v", choices, err)
	}

}

func TestShippingReturnWeightBackfillRejectsConflictsAndRollsBackWithoutAudit(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	if err := fp.NewRepository(pool, schema).EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	r := sp.NewRepository(pool, schema)
	cfg := d.DefaultTemplate()
	cfg.CarrierID = 1
	item := func(n int, tracking string) d.Row {
		return d.Row{RowNo: n, TrackingNo: tracking, Receiver: "张*", Phone: "138****1234", Status: "pending", ActualCents: 1200}
	}
	initial := []d.Row{item(2, "SF-A"), item(3, "SF-B"), item(4, "SF-DUP"), item(5, "SF-DUP"), item(6, "SF-E")}
	batch, err := r.ImportShippingReturn(ctx, "tester", "return.xlsx", "weight-conflicts", cfg, initial)
	if err != nil {
		t.Fatal(err)
	}
	changed := []d.Row{item(2, "SF-A"), item(3, "SF-C"), item(4, "SF-DUP"), item(5, "SF-DUP"), item(6, "SF-E")}
	for i := range changed {
		changed[i].SettlementWeightKG = "7.000"
	}
	changed[4].SettlementWeightKG = "NaN"
	result, err := r.ImportShippingReturn(ctx, "tester", "return.xlsx", "weight-conflicts", cfg, changed)
	if err != nil || result.WeightBackfilled != 1 || result.WeightSkipped != 4 || len(result.OrderIDs) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.shipping_return_rows WHERE batch_id=$1 AND payload ? 'settlement_weight_kg'`, schema), batch.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("stored weights=%d %v", count, err)
	}
	initial = []d.Row{item(2, "SF-CONCURRENT")}
	batch, err = r.ImportShippingReturn(ctx, "tester", "return.xlsx", "weight-concurrent", cfg, initial)
	if err != nil {
		t.Fatal(err)
	}
	initial[0].SettlementWeightKG = "7.000"
	var wg sync.WaitGroup
	results := make(chan d.Batch, 2)
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, e := r.ImportShippingReturn(ctx, "tester", "return.xlsx", "weight-concurrent", cfg, initial)
			results <- result
			errors <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for e := range errors {
		if e != nil {
			t.Fatal(e)
		}
	}
	totalBackfilled := 0
	for result := range results {
		totalBackfilled += result.WeightBackfilled
		if len(result.OrderIDs) != 0 {
			t.Fatalf("concurrent replay shipped: %+v", result)
		}
	}
	if totalBackfilled != 1 {
		t.Fatalf("concurrent replay backfilled %d times", totalBackfilled)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='shipping_return' AND entity_id=$1 AND action='backfill_settlement_weight'`, schema), batch.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent audit count=%d %v", count, err)
	}
	// A failed operation-log write must roll the weight change back as well.
	initial = []d.Row{item(2, "SF-AUDIT")}
	batch, err = r.ImportShippingReturn(ctx, "tester", "return.xlsx", "weight-audit", cfg, initial)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_weight_audit() RETURNS trigger LANGUAGE plpgsql AS $fn$ BEGIN IF NEW.action='backfill_settlement_weight' THEN RAISE EXCEPTION 'audit unavailable'; END IF; RETURN NEW; END $fn$; CREATE TRIGGER reject_weight_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_weight_audit()`, schema)); err != nil {
		t.Fatal(err)
	}
	initial[0].SettlementWeightKG = "7.000"
	if _, err = r.ImportShippingReturn(ctx, "tester", "return.xlsx", "weight-audit", cfg, initial); err == nil {
		t.Fatal("audit failure accepted")
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.shipping_return_rows WHERE batch_id=$1 AND payload ? 'settlement_weight_kg'`, schema), batch.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("audit failure left weight=%d %v", count, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_weight_audit ON %[1]s.audit_logs; CREATE FUNCTION %[1]s.reject_weight_rematch() RETURNS trigger LANGUAGE plpgsql AS $fn$ BEGIN IF NEW.action='rematch' THEN RAISE EXCEPTION 'rematch unavailable'; END IF; RETURN NEW; END $fn$; CREATE TRIGGER reject_weight_rematch BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_weight_rematch()`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ImportShippingReturn(ctx, "tester", "return.xlsx", "weight-audit", cfg, initial); err == nil {
		t.Fatal("rematch failure accepted")
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.shipping_return_rows WHERE batch_id=$1 AND payload ? 'settlement_weight_kg'`, schema), batch.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rematch failure left committed weight=%d %v", count, err)
	}
}
