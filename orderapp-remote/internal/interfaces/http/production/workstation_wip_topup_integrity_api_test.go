package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
)

type wipTopUpAuditFixture struct {
	pool     *pgxpool.Pool
	schema   string
	e        *echo.Echo
	wo, card int64
	preview  app.WIPTopUpPreview
	item     app.WIPTopUpItem
	batch    app.WIPTopUpCandidate
}

func newWIPTopUpAuditFixture(t *testing.T) wipTopUpAuditFixture {
	t.Helper()
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88960)
	registerWIPComponentTopUpAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/job-cards/%d/", card)
	r := serveMultilevelProductionJSON(t, e, "GET", path+"start-preview", nil)
	if r.Code != 200 {
		t.Fatalf("start preview: %d %s", r.Code, r.Body.String())
	}
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	for i := range p.Materials {
		if app.OperationUnitFactor(p.Materials[i].Unit) > 0 {
			p.Materials[i].Qty = math.Round(p.Materials[i].Qty*400) / 1000
		}
	}
	r = serveMultilevelProductionJSON(t, e, "POST", path+"start", map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "audit-start", "difference_reason": "先投部分", "materials": p.Materials})
	if r.Code != 200 {
		t.Fatalf("start: %d %s", r.Code, r.Body.String())
	}
	f := wipTopUpAuditFixture{pool: pool, schema: schema, e: e, wo: wo, card: card}
	f.preview = f.getPreview(t)
	if !f.preview.Supported || len(f.preview.Items) != 1 || len(f.preview.Items[0].Candidates) == 0 {
		t.Fatalf("fixture lacks candidates: %+v", f.preview)
	}
	f.item = f.preview.Items[0]
	f.batch = f.item.Candidates[0]
	if f.batch.AvailableQty+1e-9 < f.item.ShortageQty {
		t.Fatalf("candidate does not cover shortage %+v", f.preview)
	}
	return f
}
func (f wipTopUpAuditFixture) path() string {
	return fmt.Sprintf("/api/job-cards/%d/wip-top-up", f.card)
}
func (f wipTopUpAuditFixture) getPreview(t *testing.T) app.WIPTopUpPreview {
	t.Helper()
	r := serveMultilevelProductionJSON(t, f.e, "GET", f.path(), nil)
	if r.Code != 200 {
		t.Fatalf("preview: %d %s", r.Code, r.Body.String())
	}
	var p app.WIPTopUpPreview
	if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}
