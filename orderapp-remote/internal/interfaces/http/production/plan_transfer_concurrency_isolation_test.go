package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	app "orderapp/internal/application/production"
	purchaseapp "orderapp/internal/application/purchase"
	stockapp "orderapp/internal/application/stock"
	pgprod "orderapp/internal/infrastructure/postgres/production"
	pgpurchase "orderapp/internal/infrastructure/postgres/purchase"
	pgstock "orderapp/internal/infrastructure/postgres/stock"
)

// A test-only trigger pauses a real incoming transaction just before its WIP
// write; the plan transfer must wait for the shared material lock then recheck.
func TestPlanTransferSerializesWithIncomingWIP(t *testing.T) {
	for _, incoming := range []string{"transfer", "purchase"} {
		t.Run(incoming, func(t *testing.T) {
			ctx := context.Background()
			pool, schema := newProductionFlowTestDB(t)
			if err := pgpurchase.EnsureSchema(ctx, pool, schema); err != nil {
				t.Fatal(err)
			}
			seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
			seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "CONCURRENT-WIP", "生产生豆", 36000)
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10; UPDATE %[1]s.material_batch_locations SET qty_g=6000 WHERE material_batch_id=1030; INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(1030,30,'raw_materials',30000)`, schema))
			e := newProductionFlowTestEcho(pool, schema)
			stock := stockapp.NewService(pgstock.NewRepository(pool, schema))
			registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stock)
			cr := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
			var plan app.ProductionPlanDetail
			if cr.Code != 200 || json.Unmarshal(cr.Body.Bytes(), &plan) != nil || len(plan.ComponentSources) != 1 {
				t.Fatalf("plan %d %s", cr.Code, cr.Body.String())
			}
			cmd := app.PlanMaterialTransferCommand{ProductionPlanID: plan.ID, DraftToken: plan.DraftToken, IdempotencyKey: "wip-concurrency", Operator: "tester", Items: []app.PlanMaterialTransferItem{{SourceID: plan.ComponentSources[0].ID, Warehouse: "raw_materials", Batches: []app.PlanMaterialTransferBatch{{BatchCode: "CONCURRENT-WIP", QtyG: 4000}}}}}
			cr = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), cmd)
			var out app.PlanMaterialTransferResult
			if cr.Code != 200 || json.Unmarshal(cr.Body.Bytes(), &out) != nil || len(out.Documents) != 1 {
				t.Fatalf("transfer %d %s", cr.Code, cr.Body.String())
			}
			var ordinaryID int64
			if incoming == "transfer" {
				cr = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/stock-documents", map[string]any{"purpose": "material_transfer", "items": []map[string]any{{"item_type": "material", "material_id": 30, "inventory_unit": "kg", "qty_g": 13000, "from_warehouse": "raw_materials", "to_warehouse": "wip", "batch_code": "CONCURRENT-WIP"}}})
				var doc stockapp.StockDocumentDetail
				if cr.Code != 200 || json.Unmarshal(cr.Body.Bytes(), &doc) != nil {
					t.Fatalf("ordinary draft %d %s", cr.Code, cr.Body.String())
				}
				ordinaryID = doc.ID
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.pause_wip() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.material_id=30 AND NEW.warehouse='wip' AND NEW.qty_g=13000 THEN PERFORM pg_advisory_xact_lock(757001); END IF; RETURN NEW; END $$; CREATE TRIGGER pause_wip BEFORE INSERT OR UPDATE ON %[1]s.material_batch_locations FOR EACH ROW EXECUTE FUNCTION %[1]s.pause_wip()`, schema))
			gate, err := pool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer gate.Release()
			if _, err = gate.Exec(ctx, "SELECT pg_advisory_lock(757001)"); err != nil {
				t.Fatal(err)
			}
			locked := true
			defer func() {
				if locked {
					_, _ = gate.Exec(context.Background(), "SELECT pg_advisory_unlock(757001)")
				}
			}()
			done := make(chan string, 1)
			go func() {
				if incoming == "transfer" {
					r := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", ordinaryID), map[string]any{})
					if r.Code != 200 {
						done <- fmt.Sprintf("HTTP %d %s", r.Code, r.Body.String())
						return
					}
					done <- ""
					return
				}
				svc := purchaseapp.NewService(pgpurchase.NewRepository(pool, schema), stock)
				_, err := svc.CreatePurchaseReceipt(ctx, purchaseapp.CreatePurchaseReceiptCommand{MaterialID: 30, SupplierName: "隔离测试供应商", Qty: 13, UnitCode: "kg", UnitCost: 50, TargetWarehouse: "wip", Operator: "tester"})
				if err != nil {
					done <- err.Error()
					return
				}
				done <- ""
			}()
			deadline := time.Now().Add(10 * time.Second)
			for {
				var n int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND objid=757001 AND NOT granted`).Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n > 0 {
					break
				}
				select {
				case why := <-done:
					t.Fatalf("incoming ended before pause: %s", why)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("incoming did not reach WIP write")
				}
				time.Sleep(20 * time.Millisecond)
			}
			planDone := make(chan string, 1)
			go func() {
				r := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", out.Documents[0].ID), map[string]any{})
				planDone <- fmt.Sprintf("%d %s", r.Code, r.Body.String())
			}()
			deadline = time.Now().Add(10 * time.Second)
			for {
				var n int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE $1 AND query ILIKE '%FOR UPDATE%'`, "%"+schema+".materials%").Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n > 0 {
					break
				}
				select {
				case response := <-planDone:
					t.Fatalf("plan did not wait: %s", response)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("plan did not wait on material lock")
				}
				time.Sleep(20 * time.Millisecond)
			}
			if _, err = gate.Exec(ctx, "SELECT pg_advisory_unlock(757001)"); err != nil {
				t.Fatal(err)
			}
			locked = false
			select {
			case why := <-done:
				if why != "" {
					t.Fatalf("incoming failed: %s", why)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("incoming stuck")
			}
			select {
			case response := <-planDone:
				if !strings.HasPrefix(response, "200 ") {
					t.Fatalf("plan should post once concurrent incoming is committed: %s", response)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("plan stuck")
			}
			var qty int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE material_id=30 AND warehouse='wip'`, schema)).Scan(&qty); err != nil || qty != 23000 {
				t.Fatalf("WIP=%d err=%v", qty, err)
			}
			var status string
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.stock_entries WHERE id=$1`, schema), out.Documents[0].ID).Scan(&status); err != nil || status != "submitted" {
				t.Fatalf("plan status=%s err=%v", status, err)
			}
		})
	}
}
