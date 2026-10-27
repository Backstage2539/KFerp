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
	if err != nil || before.Rows[0].Status != "unmatched" {
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
