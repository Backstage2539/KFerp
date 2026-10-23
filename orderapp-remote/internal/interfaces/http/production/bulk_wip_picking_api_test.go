package production

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Exercise the HTTP workflow with an actually submitted automatic-picking plan:
// the source batch has 120 kg, while this work order reserves only 15.57 kg.
type bulkPickingFixture struct {
	pool        *pgxpool.Pool
	schema      string
	app         *echo.Echo
	workOrderID int64
	ownerID     int64
}

func newBulkPickingFixture(t *testing.T) bulkPickingFixture {
	return newBulkPickingFixtureUnit(t, "kg")
}

func newBulkPickingFixtureUnit(t *testing.T, unit string) bulkPickingFixture {
	return newBulkPickingFixtureOwner(t, unit, 0)
}

func newBulkPickingFixtureOwner(t *testing.T, unit string, ownerID int64) bulkPickingFixture {
	t.Helper()
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 72230, 30, "PR722-BAG", "生产生豆", 120000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=72230;
 `, schema))
	if ownerID > 0 {
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.customers(id,name) VALUES(%[2]d,'PR722委托客户'); UPDATE %[1]s.materials SET owner_customer_id=%[2]d WHERE id IN(10,30); UPDATE %[1]s.material_batches SET owner_customer_id=%[2]d WHERE id=72230;`, schema, ownerID))
	}
	if unit == "g" {
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET unit='g',cost_unit='g' WHERE id=30`, schema))
	} else if unit == "个" {
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 UPDATE %[1]s.materials SET unit='个',cost_unit='个',kind='pack',onhand_g=0,onhand_units=120 WHERE id=30;
 UPDATE %[1]s.material_batches SET qty_g=0,remaining_g=0,received_g=0,qty_units=120,remaining_units=120 WHERE id=72230;
 UPDATE %[1]s.material_batch_locations SET qty_g=0,qty_units=120 WHERE material_batch_id=72230;
 UPDATE %[1]s.production_bom_version_items SET consume_unit='unit',qty_per_unit=1 WHERE version_id=200 AND material_id=30;
 `, schema))
	}
	app := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(app, productionapp.NewService(postgresproduction.NewRepository(pool, schema)), stockapp.NewService(postgresstock.NewRepository(pool, schema)))
	f := bulkPickingFixture{pool: pool, schema: schema, app: app, ownerID: ownerID}
	if unit == "个" {
		f.workOrderID = f.createWorkOrder(t, 12)
	} else {
		f.workOrderID = f.createWorkOrder(t, 12.456)
	}
	return f
}

func (f bulkPickingFixture) createWorkOrder(t *testing.T, outputKG float64) int64 {
	t.Helper()
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": outputKG, "target_warehouse": "wip"}}})
	if r.Code != http.StatusOK {
		t.Fatalf("create automatic plan: %d %s", r.Code, r.Body.String())
	}
	var plan productionapp.ProductionPlanDetail
	if err := json.Unmarshal(r.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,'烘焙',GREATEST(planned_g,planned_output_g)/1000.0,'kg',15,1,GREATEST(planned_g,planned_output_g)/1000.0,GREATEST(planned_g,planned_output_g),15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, f.schema, plan.ID))
	r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if r.Code != http.StatusOK {
		t.Fatalf("submit automatic plan: %d %s", r.Code, r.Body.String())
	}
	var id int64
	if err := f.pool.QueryRow(context.Background(), fmt.Sprintf("SELECT id FROM %s.work_orders WHERE production_plan_id=$1", f.schema), plan.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f bulkPickingFixture) command(qty ...int64) map[string]any {
	items := make([]map[string]any, 0, len(qty))
	for _, g := range qty {
		items = append(items, map[string]any{"item_type": "material", "material_id": 30, "inventory_unit": "kg", "qty_g": g, "from_warehouse": "raw_materials", "to_warehouse": "wip", "batch_code": "PR722-BAG", "owner_customer_id": f.ownerID})
	}
	return map[string]any{"purpose": stockapp.PurposeMaterialTransferForManufacture, "work_order_id": f.workOrderID, "return_source": "workstation", "items": items}
}

func (f bulkPickingFixture) draft(t *testing.T, qty ...int64) stockapp.StockDocumentDetail {
	t.Helper()
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/stock-documents", f.command(qty...))
	return decodeBulkPickingDocument(t, r, "save picking draft")
}