func (f wipTopUpAuditFixture) command(id string) map[string]any {
	return map[string]any{"expected_version": f.preview.Version, "request_id": id, "materials": []map[string]any{{"key": f.item.Key, "batch_code": f.batch.BatchCode, "warehouse": f.batch.Warehouse, "owner_customer_id": f.batch.OwnerCustomerID, "qty": f.item.ShortageQty}}}
}
func (f wipTopUpAuditFixture) snapshot(t *testing.T) string {
	t.Helper()
	var state string
	err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT json_build_array(
 (SELECT COALESCE(sum(l.qty_g),0) FROM %[1]s.material_batch_locations l JOIN %[1]s.material_batches b ON b.id=l.material_batch_id WHERE b.batch_code=$1 AND l.warehouse='wip'),
 (SELECT COALESCE(sum(i.remaining_cost),0) FROM %[1]s.production_started_inputs i WHERE i.job_card_id=$2),
 (SELECT COALESCE(sum(c.remaining_qty),0) FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.job_card_id=$2),
 (SELECT count(*) FROM %[1]s.stock_entries WHERE job_card_id=$2 AND source_type='operation_material_add'),
 (SELECT count(*) FROM %[1]s.production_operation_events WHERE job_card_id=$2 AND kind='material_add'),
 (SELECT COALESCE(sum(b.reserved_g-b.consumed_g-b.returned_g),0) FROM %[1]s.work_order_material_reservation_batches b WHERE b.work_order_id=$3))::text`, f.schema), f.batch.BatchCode, f.card, f.wo).Scan(&state)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestWIPTopUpAuditFailureRollsBackAll(t *testing.T) {
	f := newWIPTopUpAuditFixture(t)
	ctx := context.Background()
	before := f.snapshot(t)
	mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(`ALTER TABLE %s.audit_logs ADD CONSTRAINT reject_wip_topup CHECK(action <> 'wip_component_topup')`, f.schema))
	r := serveMultilevelProductionJSON(t, f.e, "POST", f.path(), f.command("reject-audit"))
	if r.Code < 400 {
		t.Fatalf("audit failure accepted %d %s", r.Code, r.Body.String())
	}
	if after := f.snapshot(t); after != before {
		t.Fatalf("audit rollback changed stock/cost/records: before=%s after=%s", before, after)
	}
}

func TestWIPTopUpRejectsChangedIdentityAndRespectsReservations(t *testing.T) {
	cases := []string{"stale task", "quality held", "wrong owner", "other work reservation", "own work reservation"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			f := newWIPTopUpAuditFixture(t)
			ctx := context.Background()
			before := f.snapshot(t)
			switch name {
			case "stale task":
				mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(`UPDATE %s.job_cards SET operator=operator WHERE id=%d`, f.schema, f.card))
			case "quality held":
				_, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET quality_status='hold' WHERE batch_code=$1`, f.schema), f.batch.BatchCode)
				if err != nil {
					t.Fatal(err)
				}
			case "wrong owner":
				_, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET owner_customer_id=99 WHERE batch_code=$1`, f.schema), f.batch.BatchCode)
				if err != nil {
					t.Fatal(err)
				}
			case "other work reservation":
				_, err := f.pool.Exec(ctx, fmt.Sprintf(`WITH reservation AS (INSERT INTO %[1]s.work_order_material_reservations(work_order_id,material_id,status) VALUES(999999,10,'reserved') RETURNING id)
 INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,batch_code,warehouse,owner_customer_id,reserved_g,status) SELECT reservation.id,999999,10,'material',10,b.id,b.batch_code,'wip',0,l.qty_g,'reserved' FROM reservation,%[1]s.material_batches b JOIN %[1]s.material_batch_locations l ON l.material_batch_id=b.id WHERE b.batch_code=$1 AND l.warehouse='wip'`, f.schema), f.batch.BatchCode)
				if err != nil {
					t.Fatal(err)
				}
			case "own work reservation":
				tag, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.work_order_material_reservation_batches b SET reserved_g=consumed_g+returned_g+l.qty_g,status='reserved' FROM %[1]s.material_batches m JOIN %[1]s.material_batch_locations l ON l.material_batch_id=m.id WHERE b.work_order_id=$1 AND b.material_batch_id=m.id AND b.warehouse='wip' AND l.warehouse='wip' AND m.batch_code=$2`, f.schema), f.wo, f.batch.BatchCode)
				if err != nil {
					t.Fatal(err)
				}
				if tag.RowsAffected() != 1 {
					t.Fatalf("own reservation fixture matched %d rows", tag.RowsAffected())
				}
				p := f.getPreview(t)
				if len(p.Items) != 1 {
					t.Fatalf("own reserved preview missing: %+v", p)
				}
				found := false
				for _, c := range p.Items[0].Candidates {
					if c.BatchCode == f.batch.BatchCode && c.AvailableQty+1e-8 >= f.item.ShortageQty {
						found = true
					}
				}
				if !found {
					t.Fatalf("own reservation should remain usable: %+v", p)
				}
				f.preview = p
				r := serveMultilevelProductionJSON(t, f.e, "POST", f.path(), f.command("own-reserved"))
				if r.Code != 200 {
					t.Fatalf("own reservation topup failed %d %s", r.Code, r.Body.String())
				}
				return
			}
			before = f.snapshot(t)
			r := serveMultilevelProductionJSON(t, f.e, "POST", f.path(), f.command("stale-"+strings.ReplaceAll(name, " ", "-")))
			if r.Code < 400 {
				t.Fatalf("%s unexpectedly accepted: %d %s", name, r.Code, r.Body.String())
			}
			if after := f.snapshot(t); after != before {
				t.Fatalf("%s changed stock/cost/records: %s -> %s", name, before, after)
			}
			if name != "stale task" {
				fresh := f.getPreview(t)
				for _, it := range fresh.Items {
					for _, c := range it.Candidates {
						if c.BatchCode == f.batch.BatchCode && c.AvailableQty > 1e-9 {
							t.Fatalf("%s batch still available: %+v", name, c)
						}
					}
				}
			}
		})
	}
}

func TestWIPTopUpConcurrentDifferentKeysPostsOnlyOnce(t *testing.T) {
	f := newWIPTopUpAuditFixture(t)
	before := f.snapshot(t)
	ch := make(chan int, 2)
	ready := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func(i int) {
			<-ready
			r := serveMultilevelProductionJSON(t, f.e, "POST", f.path(), f.command(fmt.Sprintf("concurrent-%d", i)))
			ch <- r.Code
		}(i)
	}
	close(ready)
	a, b := <-ch, <-ch
	successes := 0
	for _, c := range []int{a, b} {
		if c == 200 {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one posting, got %d/%d before=%s after=%s", a, b, before, f.snapshot(t))
	}
	assertProductionFlowCount(t, f.pool, f.schema, "production_operation_events", "kind='material_add' AND status='posted'", 1)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "source_type='operation_material_add' AND status='submitted'", 1)
}
