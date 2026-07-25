package stock

import (
	"context"
	"fmt"
	"strings"
	"testing"

	stockapp "orderapp/internal/application/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEnsureUnifiedStockDocumentTablesAddsColumnsBeforeDependentIndex(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %s.stock_entries (
			id BIGSERIAL PRIMARY KEY,entry_no TEXT NOT NULL UNIQUE,entry_type TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'submitted',work_order_id BIGINT NOT NULL DEFAULT 0,
			job_card_id BIGINT NOT NULL DEFAULT 0,running_item_id BIGINT NOT NULL DEFAULT 0,
			source_type TEXT NOT NULL DEFAULT '',source_id BIGINT NOT NULL DEFAULT 0,
			operator TEXT NOT NULL DEFAULT '',note TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %s.stock_entry_items (
			id BIGSERIAL PRIMARY KEY,stock_entry_id BIGINT NOT NULL,material_id BIGINT NOT NULL DEFAULT 0,
			product_id BIGINT NOT NULL DEFAULT 0,item_type TEXT NOT NULL DEFAULT '',item_name TEXT NOT NULL DEFAULT '',
			spec_g BIGINT NOT NULL DEFAULT 0,from_warehouse TEXT NOT NULL DEFAULT '',to_warehouse TEXT NOT NULL DEFAULT '',
			qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,batch_code TEXT NOT NULL DEFAULT '',
			unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,total_cost NUMERIC(12,4) NOT NULL DEFAULT 0
		);
		INSERT INTO %s.stock_entries(entry_no,entry_type) VALUES('SE-LEGACY-1','wip_return');
	`, schema, schema, schema))
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})

	if err := ensureUnifiedStockDocumentTables(ctx, pool, schema); err != nil {
		t.Fatalf("ensureUnifiedStockDocumentTables on existing Stock Entry tables: %v", err)
	}
	var purpose string
	var isReturn, legacy bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT purpose,is_return,legacy
		FROM %s.stock_entries
		WHERE entry_no='SE-LEGACY-1'
	`, schema)).Scan(&purpose, &isReturn, &legacy); err != nil {
		t.Fatal(err)
	}
	var indexCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname=$1 AND tablename='stock_entries' AND indexname='stock_entries_idempotency_uq'
	`, schema).Scan(&indexCount); err != nil {
		t.Fatal(err)
	}
	if purpose != stockapp.PurposeMaterialTransferForManufacture || !isReturn || !legacy || indexCount != 1 {
		t.Fatalf("legacy purpose/return/legacy/index = %q/%t/%t/%d", purpose, isReturn, legacy, indexCount)
	}
}

func setupUnifiedStockDocumentTest(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY, code TEXT NOT NULL, name TEXT NOT NULL, kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g', purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0, onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0, min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (id BIGINT PRIMARY KEY,name TEXT NOT NULL);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id,spec_g)
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT,action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,
	meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.work_orders (
	id BIGINT PRIMARY KEY,work_order_no TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'running',
	product_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL DEFAULT 0,completed_at TIMESTAMPTZ
);
CREATE TABLE %s.work_order_material_reservations (
	id BIGSERIAL PRIMARY KEY,work_order_id BIGINT NOT NULL,material_id BIGINT NOT NULL,
	reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,
	consumed_g BIGINT NOT NULL DEFAULT 0,consumed_units BIGINT NOT NULL DEFAULT 0,
	returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.job_cards (
	id BIGINT PRIMARY KEY,work_order_id BIGINT NOT NULL,status TEXT NOT NULL DEFAULT 'pending'
);
INSERT INTO %s.materials(id,code,name,unit) VALUES(1,'BEAN-1','水洗豆','g');
INSERT INTO %s.products(id,name) VALUES(9,'测试熟豆');
INSERT INTO %s.work_orders(id,work_order_no,status,product_id,spec_g) VALUES(88,'WO-0000000088','running',9,1000);
INSERT INTO %s.work_order_material_reservations(work_order_id,material_id,reserved_g) VALUES(88,1,6000);
INSERT INTO %s.job_cards(id,work_order_id,status) VALUES(91,88,'completed');
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	return pool, schema
}

func TestUnifiedStockDocumentDraftSubmitIdempotencyAndCancel(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))

	draft, err := svc.CreateStockDocumentDraft(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "jj", IdempotencyKey: "receipt-1",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", ToWarehouse: "raw_materials", QtyG: 10000, UnitCost: 42.5,
			Supplier: "测试供应商", CropSeason: "2025/26", Origin: "云南", ProducerFlavorDescription: "红糖",
		}},
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	var onhandG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&onhandG); err != nil {
		t.Fatal(err)
	}
	if draft.Status != "draft" || onhandG != 0 {
		t.Fatalf("draft status/onhand = %s/%d, want draft/0", draft.Status, onhandG)
	}
	submitted, err := svc.SubmitStockDocument(ctx, draft.ID, "jj")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	again, err := svc.SubmitStockDocument(ctx, draft.ID, "jj")
	if err != nil {
		t.Fatalf("idempotent submit: %v", err)
	}
	var ledgerCount, auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.stock_ledger_entries WHERE source_doc_type='stock_entry' AND source_doc_id=$1`, schema), draft.ID).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='stock_entry' AND entity_id=$1`, schema), draft.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if submitted.Status != "submitted" || again.ID != submitted.ID || ledgerCount != 1 || auditCount < 2 {
		t.Fatalf("submit = %+v again=%+v ledger/audit=%d/%d", submitted, again, ledgerCount, auditCount)
	}
	cancelled, err := svc.CancelStockDocument(ctx, draft.ID, "jj")
	if err != nil {
		t.Fatalf("cancel receipt: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&onhandG); err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" || onhandG != 0 {
		t.Fatalf("cancelled status/onhand = %s/%d, want cancelled/0", cancelled.Status, onhandG)
	}
}

func TestUnifiedStockDocumentListIncludesLegacyParallelDocumentsReadOnly(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.material_receipts(material_id,qty_g,unit_cost,status,operator,note)
		VALUES(1,1000,42,'submitted','old','历史原料入库');
		INSERT INTO %s.material_transfers(transfer_no,material_id,material_name,from_warehouse,to_warehouse,qty_g,status,operator,note)
		VALUES('MT-OLD-1',1,'水洗豆','raw_materials','wip',500,'submitted','old','历史 WIP 转仓');
		INSERT INTO %s.finished_product_transfers(transfer_no,product_id,product_name,spec_g,from_warehouse,to_warehouse,qty_g,qty_units,status,operator,note)
		VALUES('FT-OLD-1',9,'测试熟豆',1000,'finished_goods','sales',1000,1,'submitted','old','历史成品转仓');
	`, schema, schema, schema))

	svc := stockapp.NewService(NewRepository(pool, schema))
	result, err := svc.ListStockDocuments(ctx, stockapp.StockDocumentQuery{Limit: 20})
	if err != nil {
		t.Fatalf("list stock documents: %v", err)
	}
	if result.Total != 3 || len(result.Rows) != 3 {
		t.Fatalf("legacy list total/rows = %d/%d, want 3/3: %+v", result.Total, len(result.Rows), result.Rows)
	}
	seen := map[string]bool{}
	for _, row := range result.Rows {
		if !row.Legacy || row.ID >= 0 {
			t.Fatalf("legacy row must be read-only and use a non-document id: %+v", row)
		}
		seen[row.SourceType] = true
	}
	for _, sourceType := range []string{"material_receipt", "material_transfer", "finished_product_transfer"} {
		if !seen[sourceType] {
			t.Fatalf("legacy list missing source type %q: %+v", sourceType, result.Rows)
		}
	}
}