func decodeBulkPickingDocument(t *testing.T, r *httptest.ResponseRecorder, action string) stockapp.StockDocumentDetail {
	t.Helper()
	if r.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", action, r.Code, r.Body.String())
	}
	var detail stockapp.StockDocumentDetail
	if err := json.Unmarshal(r.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	return detail
}

func (f bulkPickingFixture) post(t *testing.T, id int64, action string) *httptest.ResponseRecorder {
	t.Helper()
	return serveMultilevelProductionJSON(t, f.app, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/%s", id, action), nil)
}

func (f bulkPickingFixture) quantities(t *testing.T, raw, wip, rawReserved, wipReserved int64) {
	t.Helper()
	var got [4]int64
	err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT
 COALESCE((SELECT SUM(qty_g) FROM %[1]s.material_batch_locations WHERE material_batch_id=72230 AND warehouse='raw_materials'),0),
 COALESCE((SELECT SUM(qty_g) FROM %[1]s.material_batch_locations WHERE material_batch_id=72230 AND warehouse='wip'),0),
 COALESCE((SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=$1 AND material_batch_id=72230 AND warehouse='raw_materials' AND status='reserved'),0),
 COALESCE((SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=$1 AND material_batch_id=72230 AND warehouse='wip' AND status='reserved'),0)`, f.schema), f.workOrderID).Scan(&got[0], &got[1], &got[2], &got[3])
	if err != nil {
		t.Fatal(err)
	}
	if want := [4]int64{raw, wip, rawReserved, wipReserved}; got != want {
		t.Fatalf("physical raw/WIP and own reserved raw/WIP = %v, want %v", got, want)
	}
}

func (f bulkPickingFixture) auditQuantities(t *testing.T, id int64, action string, actualG, actualN, reservedG, reservedN, freeG, freeN int64) {
	t.Helper()
	var raw []byte
	if err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT meta FROM %s.audit_logs WHERE entity_type='stock_entry' AND entity_id=$1 AND action=$2`, f.schema), id, action).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]int64{"actual_qty_g": actualG, "actual_qty_units": actualN, "reservation_moved_g": reservedG, "reservation_moved_units": reservedN, "free_wip_g": freeG, "free_wip_units": freeN} {
		var got int64
		if err := json.Unmarshal(meta[key], &got); err != nil || got != want {
			t.Fatalf("%s audit %s = %s, want %d (err=%v)", action, key, meta[key], want, err)
		}
	}
}

