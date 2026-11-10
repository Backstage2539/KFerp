package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
	"testing"
)

func TestTransferredUnknownCostProductFlowsIntoNextWorkOrder(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 99230)
	frozen := `{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET process_snapshot_json='%[2]s'::jsonb,operation_material_bindings='{"product:90001:0:1000":{"node_key":"pack","source_warehouse":"finished_shop"}}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=%[3]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%[3]d;
 INSERT INTO %[1]s.warehouses(code,name,kind,active) VALUES('finished_shop','成品二仓','finished',true) ON CONFLICT(code) DO UPDATE SET active=true;
 INSERT INTO %[1]s.products(id,name) VALUES(90001,'半成品商品');
 INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(90001,1000,'finished_goods',30,0);
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,qty_g,qty_units,remaining_g,remaining_units,quality_status,unit_cost,cost_unavailable) VALUES(90001,'CHAIN-UNKNOWN','finished_product',90001,'半成品商品',1000,30000,30,30000,30,'pass',50,true);`, schema, frozen, wo))
	svc := stockapp.NewService(stockpg.NewRepository(pool, schema))
	transfer, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Operator: "test", Items: []stockapp.StockDocumentItemCommand{{ProductID: 90001, ItemType: "finished_product", SpecG: 1000, QtyUnits: 23, BatchCode: "CHAIN-UNKNOWN", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop"}}})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if len(transfer.Items) != 1 || len(transfer.Items[0].TransferBatchMoves) != 1 {
		t.Fatalf("transfer %+v", transfer)
	}
	target := transfer.Items[0].TransferBatchMoves[0].TargetBatchCode
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_order_material_reservations SET component_type='product',material_id=0,component_id=90001,component_spec_g=1000,material_name='半成品商品',source_warehouse='finished_shop',reserved_g=22700 WHERE work_order_id=%[2]d AND material_id=10;
 DELETE FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=%[2]d AND material_id=10;
 INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,component_spec_g,stock_batch_id,batch_code,warehouse,reserved_g) SELECT r.id,r.work_order_id,0,'product',90001,1000,b.id,b.batch_code,'finished_shop',22700 FROM %[1]s.work_order_material_reservations r CROSS JOIN %[1]s.stock_batches b WHERE r.work_order_id=%[2]d AND r.component_id=90001 AND b.batch_code='%[3]s';`, schema, wo, target))
	var card int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	rec := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var preview app.OperationCompletionPreview
	if err = json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	rec = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": preview.InputQty, "actual_output_qty": 100, "input_unit": preview.InputUnit, "output_unit": preview.OutputUnit, "expected_version": preview.Version, "request_id": "chain-unknown", "materials": preview.Materials, "actual_minutes": 10})
	if rec.Code != 200 {
		t.Fatalf("complete %d %s", rec.Code, rec.Body.String())
	}
	var unavailable bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT cost_unavailable FROM %[1]s.stock_batches WHERE source_doc_id=(SELECT id FROM %[1]s.production_operation_events WHERE request_id='chain-unknown') AND batch_code LIKE 'FP-OP-%%'`, schema)).Scan(&unavailable); err != nil || !unavailable {
		t.Fatalf("downstream unavailable=%v err=%v", unavailable, err)
	}
}

func TestLegacyFinishedReceiptInheritsUnknownConsumedMaterialCost(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, run := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 99330)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET cost_unavailable=true WHERE material_id=10`, schema))
	rec := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/work-orders/%d/complete", wo), map[string]any{"finished_units": 100, "consumed_input_g": 22700, "warehouse": "finished_goods", "note": "隔离验证"})
	if rec.Code != 200 {
		t.Fatalf("finish %d %s", rec.Code, rec.Body.String())
	}
	var unknown bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT cost_unavailable FROM %s.stock_batches WHERE source_doc_type='production_run' AND source_doc_id=$1`, schema), run).Scan(&unknown); err != nil || !unknown {
		t.Fatalf("output unknown=%v err=%v", unknown, err)
	}
}
