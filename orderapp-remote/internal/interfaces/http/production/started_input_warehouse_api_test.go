package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"testing"
)

func TestBOMStartUsesIssuedLocationInsteadOfFrozenSource(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, wo := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74820, "bom_start_v1")
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET onhand_g=22700 WHERE id=10;UPDATE %[1]s.work_orders SET operation_material_bindings='{"material:10:0:0":{"source_warehouse":"raw_materials"}}' WHERE id=%[2]d;UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_id=10;UPDATE %[1]s.work_order_material_reservation_batches SET warehouse='raw_materials' WHERE work_order_id=%[2]d AND material_id=10;UPDATE %[1]s.work_order_material_reservations SET source_warehouse='raw_materials' WHERE work_order_id=%[2]d AND material_id=10`, schema, wo))
	var code string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT batch_code FROM %s.material_batches WHERE material_id=10 LIMIT 1`, schema)).Scan(&code); err != nil {
		t.Fatal(err)
	}
	_, err := stockapp.NewService(stockrepo.NewRepository(pool, schema)).CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: wo, Operator: "PR747验收", Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 10, InventoryUnit: "kg", QtyG: 22700, BatchCode: code, FromWarehouse: "raw_materials", ToWarehouse: "wip"}}})
	if err != nil {
		t.Fatal(err)
	}
	var card int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	r := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	var p app.OperationCompletionPreview
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if err = json.Unmarshal(r.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	for _, m := range p.Materials {
		if m.MaterialID == 10 && m.Warehouse != "wip" {
			t.Fatalf("issued material must consume WIP, got %s", m.Warehouse)
		}
	}
	r = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "issued-start"})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "stock_entry_items", fmt.Sprintf("material_id=10 AND from_warehouse='wip' AND stock_entry_id IN(SELECT id FROM %s.stock_entries WHERE source_type='operation_start')", schema), 1)
	r = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	if err = json.Unmarshal(r.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	r = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start-reversal", card), map[string]any{"expected_version": p.Version, "request_id": "issued-reverse", "reason": "尚未投入加工", "unprocessed_confirmed": true})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	r = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	if err = json.Unmarshal(r.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	for _, m := range p.Materials {
		if m.MaterialID == 10 && m.Warehouse != "wip" {
			t.Fatalf("returned issued batch must remain in WIP: %s", m.Warehouse)
		}
	}
}
func TestBOMStartRollbackModePausesNewAndRetainsLegacy(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, wo := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74821, "bom_start_v1")
	t.Setenv("KFERP_BOM_START_READ_ONLY", "1")
	res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/work-orders/%d/start", wo), nil)
	if res.Code == 200 || !strings.Contains(res.Body.String(), "暂停写入") {
		t.Fatalf("rollback accepted new-policy work order: %s", res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='released' AND running_item_id=0", wo), 1)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='legacy' WHERE id=$1`, schema), wo); err != nil {
		t.Fatal(err)
	}
	res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/work-orders/%d/start", wo), nil)
	if res.Code != 200 {
		t.Fatalf("rollback mode prevented legacy work order: %s", res.Body.String())
	}
}