func TestPR722BulkPickingAPIWholeBagDraftReloadSubmitAndCancel(t *testing.T) {
	f := newBulkPickingFixture(t)
	f.quantities(t, 120000, 0, 15570, 0)
	draft := f.draft(t, 60000)
	loaded := decodeBulkPickingDocument(t, serveMultilevelProductionJSON(t, f.app, http.MethodGet, fmt.Sprintf("/api/stock-documents/%d", draft.ID), nil), "reload draft")
	if loaded.Status != "draft" || len(loaded.Items) != 1 || loaded.Items[0].QtyG != 60000 || !loaded.Items[0].FrozenPicking {
		t.Fatalf("whole bag draft lost quantity/frozen source: %+v", loaded)
	}
	preview := serveMultilevelProductionJSON(t, f.app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/stock-document-preview", f.workOrderID), map[string]any{"action": "issue", "stock_document_id": draft.ID, "return_source": "workstation"})
	if preview.Code != http.StatusOK {
		t.Fatalf("restore preview: %d %s", preview.Code, preview.Body.String())
	}
	var restored productionapp.StockDocumentPreview
	if err := json.Unmarshal(preview.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Document.Items) != 1 || restored.Document.Items[0].QtyG != 60000 {
		t.Fatalf("preview trimmed saved 60kg: %+v", restored.Document.Items)
	}
	f.quantities(t, 120000, 0, 15570, 0)
	first := decodeBulkPickingDocument(t, f.post(t, draft.ID, "submit"), "submit 60kg against 15.57kg reservation")
	if first.Status != "submitted" {
		t.Fatalf("submitted status=%s", first.Status)
	}
	f.quantities(t, 60000, 60000, 0, 15570)
	decodeBulkPickingDocument(t, f.post(t, draft.ID, "submit"), "idempotent submit")
	f.quantities(t, 60000, 60000, 0, 15570)
	assertProductionFlowCount(t, f.pool, f.schema, "audit_logs", fmt.Sprintf("entity_type='stock_entry' AND entity_id=%d AND action='submit'", draft.ID), 1)
	f.auditQuantities(t, draft.ID, "submit", 60000, 0, 15570, 0, 44430, 0)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entry_items", fmt.Sprintf("stock_entry_id=%d AND picking_reservation_snapshot IS NOT NULL", draft.ID), 1)

	// Once the reservation has been moved, another legitimate bag top-up is all free WIP.
	second := f.draft(t, 20000)
	decodeBulkPickingDocument(t, f.post(t, second.ID, "submit"), "top up after reservation is satisfied")
	f.quantities(t, 40000, 80000, 0, 15570)
	f.auditQuantities(t, second.ID, "submit", 20000, 0, 0, 0, 20000, 0)
	// Cancelling the first document before the second must reverse only its own reservation.
	decodeBulkPickingDocument(t, f.post(t, draft.ID, "cancel"), "cancel first document before second")
	f.quantities(t, 100000, 20000, 15570, 0)
	f.auditQuantities(t, draft.ID, "cancel", 60000, 0, 15570, 0, 44430, 0)
	decodeBulkPickingDocument(t, f.post(t, second.ID, "cancel"), "cancel free-only second document")
	f.quantities(t, 120000, 0, 15570, 0)
	f.auditQuantities(t, second.ID, "cancel", 20000, 0, 0, 0, 20000, 0)
	for _, id := range []int64{draft.ID, second.ID} {
		assertProductionFlowCount(t, f.pool, f.schema, "audit_logs", fmt.Sprintf("entity_type='stock_entry' AND entity_id=%d AND action='cancel'", id), 1)
	}
	assertProductionFlowCount(t, f.pool, f.schema, "work_orders", fmt.Sprintf("id=%d AND status='released'", f.workOrderID), 1)
}

func TestPR722BulkPickingAPIDuplicateLinesAndOtherWorkOrderProtection(t *testing.T) {
	f := newBulkPickingFixture(t)
	other := f.createWorkOrder(t, 40) // another 50kg reservation on the same physical batch
	draft := f.draft(t, 40000, 40000)
	r := f.post(t, draft.ID, "submit")
	if r.Code != http.StatusBadRequest {
		t.Fatalf("80kg must not consume other work order's 50kg reservation: %d %s", r.Code, r.Body.String())
	}
	if !strings.Contains(r.Body.String(), "库存") && !strings.Contains(r.Body.String(), "预留") {
		t.Fatalf("expected stock/reservation protection reason: %s", r.Body.String())
	}
	f.quantities(t, 120000, 0, 15570, 0)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", fmt.Sprintf("id=%d AND status='draft'", draft.ID), 1)
	assertProductionFlowCount(t, f.pool, f.schema, "audit_logs", fmt.Sprintf("entity_type='stock_entry' AND entity_id=%d AND action='submit'", draft.ID), 0)
	decodeBulkPickingDocument(t, serveMultilevelProductionJSON(t, f.app, http.MethodPut, fmt.Sprintf("/api/stock-documents/%d", draft.ID), f.command(30000, 30000)), "correct duplicate batch rows")
	decodeBulkPickingDocument(t, f.post(t, draft.ID, "submit"), "60kg aggregate may use own reservation and free stock")
	f.quantities(t, 60000, 60000, 0, 15570)
	assertProductionFlowCount(t, f.pool, f.schema, "work_order_material_reservation_batches", fmt.Sprintf("work_order_id=%d AND warehouse='raw_materials' AND reserved_g=50000 AND consumed_g=0", other), 1)
}

func TestPR722BulkPickingAPIConcurrentWholeBags(t *testing.T) {
	for _, differentOrders := range []bool{false, true} {
		t.Run(fmt.Sprintf("different_orders_%t", differentOrders), func(t *testing.T) {
			f := newBulkPickingFixture(t)
			other := f
			if differentOrders {
				other.workOrderID = f.createWorkOrder(t, 12.456)
			}
			first, second := f.draft(t, 70000), other.draft(t, 70000)
			results := make(chan *httptest.ResponseRecorder, 2)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for _, id := range []int64{first.ID, second.ID} {
				wg.Add(1)
				go func(id int64) {
					defer wg.Done()
					<-start
					req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", id), nil)
					rec := httptest.NewRecorder()
					f.app.ServeHTTP(rec, req)
					results <- rec
				}(id)
			}
			close(start)
			wg.Wait()
			close(results)
			passed, failed := 0, 0
			var submitted stockapp.StockDocumentDetail
			for r := range results {
				if r.Code == http.StatusOK {
					passed++
					submitted = decodeBulkPickingDocument(t, r, "concurrent successful issue")
				} else if r.Code == http.StatusBadRequest && strings.Contains(r.Body.String(), "库存") {
					failed++
				} else {
					t.Fatalf("concurrent loser must report stock shortage rather than deadlock or stale reservation: %d %s", r.Code, r.Body.String())
				}
			}
			if passed != 1 || failed != 1 {
				t.Fatalf("120kg permits only one concurrent 70kg issue: passed=%d failed=%d", passed, failed)
			}
			winner := f
			winner.workOrderID = submitted.WorkOrderID
			winner.quantities(t, 50000, 70000, 0, 15570)
			if differentOrders {
				loser := other
				if submitted.WorkOrderID == other.workOrderID {
					loser = f
				}
				loser.quantities(t, 50000, 70000, 15570, 0)
			}
			assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted'", 1)
		})
	}
}

func TestPR722BulkPickingAPIInvalidInputAndEmployeeBinding(t *testing.T) {
	f := newBulkPickingFixture(t)
	for _, qty := range []int64{0, -60000} {
		r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/stock-documents", f.command(qty))
		if r.Code != http.StatusBadRequest {
			t.Fatalf("invalid quantity %d accepted: %d %s", qty, r.Code, r.Body.String())
		}
		if !strings.Contains(r.Body.String(), "数量") {
			t.Fatalf("invalid quantity failed for unrelated reason: %s", r.Body.String())
		}
	}
	// These altered sources really have eligible stock. Rejection must enforce
	// this plan's frozen identity, rather than accidentally rely on empty stock.
	seedProductionFlowWIPBatch(t, context.Background(), f.pool, f.schema, 72231, 30, "UNRELATED-BATCH", "生产生豆", 120000)
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`
 UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=72231;
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES(72230,'PR722-BAG',30,'finished_goods',120000);
 UPDATE %[1]s.material_batches SET qty_g=240000,received_g=240000,remaining_g=240000 WHERE id=72230;
 `, f.schema))
	for _, identity := range []map[string]any{{"owner_customer_id": 72202}, {"batch_code": "UNRELATED-BATCH"}, {"from_warehouse": "finished_goods"}} {
		cmd := f.command(1000)
		item := cmd["items"].([]map[string]any)[0]
		for k, v := range identity {
			item[k] = v
		}
		r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/stock-documents", cmd)
		if r.Code == http.StatusOK {
			draft := decodeBulkPickingDocument(t, r, "save invalid source draft")
			r = f.post(t, draft.ID, "submit")
		}
		if r.Code != http.StatusBadRequest {
			t.Fatalf("source identity alteration %v accepted: %d %s", identity, r.Code, r.Body.String())
		}
		if !strings.Contains(r.Body.String(), "货主") && !strings.Contains(r.Body.String(), "来源") && !strings.Contains(r.Body.String(), "预留") {
			t.Fatalf("source alteration failed for unrelated reason: %s", r.Body.String())
		}
	}
	unbound := echo.New()
	registerStockEntryAPI(unbound, productionapp.NewService(postgresproduction.NewRepository(f.pool, f.schema)), stockapp.NewService(postgresstock.NewRepository(f.pool, f.schema)))
	r := serveMultilevelProductionJSON(t, unbound, http.MethodPost, "/api/stock-documents", f.command(60000))
	if r.Code != http.StatusBadRequest {
		t.Fatalf("unbound employee allowed write: %d %s", r.Code, r.Body.String())
	}
	f.quantities(t, 120000, 0, 15570, 0)
}

