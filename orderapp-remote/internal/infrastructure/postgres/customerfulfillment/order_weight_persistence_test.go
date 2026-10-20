package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerfulfillment"
	"strings"
	"testing"
)

func seedPR706FrozenWeightMaterials(t *testing.T, pool *pgxpool.Pool, schema string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_version_variants SET logistics_weight_g=250,material_loss_rate=20 WHERE id=7032;
 INSERT INTO %[1]s.materials(id,code,name,unit,cost_unit)VALUES(70601,'PR706-BEAN','PR706熟豆','kg','kg'),(70602,'PR706-BAG','PR706袋','个','个');
 INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct,material_loss_rate)VALUES(7030,7032,70601,'material','fixed_qty',.227,0,20),(7030,7032,70602,'material','fixed_qty',1,0,0);`, schema)); err != nil {
		t.Fatal(err)
	}
}
func assertPR706FrozenWeights(t *testing.T, pool *pgxpool.Pool, schema string, orderID int64, qty float64) {
	t.Helper()
	var materialUnit, materialItem, logisticsUnit, logisticsItem float64
	var snapshot string
	err := pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT material_unit_weight_g::float8,material_item_weight_g::float8,logistics_unit_weight_g::float8,logistics_item_weight_g::float8,weight_snapshot_json::text FROM %s.order_items WHERE order_id=$1`, schema), orderID).Scan(&materialUnit, &materialItem, &logisticsUnit, &logisticsItem, &snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if materialUnit != 227 || materialItem != 227*qty || logisticsUnit != 250 || logisticsItem != 250*qty || !strings.Contains(snapshot, `"rule_version": 1`) {
		t.Fatalf("frozen weights %v/%v and %v/%v snapshot=%s", materialUnit, materialItem, logisticsUnit, logisticsItem, snapshot)
	}
}
func TestPR706ExplicitStockOrderFreezesBothWeights(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	seedPR706FrozenWeightMaterials(t, pool, schema)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	order, err := repo.submitExplicitDirectShipOrderTx(ctx, tx, app.SubmitCustomerDirectShipOrderCommand{CustomerID: 501, FulfillmentMode: "stock", OrderDate: "2026-10-20", Items: []app.SubmitCustomerDirectShipOrderItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, ProductName: "PR706库存代发", InventoryUnit: "袋", QuantityUnits: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	assertPR706FrozenWeights(t, pool, schema, order.OrderID, 4)
}
func TestPR706ImportedOrderFreezesExplicitSpecWeights(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	seedPR706FrozenWeightMaterials(t, pool, schema)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET name='PR706导入商品',customer_id=0 WHERE id=910`, schema)); err != nil {
		t.Fatal(err)
	}
	batch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{CustomerID: 501, ImportType: app.ImportTypeDirectShipWorkbook, SourceFilename: "PR706.xlsx", SourceSHA256: strings.Repeat("7", 64), CreatedBy: "test", Parsed: app.ParsedWorkbook{ImportType: app.ImportTypeDirectShipWorkbook, Rows: []app.ParsedRow{
		{SheetName: "代发", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:PR706-IMPORT", Payload: map[string]any{"order_date": "2026-10-20", "order_no": "PR706-IMPORT", "receiver_address": "张三 13800000000 上海测试路"}},
		{SheetName: "代发", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:PR706-IMPORT:2", Payload: map[string]any{"order_no": "PR706-IMPORT", "product_title": "PR706导入商品", "spec": "袋", "quantity_units": int64(4), "bom_spec_id": int64(7031), "bom_variant_id": int64(7032)}}}, Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, DirectShipOrders: 1, DirectShipItems: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: batch.ID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	var orderID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT order_id FROM %s.customer_direct_ship_import_orders WHERE customer_id=501 AND external_order_no='PR706-IMPORT'`, schema)).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	assertPR706FrozenWeights(t, pool, schema, orderID, 4)
	// A quantity-only reimport after publication preserves the original validated identity/weight.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7033,7030,'V2','published',1,'袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g)VALUES(7034,7033,7031,'新版袋','袋',true,999);
 UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE bom_id=7030;`, schema)); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = repo.applyDirectShipItemTx(ctx, tx, 501, batch.ID, newDirectShipApplyState(), importRowRecord{Payload: map[string]any{"order_no": "PR706-IMPORT", "product_title": "PR706导入商品", "spec": "袋", "quantity_units": int64(5)}})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	assertPR706FrozenWeights(t, pool, schema, orderID, 5)

}

func TestPR706ImportedUnknownSpecDoesNotUseDefaultWeight(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	seedPR706FrozenWeightMaterials(t, pool, schema)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.products SET name='PR706导入商品',customer_id=0 WHERE id=910`, schema)); err != nil {
		t.Fatal(err)
	}
	batch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{CustomerID: 501, ImportType: app.ImportTypeDirectShipWorkbook, SourceFilename: "PR706.xlsx", SourceSHA256: strings.Repeat("7", 64), CreatedBy: "test", Parsed: app.ParsedWorkbook{ImportType: app.ImportTypeDirectShipWorkbook, Rows: []app.ParsedRow{
		{SheetName: "代发", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:PR706-IMPORT", Payload: map[string]any{"order_date": "2026-10-20", "order_no": "PR706-IMPORT", "receiver_address": "张三 13800000000 上海测试路"}},
		{SheetName: "代发", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:PR706-IMPORT:2", Payload: map[string]any{"order_no": "PR706-IMPORT", "product_title": "PR706导入商品", "spec": "袋", "quantity_units": int64(4)}}}, Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, DirectShipOrders: 1, DirectShipItems: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: batch.ID, Actor: "test"}); err != nil {
		t.Fatal(err)
	}
	var orderID int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT order_id FROM %s.customer_direct_ship_import_orders WHERE customer_id=501 AND external_order_no='PR706-IMPORT'`, schema)).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	var material, logistics *string
	var snapshot string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g::text,logistics_item_weight_g::text,weight_snapshot_json::text FROM %s.order_items WHERE order_id=$1`, schema), orderID).Scan(&material, &logistics, &snapshot); err != nil {
		t.Fatal(err)
	}
	if material != nil || logistics != nil || !strings.Contains(snapshot, `"reason": "unresolved_import_spec"`) {
		t.Fatalf("ambiguous import guessed default weights material%v logistics%v snapshot%s", material, logistics, snapshot)
	}
}

func assertPR706RequestWeightDTO(t *testing.T, request app.MiniDirectShipRequest) {
	t.Helper()
	if len(request.Items) != 1 {
		t.Fatalf("request items %+v", request.Items)
	}
	raw, err := json.Marshal(request.Items[0])
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["material_item_weight_g"] != "2724.000000" || got["logistics_item_weight_g"] != "3000.000000" {
		t.Fatalf("saved request DTO missing frozen weights: %s", raw)
	}
}

func TestPR706MiniRequestAggregatesDifferentFrozenUnitWeights(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	seedPR706FrozenWeightMaterials(t, pool, schema)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	cmd := app.SubmitCustomerDirectShipOrderCommand{CustomerID: 501, FulfillmentMode: "stock", OrderDate: "2026-10-20", Items: []app.SubmitCustomerDirectShipOrderItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, InventoryUnit: "袋", QuantityUnits: 2}}}
	first, err := repo.submitExplicitDirectShipOrderTx(ctx, tx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=300 WHERE id=7032`, schema)); err != nil {
		t.Fatal(err)
	}
	second, err := repo.submitExplicitDirectShipOrderTx(ctx, tx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	var requestID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_requests(customer_id)VALUES(501)RETURNING id`, schema)).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	for idx, id := range []int64{first.OrderID, second.OrderID} {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_request_orders(request_id,order_id,warehouse_code)VALUES($1,$2,$3)`, schema), requestID, id, fmt.Sprint(idx)); err != nil {
			t.Fatal(err)
		}
	}
	item := app.MiniDirectShipItemCommand{ProductID: 910, BomSpecID: 7031, Qty: 4}
	if err = loadMiniRequestItemWeights(ctx, tx, schema, requestID, &item); err != nil {
		t.Fatal(err)
	}
	if item.LogisticsUnitWeightG != nil || item.MaterialUnitWeightG == nil || *item.MaterialUnitWeightG != "227.000000" || item.MaterialItemWeightG == nil || *item.MaterialItemWeightG != "908.000000" || item.LogisticsItemWeightG == nil || *item.LogisticsItemWeightG != "1100.000000" || !strings.Contains(item.WeightSnapshotJSON, `"mixed_unit_weight": true`) {
		t.Fatalf("mixed source aggregation %+v", item)
	}
}

func TestPR706StartupRepairPreservesFrozenWeightsAndItemIdentity(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	seedPR706FrozenWeightMaterials(t, pool, schema)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	order, err := repo.submitExplicitDirectShipOrderTx(ctx, tx, app.SubmitCustomerDirectShipOrderCommand{CustomerID: 501, FulfillmentMode: "stock", OrderDate: "2026-10-20", Items: []app.SubmitCustomerDirectShipOrderItem{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, InventoryUnit: "袋", QuantityUnits: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_direct_ship_import_orders(batch_id,customer_id,external_order_no,order_date,order_id)VALUES(0,501,'PR706-STARTUP','2026-10-20',$1)`, schema), order.OrderID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var itemID int64
	var snapshot string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,weight_snapshot_json::text FROM %s.order_items WHERE order_id=$1`, schema), order.OrderID).Scan(&itemID, &snapshot); err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 2; iteration++ {
		if err = EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
		assertPR706FrozenWeights(t, pool, schema, order.OrderID, 4)
		var currentID int64
		var currentSnapshot string
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,weight_snapshot_json::text FROM %s.order_items WHERE order_id=$1`, schema), order.OrderID).Scan(&currentID, &currentSnapshot); err != nil {
			t.Fatal(err)
		}
		if currentID != itemID || currentSnapshot != snapshot {
			t.Fatalf("startup rebuilt frozen line id%d=>%d snapshot%s=>%s", itemID, currentID, snapshot, currentSnapshot)
		}
	}
}