func TestUnifiedStockDocumentLegacyWriteServicesDoNotCreateParallelDocuments(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))

	receipt, err := svc.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 1, QtyG: 2000, UnitCost: 42, Operator: "legacy-api",
	})
	if err != nil {
		t.Fatalf("legacy receipt: %v", err)
	}
	transfer, err := svc.TransferMaterial(ctx, stockapp.MaterialTransferCommand{
		MaterialID: 1, FromWarehouse: "raw_materials", ToWarehouse: "wip", QtyG: 1000, Operator: "legacy-api",
	})
	if err != nil {
		t.Fatalf("legacy material transfer: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(9,1000,'finished_goods',2,0)
	`, schema))
	finished, err := svc.TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID: 9, SpecG: 1000, FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
		QtyUnits: 1, Operator: "legacy-api",
	})
	if err != nil {
		t.Fatalf("legacy finished transfer: %v", err)
	}
	for name, entryNo := range map[string]string{
		"receipt": receipt.EntryNo, "material transfer": transfer.EntryNo, "finished transfer": finished.EntryNo,
	} {
		if !strings.HasPrefix(entryNo, "SE-") {
			t.Fatalf("%s number = %q, want unified SE number", name, entryNo)
		}
	}
	var stockEntryCount, parallelCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.stock_entries WHERE legacy=false`, schema)).Scan(&stockEntryCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			(SELECT count(*) FROM %s.material_receipts)
			+ (SELECT count(*) FROM %s.material_transfers)
			+ (SELECT count(*) FROM %s.finished_product_transfers)
	`, schema, schema, schema)).Scan(&parallelCount); err != nil {
		t.Fatal(err)
	}
	if stockEntryCount != 3 || parallelCount != 0 {
		t.Fatalf("unified/parallel counts = %d/%d, want 3/0", stockEntryCount, parallelCount)
	}
}

func TestUnifiedStockDocumentManufacturingFIFOAndRollback(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	receipt, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 10000, UnitCost: 40}},
	})
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	issue, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 6000}},
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if len(issue.Items) != 1 || len(issue.Items[0].Allocations) != 1 || issue.Items[0].Allocations[0].BatchCode != receipt.Items[0].BatchCode {
		t.Fatalf("FIFO allocations = %+v receipt=%+v", issue.Items, receipt.Items)
	}
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, IsReturn: true, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 1000}},
	}); err != nil {
		t.Fatalf("return: %v", err)
	}
	consume, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 4000}},
	})
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, IsReturn: true, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 2000}},
	}); err == nil || !strings.Contains(err.Error(), "insufficient") {
		t.Fatalf("over-return error = %v, want insufficient", err)
	}
	if _, err := svc.CancelStockDocument(ctx, consume.ID, "jj"); err != nil {
		t.Fatalf("cancel consumption: %v", err)
	}
	var rawG, wipG, onhandG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE material_id=1 AND warehouse='raw_materials'`, schema)).Scan(&rawG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE material_id=1 AND warehouse='wip'`, schema)).Scan(&wipG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&onhandG); err != nil {
		t.Fatal(err)
	}
	if rawG != 5000 || wipG != 5000 || onhandG != 10000 {
		t.Fatalf("raw/wip/onhand = %d/%d/%d, want 5000/5000/10000", rawG, wipG, onhandG)
	}
}

func TestUnifiedStockDocumentRejectsFrozenBatchAndCrossWorkOrderReturn(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	receipt, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 6000, UnitCost: 40}},
	})
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	batchID := receipt.Items[0].Allocations[0].MaterialBatchID
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET quality_status='hold' WHERE id=%d`, schema, batchID))
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", QtyG: 1000, BatchCode: receipt.Items[0].BatchCode,
		}},
	}); err == nil || !strings.Contains(err.Error(), "quality") {
		t.Fatalf("frozen batch error = %v, want quality block", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.material_batches SET quality_status='pass' WHERE id=%d;
		INSERT INTO %s.work_orders(id,work_order_no,status,product_id,spec_g) VALUES(89,'WO-0000000089','running',9,1000);
		INSERT INTO %s.work_order_material_reservations(work_order_id,material_id,reserved_g) VALUES(89,1,6000);
		INSERT INTO %s.job_cards(id,work_order_id,status) VALUES(92,89,'completed');
	`, schema, batchID, schema, schema, schema))
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 6000}},
	}); err != nil {
		t.Fatalf("issue to work order 88: %v", err)
	}
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, IsReturn: true, WorkOrderID: 89, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 1000}},
	}); err == nil || !strings.Contains(err.Error(), "insufficient") {
		t.Fatalf("cross-work-order return error = %v, want insufficient", err)
	}
}

func TestUnifiedStockDocumentManufactureRequiresClosedMaterialAndReversesFinishedInventory(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 6000, UnitCost: 40}},
	}); err != nil {
		t.Fatalf("receipt: %v", err)
	}
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 6000}},
	}); err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeManufacture, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 5}},
	}); err == nil || !strings.Contains(err.Error(), "unconsumed") {
		t.Fatalf("premature manufacture error = %v, want unconsumed material", err)
	}
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 6000}},
	}); err != nil {
		t.Fatalf("consume: %v", err)
	}
	manufacture, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeManufacture, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 5}},
	})
	if err != nil {
		t.Fatalf("manufacture: %v", err)
	}
	var units int64
	var workOrderStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=9 AND spec_g=1000 AND warehouse='finished_goods'`, schema)).Scan(&units); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.work_orders WHERE id=88`, schema)).Scan(&workOrderStatus); err != nil {
		t.Fatal(err)
	}
	if units != 5 || workOrderStatus != "completed" {
		t.Fatalf("manufacture units/status = %d/%s, want 5/completed", units, workOrderStatus)
	}
	if _, err := svc.CancelStockDocument(ctx, manufacture.ID, "jj"); err != nil {
		t.Fatalf("cancel manufacture: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=9 AND spec_g=1000 AND warehouse='finished_goods'`, schema)).Scan(&units); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.work_orders WHERE id=88`, schema)).Scan(&workOrderStatus); err != nil {
		t.Fatal(err)
	}
	if units != 0 || workOrderStatus != "running" {
		t.Fatalf("cancel manufacture units/status = %d/%s, want 0/running", units, workOrderStatus)
	}
}