func TestPR722BulkPickingAPIGramAndCountUnits(t *testing.T) {
	for _, unit := range []string{"g", "个"} {
		t.Run(unit, func(t *testing.T) {
			f := newBulkPickingFixtureUnit(t, unit)
			cmd := f.command(60000)
			item := cmd["items"].([]map[string]any)[0]
			item["inventory_unit"] = unit
			if unit == "个" {
				item["qty_g"] = 0
				item["qty_units"] = 60
			}
			draft := decodeBulkPickingDocument(t, serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/stock-documents", cmd), "save unit-specific issue")
			posted := decodeBulkPickingDocument(t, f.post(t, draft.ID, "submit"), "post unit-specific whole bag")
			if unit == "g" {
				if posted.Items[0].QtyG != 60000 || posted.Items[0].QtyUnits != 0 {
					t.Fatalf("grams changed dimensions: %+v", posted.Items[0])
				}
				f.quantities(t, 60000, 60000, 0, 15570)
				f.auditQuantities(t, draft.ID, "submit", 60000, 0, 15570, 0, 44430, 0)
			} else {
				if posted.Items[0].QtyG != 0 || posted.Items[0].QtyUnits != 60 {
					t.Fatalf("count changed dimensions: %+v", posted.Items[0])
				}
				assertProductionFlowCount(t, f.pool, f.schema, "material_batch_locations", "material_batch_id=72230 AND warehouse='raw_materials' AND qty_g=0 AND qty_units=60", 1)
				assertProductionFlowCount(t, f.pool, f.schema, "material_batch_locations", "material_batch_id=72230 AND warehouse='wip' AND qty_g=0 AND qty_units=60", 1)
				assertProductionFlowCount(t, f.pool, f.schema, "work_order_material_reservation_batches", fmt.Sprintf("work_order_id=%d AND warehouse='wip' AND reserved_g=0 AND reserved_units=12", f.workOrderID), 1)
				f.auditQuantities(t, draft.ID, "submit", 0, 60, 0, 12, 0, 48)
			}
		})
	}
}

func TestPR722BulkPickingAPIQualityHoldAndSubmitAuditAtomicity(t *testing.T) {
	f := newBulkPickingFixture(t)
	draft := f.draft(t, 60000)
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`UPDATE %s.material_batches SET quality_status='hold' WHERE id=72230`, f.schema))
	r := f.post(t, draft.ID, "submit")
	if r.Code != http.StatusBadRequest || (!strings.Contains(r.Body.String(), "质检") && !strings.Contains(r.Body.String(), "冻结")) {
		t.Fatalf("held batch must fail for quality reason: %d %s", r.Code, r.Body.String())
	}
	f.quantities(t, 120000, 0, 15570, 0)
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`
 UPDATE %[1]s.material_batches SET quality_status='pass' WHERE id=72230;
 CREATE FUNCTION %[1]s.reject_bulk_issue_audit() RETURNS trigger AS $body$
 BEGIN IF NEW.entity_type='stock_entry' AND NEW.action='submit' THEN RAISE EXCEPTION 'forced bulk picking audit failure'; END IF; RETURN NEW; END; $body$ LANGUAGE plpgsql;
 CREATE TRIGGER reject_bulk_issue_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_bulk_issue_audit();
 `, f.schema))
	r = f.post(t, draft.ID, "submit")
	if r.Code != http.StatusBadRequest || !strings.Contains(r.Body.String(), "forced bulk picking audit failure") {
		t.Fatalf("submit audit failure: %d %s", r.Code, r.Body.String())
	}
	f.quantities(t, 120000, 0, 15570, 0)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", fmt.Sprintf("id=%d AND status='draft'", draft.ID), 1)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entry_items", fmt.Sprintf("stock_entry_id=%d AND picking_reservation_snapshot IS NOT NULL", draft.ID), 0)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_ledger_entries", fmt.Sprintf("source_doc_type='stock_entry' AND source_doc_id=%d", draft.ID), 0)
}

func TestPR722BulkPickingAPICancellationRejectsConsumedOrReservedSurplus(t *testing.T) {
	for _, occupied := range []bool{false, true} {
		t.Run(fmt.Sprintf("other_reservation_%t", occupied), func(t *testing.T) {
			f := newBulkPickingFixture(t)
			draft := f.draft(t, 60000)
			decodeBulkPickingDocument(t, f.post(t, draft.ID, "submit"), "post whole bag before downstream use")
			wip, ownReserved := int64(60000), int64(15570)
			if occupied {
				other := f.createWorkOrder(t, 24) // 30kg of the free 44.43kg is now reserved by another order.
				assertProductionFlowCount(t, f.pool, f.schema, "work_order_material_reservation_batches", fmt.Sprintf("work_order_id=%d AND warehouse='wip' AND reserved_g=30000", other), 1)
			} else {
				cmd := f.command(10000)
				cmd["purpose"] = stockapp.PurposeMaterialConsumption
				item := cmd["items"].([]map[string]any)[0]
				item["from_warehouse"] = "wip"
				delete(item, "to_warehouse")
				consume := decodeBulkPickingDocument(t, serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/stock-documents", cmd), "save partial consumption")
				decodeBulkPickingDocument(t, f.post(t, consume.ID, "submit"), "consume 10kg in WIP")
				wip, ownReserved = 50000, 5570
			}
			r := f.post(t, draft.ID, "cancel")
			if r.Code != http.StatusBadRequest || (!strings.Contains(r.Body.String(), "预留") && !strings.Contains(r.Body.String(), "消耗") && !strings.Contains(r.Body.String(), "库存")) {
				t.Fatalf("cancel occupied or consumed stock must explain conflict: %d %s", r.Code, r.Body.String())
			}
			f.quantities(t, 60000, wip, 0, ownReserved)
			assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", fmt.Sprintf("id=%d AND status='submitted'", draft.ID), 1)
			assertProductionFlowCount(t, f.pool, f.schema, "audit_logs", fmt.Sprintf("entity_type='stock_entry' AND entity_id=%d AND action='cancel'", draft.ID), 0)
		})
	}
}

func TestPR722BulkPickingAPILegacyPostedDocumentCancellation(t *testing.T) {
	f := newBulkPickingFixture(t)
	draft := f.draft(t, 10000)
	decodeBulkPickingDocument(t, f.post(t, draft.ID, "submit"), "submit ordinary partial picking")
	f.quantities(t, 110000, 10000, 5570, 10000)
	// NULL is the persisted shape of documents posted before this release.
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`UPDATE %s.stock_entry_items SET picking_reservation_snapshot=NULL WHERE stock_entry_id=%d`, f.schema, draft.ID))
	decodeBulkPickingDocument(t, f.post(t, draft.ID, "cancel"), "cancel legacy posted partial picking")
	f.quantities(t, 120000, 0, 15570, 0)
	assertProductionFlowCount(t, f.pool, f.schema, "audit_logs", fmt.Sprintf("entity_type='stock_entry' AND entity_id=%d AND action='cancel'", draft.ID), 1)
}

func TestPR722BulkPickingAPICustomerOwnedWholeBagCancellation(t *testing.T) {
	f := newBulkPickingFixtureOwner(t, "kg", 72201)
	draft := f.draft(t, 60000)
	posted := decodeBulkPickingDocument(t, f.post(t, draft.ID, "submit"), "post customer's whole bag")
	if len(posted.Items) != 1 || posted.Items[0].OwnerCustomerID != 72201 {
		t.Fatalf("customer stock owner changed: %+v", posted.Items)
	}
	f.quantities(t, 60000, 60000, 0, 15570)
	assertProductionFlowCount(t, f.pool, f.schema, "work_order_material_reservation_batches", fmt.Sprintf("work_order_id=%d AND warehouse='wip' AND owner_customer_id=72201 AND reserved_g=15570", f.workOrderID), 1)
	assertProductionFlowCount(t, f.pool, f.schema, "material_batches", "id=72230 AND owner_customer_id=72201 AND remaining_g=120000", 1)
	f.auditQuantities(t, draft.ID, "submit", 60000, 0, 15570, 0, 44430, 0)
	decodeBulkPickingDocument(t, f.post(t, draft.ID, "cancel"), "cancel customer's whole bag")
	f.quantities(t, 120000, 0, 15570, 0)
	assertProductionFlowCount(t, f.pool, f.schema, "work_order_material_reservation_batches", fmt.Sprintf("work_order_id=%d AND warehouse='raw_materials' AND owner_customer_id=72201 AND reserved_g=15570", f.workOrderID), 1)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_ledger_entries", fmt.Sprintf("source_doc_type='stock_entry_cancel' AND source_doc_id=%d AND owner_customer_id=72201 AND warehouse='wip' AND qty_change_g=-60000", draft.ID), 1)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_ledger_entries", fmt.Sprintf("source_doc_type='stock_entry_cancel' AND source_doc_id=%d AND owner_customer_id=72201 AND warehouse='raw_materials' AND qty_change_g=60000", draft.ID), 1)
}

func TestPR722BulkPickingAPITypedProductSplitBatchCancellation(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedTypedProductComponentFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.finished_inventory SET onhand_units=120000 WHERE product_id=2; UPDATE %[1]s.stock_batches SET qty_g=120000,qty_units=120000,remaining_g=120000,remaining_units=120000 WHERE batch_code='FP-TYPED-ROASTED';`, schema))
	app := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(app, productionapp.NewService(postgresproduction.NewRepository(pool, schema)), stockapp.NewService(postgresstock.NewRepository(pool, schema)))
	created := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22700}})
	if created.Code != http.StatusOK {
		t.Fatalf("create typed-component plan: %d %s", created.Code, created.Body.String())
	}
	var plan productionapp.ProductionPlanDetail
	if err := json.Unmarshal(created.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,'包装',planned_output_g,'g',15,1,planned_output_g,planned_output_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, plan.ID))
	submitted := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submitted.Code != http.StatusOK {
		t.Fatalf("submit typed-component plan: %d %s", submitted.Code, submitted.Body.String())
	}
	f := bulkPickingFixture{pool: pool, schema: schema, app: app}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1 AND output_product_id=1`, schema), plan.ID).Scan(&f.workOrderID); err != nil {
		t.Fatal(err)
	}
	preview := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/stock-document-preview", f.workOrderID), map[string]any{"action": "issue"})
	var data productionapp.StockDocumentPreview
	if err := json.Unmarshal(preview.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if preview.Code != http.StatusOK || len(data.Document.Items) != 1 {
		t.Fatalf("typed preview: %d %s", preview.Code, preview.Body.String())
	}
	item := data.Document.Items[0]
	if item.ItemType != "finished_product" || item.ProductID != 2 || item.QtyG+item.QtyUnits*item.SpecG != 22700 {
		t.Fatalf("unexpected typed source: %+v", item)
	}
	cmd := map[string]any{"purpose": stockapp.PurposeMaterialTransferForManufacture, "work_order_id": f.workOrderID, "items": []stockapp.StockDocumentItemCommand{{ItemType: item.ItemType, ProductID: item.ProductID, SpecG: item.SpecG, BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID, InventoryUnit: item.InventoryUnit, OwnerCustomerID: item.OwnerCustomerID, FromWarehouse: item.FromWarehouse, ToWarehouse: item.ToWarehouse, BatchCode: item.BatchCode, QtyG: 60000}}}
	draft := decodeBulkPickingDocument(t, serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/stock-documents", cmd), "save typed 60kg issue")
	decodeBulkPickingDocument(t, f.post(t, draft.ID, "submit"), "post typed 60kg issue")
	var sourceID, targetID, reservedG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT source_batch_id,target_batch_id FROM %s.stock_entry_finished_batch_moves WHERE stock_entry_item_id=$1`, schema), draft.Items[0].ID).Scan(&sourceID, &targetID); err != nil {
		t.Fatal(err)
	}
	if sourceID == targetID || targetID <= 0 {
		t.Fatalf("typed transfer must create a distinct WIP batch: %d/%d", sourceID, targetID)
	}
	assertProductionFlowCount(t, pool, schema, "stock_batches", fmt.Sprintf("id=%d AND remaining_g=60000 AND remaining_units=60000", sourceID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches", fmt.Sprintf("id=%d AND remaining_g=60000 AND remaining_units=60000 AND source_doc_type='stock_entry_transfer'", targetID), 1)
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(reserved_g),0) FROM %s.work_order_material_reservation_batches WHERE work_order_id=$1 AND stock_batch_id=$2 AND warehouse='wip'`, schema), f.workOrderID, targetID).Scan(&reservedG); err != nil || reservedG != 22700 {
		t.Fatalf("typed target reservation=%d err=%v", reservedG, err)
	}
	f.auditQuantities(t, draft.ID, "submit", 60000, 60000, 22700, 22700, 37300, 37300)
	decodeBulkPickingDocument(t, f.post(t, draft.ID, "cancel"), "cancel split typed batch")
	assertProductionFlowCount(t, pool, schema, "stock_batches", fmt.Sprintf("id=%d AND remaining_g=120000 AND remaining_units=120000", sourceID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches", fmt.Sprintf("id=%d AND remaining_g=0 AND remaining_units=0", targetID), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf("work_order_id=%d AND stock_batch_id=%d AND warehouse='finished_goods' AND reserved_g=22700", f.workOrderID, sourceID), 1)
	f.auditQuantities(t, draft.ID, "cancel", 60000, 60000, 22700, 22700, 37300, 37300)
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=2 AND spec_g=1 AND warehouse='finished_goods' AND onhand_units=120000 AND onhand_loose_g=0", 1)
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=2 AND spec_g=1 AND warehouse='wip' AND onhand_units=0 AND onhand_loose_g=0", 1)
}

// A real PostgreSQL row-lock barrier makes the formerly inverted order
// deterministic: picking owns the work order and waits for the material, while
// the second API request reaches the work-order lock. The second request must
// not already own a material SHARE lock, or releasing the barrier deadlocks.
func TestPR722BulkPickingAPIPostingLockOrder(t *testing.T) {
	for _, mode := range []string{"submit_consumption", "create_submit_consumption", "create_submit_picking"} {
		t.Run(mode, func(t *testing.T) {
			f := newBulkPickingFixture(t)
			initial := f.draft(t, 60000)
			decodeBulkPickingDocument(t, f.post(t, initial.ID, "submit"), "seed WIP for concurrent consumption")
			picking := f.draft(t, 10000)
			command := f.command(1000)
			if mode != "create_submit_picking" {
				command["purpose"] = stockapp.PurposeMaterialConsumption
				item := command["items"].([]map[string]any)[0]
				item["from_warehouse"] = "wip"
				delete(item, "to_warehouse")
			}
			secondPath := "/api/stock-entries"
			var secondBody []byte
			if mode == "submit_consumption" {
				draft := decodeBulkPickingDocument(t, serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/stock-documents", command), "save consumption draft")
				secondPath = fmt.Sprintf("/api/stock-documents/%d/submit", draft.ID)
			} else {
				var err error
				secondBody, err = json.Marshal(command)
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			barrier, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer barrier.Rollback(context.Background())
			var barrierPID int
			if err = barrier.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&barrierPID); err != nil {
				t.Fatal(err)
			}
			if _, err = barrier.Exec(ctx, fmt.Sprintf("SELECT id FROM %s.materials WHERE id=30 FOR SHARE", f.schema)); err != nil {
				t.Fatal(err)
			}
			request := func(path string, body []byte) <-chan *httptest.ResponseRecorder {
				done := make(chan *httptest.ResponseRecorder, 1)
				go func() {
					req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)).WithContext(ctx)
					if len(body) > 0 {
						req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
					}
					rec := httptest.NewRecorder()
					f.app.ServeHTTP(rec, req)
					done <- rec
				}()
				return done
			}
			firstResult := request(fmt.Sprintf("/api/stock-documents/%d/submit", picking.ID), nil)
			pickingPID := waitBulkPickingBlockedQuery(t, ctx, f.pool, barrierPID, f.schema+".materials")
			secondResult := request(secondPath, secondBody)
			waitBulkPickingBlockedQuery(t, ctx, f.pool, pickingPID, f.schema+".work_orders")
			if err = barrier.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			for _, result := range []<-chan *httptest.ResponseRecorder{firstResult, secondResult} {
				select {
				case r := <-result:
					decodeBulkPickingDocument(t, r, "concurrent stock API must serialize without a deadlock")
				case <-ctx.Done():
					t.Fatal("concurrent stock API did not complete after barrier release")
				}
			}
			if mode == "create_submit_picking" {
				f.quantities(t, 49000, 71000, 0, 15570)
			} else {
				f.quantities(t, 50000, 69000, 0, 14570)
			}
		})
	}
}

func waitBulkPickingBlockedQuery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, blockerPID int, queryFragment string) int {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var pid int
		err := pool.QueryRow(ctx, `SELECT COALESCE(MIN(pid),0) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND $1=ANY(pg_blocking_pids(pid)) AND position($2 in query)>0`, blockerPID, queryFragment).Scan(&pid)
		if err != nil {
			t.Fatal(err)
		}
		if pid > 0 {
			return pid
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("API did not reach controlled lock barrier: blocker=%d query=%s", blockerPID, queryFragment)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
