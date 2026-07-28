package stock

import (
	"context"
	"fmt"
	"strings"
	"testing"

	stockapp "orderapp/internal/application/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStockFrozenMaterialRequirementsPreserveWeightLossAndCountBasis(t *testing.T) {
	requirements, err := stockFrozenMaterialRequirements(`[
		{"material_id":1,"unit":"g","consume_unit":"ratio_pct","ratio_pct":100,"material_loss_rate":0.12},
		{"material_id":2,"unit":"个","source":"packaging","consume_unit":"unit_per_bag","qty_per_unit":1}
	]`, 6356, 6356, 14)
	if err != nil {
		t.Fatal(err)
	}
	if requirements[1].RequiredG != 7223 || requirements[1].RequiredUnits != 0 {
		t.Fatalf("weight requirement = %+v, want 7223g", requirements[1])
	}
	if requirements[2].RequiredUnits != 14 || requirements[2].RequiredG != 0 {
		t.Fatalf("count requirement = %+v, want 14 units", requirements[2])
	}
}

func TestStockFrozenMaterialRequirementsHonorConsumeUnitConversion(t *testing.T) {
	requirements, err := stockFrozenMaterialRequirements(`[
		{"material_id":1,"unit":"g","consume_unit":"kg","qty_per_unit":1,"output_qty":1,"output_unit":"kg"},
		{"material_id":2,"unit":"kg","consume_unit":"g","qty_per_unit":100,"output_qty":1,"output_unit":"kg"},
		{"material_id":3,"unit":"g","consume_unit":"g_per_bag","qty_per_unit":12,"output_qty":1,"output_unit":"kg"}
	]`, 1000, 1000, 4)
	if err != nil {
		t.Fatal(err)
	}
	if requirements[1].RequiredG != 1000 {
		t.Fatalf("1kg consumed into g inventory = %+v, want 1000g", requirements[1])
	}
	if requirements[2].RequiredG != 100 {
		t.Fatalf("100g consumed into kg inventory = %+v, want 100g", requirements[2])
	}
	if requirements[3].RequiredG != 48 {
		t.Fatalf("12g per bag x 4 = %+v, want 48g", requirements[3])
	}
}

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
	running_item_id BIGINT NOT NULL DEFAULT 0,
	product_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL DEFAULT 0,
	planned_g BIGINT NOT NULL DEFAULT 0,planned_output_g BIGINT NOT NULL DEFAULT 0,
	sales_spec_count NUMERIC(18,6) NOT NULL DEFAULT 0,
	material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,completed_at TIMESTAMPTZ
);
CREATE TABLE %s.work_order_material_reservations (
	id BIGSERIAL PRIMARY KEY,work_order_id BIGINT NOT NULL,material_id BIGINT NOT NULL,
	reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,
	consumed_g BIGINT NOT NULL DEFAULT 0,consumed_units BIGINT NOT NULL DEFAULT 0,
	returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved',
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

func TestProductionIssueReportsChineseRawMaterialFIFOShortageWithoutPartialPosting(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.materials(id,code,name,unit) VALUES(2,'BEAN-2','黄波旁水洗','g');
		INSERT INTO %s.work_order_material_reservations(work_order_id,material_id,reserved_g) VALUES(88,2,6000);
	`, schema, schema))
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{
			{
				MaterialID: 1, ItemType: "material", InventoryUnit: "g",
				ToWarehouse: "raw_materials", QtyG: 1000, UnitCost: 40,
			},
			{
				MaterialID: 2, ItemType: "material", InventoryUnit: "g",
				ToWarehouse: "raw_materials", QtyG: 500, UnitCost: 50,
			},
		},
	}); err != nil {
		t.Fatalf("receipt: %v", err)
	}
	frozenReceipt, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 2, ItemType: "material", InventoryUnit: "g",
			ToWarehouse: "raw_materials", QtyG: 1, UnitCost: 50,
		}},
	})
	if err != nil {
		t.Fatalf("frozen receipt: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.material_batches SET quality_status='hold' WHERE batch_code='%s';
	`, schema, frozenReceipt.Items[0].BatchCode))
	_, err = svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{
			{
				MaterialID: 1, ItemType: "material", ItemName: "水洗豆", InventoryUnit: "g",
				FromWarehouse: "raw_materials", ToWarehouse: "wip", QtyG: 500,
			},
			{
				MaterialID: 2, ItemType: "material", ItemName: "黄波旁水洗", InventoryUnit: "g",
				FromWarehouse: "raw_materials", ToWarehouse: "wip", QtyG: 1974,
			},
		},
	})
	if err == nil ||
		!strings.Contains(err.Error(), "原料仓库存不足") ||
		!strings.Contains(err.Error(), "黄波旁水洗") ||
		!strings.Contains(err.Error(), "需领用1974g") ||
		!strings.Contains(err.Error(), "可用500g") ||
		!strings.Contains(err.Error(), "缺口1474g") ||
		!strings.Contains(err.Error(), "另有质检冻结库存1g，解除后仍不足") {
		t.Fatalf("issue error = %v, want Chinese FIFO shortage detail", err)
	}
	var firstRawG, firstWIPG, secondRawG, secondWIPG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(SUM(qty_g) FILTER (WHERE material_id=1 AND warehouse='raw_materials'),0)::bigint,
			COALESCE(SUM(qty_g) FILTER (WHERE material_id=1 AND warehouse='wip'),0)::bigint,
			COALESCE(SUM(qty_g) FILTER (WHERE material_id=2 AND warehouse='raw_materials'),0)::bigint,
			COALESCE(SUM(qty_g) FILTER (WHERE material_id=2 AND warehouse='wip'),0)::bigint
		FROM %s.material_batch_locations
		WHERE material_id IN (1,2)
	`, schema)).Scan(&firstRawG, &firstWIPG, &secondRawG, &secondWIPG); err != nil {
		t.Fatal(err)
	}
	if firstRawG != 1000 || firstWIPG != 0 || secondRawG != 501 || secondWIPG != 0 {
		t.Fatalf(
			"raw/wip after failed multi-material issue = %d/%d and %d/%d, want 1000/0 and 501/0",
			firstRawG, firstWIPG, secondRawG, secondWIPG,
		)
	}
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

func TestUnifiedStockDocumentListAndDetailExposeWorkOrderNumberAndCountTotal(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))

	draft, err := svc.CreateStockDocumentDraft(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "个", QtyUnits: 7,
		}},
	})
	if err != nil {
		t.Fatalf("create count draft: %v", err)
	}
	result, err := svc.ListStockDocuments(ctx, stockapp.StockDocumentQuery{WorkOrderID: 88, Limit: 20})
	if err != nil {
		t.Fatalf("list stock documents: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("list rows = %+v, want one work-order document", result.Rows)
	}
	row := result.Rows[0]
	if row.ID != draft.ID || row.WorkOrderNo != "WO-0000000088" || row.TotalQtyG != 0 || row.TotalQtyUnits != 7 {
		t.Fatalf("list row = %+v, want work order number and 7 count units", row)
	}
	detail, err := svc.GetStockDocument(ctx, draft.ID)
	if err != nil {
		t.Fatalf("get stock document: %v", err)
	}
	if detail.WorkOrderNo != "WO-0000000088" || detail.TotalQtyG != 0 || detail.TotalQtyUnits != 7 {
		t.Fatalf("detail = %+v, want work order number and 7 count units", detail.StockDocumentRow)
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
	}); err == nil || !strings.Contains(err.Error(), "库存不足") {
		t.Fatalf("over-return error = %v, want Chinese stock shortage", err)
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
	}); err == nil || !strings.Contains(err.Error(), "质检冻结") {
		t.Fatalf("frozen batch error = %v, want Chinese quality block", err)
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
	}); err == nil || !strings.Contains(err.Error(), "库存不足") {
		t.Fatalf("cross-work-order return error = %v, want Chinese stock shortage", err)
	}
}

func TestProductionReturnCapsQualityBlockedBatchToWorkOrderBalance(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.work_orders(id,work_order_no,status,product_id,spec_g) VALUES(89,'WO-0000000089','running',9,1000);
		INSERT INTO %s.work_order_material_reservations(work_order_id,material_id,reserved_g) VALUES(89,1,6000);
	`, schema, schema))
	receipt, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "g",
			ToWarehouse: "raw_materials", QtyG: 1000, UnitCost: 40,
		}},
	})
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	for workOrderID, quantity := range map[int64]int64{88: 100, 89: 900} {
		if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
			Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: workOrderID, Operator: "jj",
			Items: []stockapp.StockDocumentItemCommand{{
				MaterialID: 1, ItemType: "material", InventoryUnit: "g",
				FromWarehouse: "raw_materials", ToWarehouse: "wip", QtyG: quantity,
			}},
		}); err != nil {
			t.Fatalf("issue work order %d: %v", workOrderID, err)
		}
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.material_batches SET quality_status='hold' WHERE batch_code='%s';
	`, schema, receipt.Items[0].BatchCode))
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, IsReturn: true, WorkOrderID: 88, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "g",
			FromWarehouse: "wip", ToWarehouse: "raw_materials", QtyG: 500,
		}},
	}); err == nil ||
		!strings.Contains(err.Error(), "WIP在制仓库存不足") ||
		!strings.Contains(err.Error(), "另有质检冻结库存100g，解除后仍不足") ||
		strings.Contains(err.Error(), "请先处理质检") {
		t.Fatalf("return error = %v, want work-order-capped blocked stock diagnostic", err)
	}
}

func TestReleasedWorkOrderIssueUsesFrozenSnapshotWithoutReservationAndGuardsQuantity(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.materials SET unit='个' WHERE id=1;
		INSERT INTO %s.work_orders(
			id,work_order_no,status,product_id,spec_g,planned_g,planned_output_g,sales_spec_count,material_snapshot
		) VALUES(
			90,'WO-PR559-0090','released',9,1000,5,5,5,
			'[{"material_id":1,"material_name":"水洗豆","unit":"个","source":"packaging","consume_unit":"unit_per_bag","qty_per_unit":1}]'::jsonb
		);
	`, schema, schema))
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "个", QtyUnits: 10, UnitCost: 1,
		}},
	}); err != nil {
		t.Fatalf("receipt: %v", err)
	}
	draft, err := svc.CreateStockDocumentDraft(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 90, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "个", QtyUnits: 6,
		}},
	})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := svc.SubmitStockDocument(ctx, draft.ID, "jj"); err == nil ||
		!strings.Contains(err.Error(), "当前剩余 WIP 缺口") ||
		!strings.Contains(err.Error(), "本次领用6个") ||
		!strings.Contains(err.Error(), "当前可领5个") {
		t.Fatalf("over-issue error = %v, want Chinese material quantity detail", err)
	}
	valid, err := svc.UpdateStockDocumentDraft(ctx, draft.ID, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 90, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "个", QtyUnits: 5,
		}},
	})
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	submitted, err := svc.SubmitStockDocument(ctx, valid.ID, "jj")
	if err != nil {
		t.Fatalf("submit frozen snapshot material: %v", err)
	}
	if submitted.WorkOrderNo != "WO-PR559-0090" {
		t.Fatalf("work order no = %q", submitted.WorkOrderNo)
	}
}

func TestHistoricalWorkOrderIssueUsesReservationRequirementAndCurrentWIPShortage(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		ALTER TABLE %s.work_order_material_reservations
			ADD COLUMN material_name TEXT NOT NULL DEFAULT '',
			ADD COLUMN unit TEXT NOT NULL DEFAULT 'g',
			ADD COLUMN required_g BIGINT NOT NULL DEFAULT 0,
			ADD COLUMN required_units BIGINT NOT NULL DEFAULT 0;
		UPDATE %s.work_orders SET status='completed' WHERE id=88;
		UPDATE %s.work_order_material_reservations SET status='released' WHERE work_order_id=88;
		INSERT INTO %s.work_orders(
			id,work_order_no,status,product_id,spec_g,planned_g,planned_output_g,sales_spec_count,material_snapshot
		) VALUES(92,'WO-HIST-0092','released',9,1000,5000,5000,5,'[]'::jsonb);
		INSERT INTO %s.work_order_material_reservations(
			work_order_id,material_id,material_name,unit,required_g,reserved_g,status
		) VALUES(92,1,'水洗豆','g',5000,5000,'reserved');
	`, schema, schema, schema, schema, schema))
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "g", QtyG: 6000, UnitCost: 40,
		}},
	}); err != nil {
		t.Fatalf("receipt: %v", err)
	}
	staleDraft, err := svc.CreateStockDocumentDraft(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 92, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "g", QtyG: 4000,
		}},
	})
	if err != nil {
		t.Fatalf("stale draft: %v", err)
	}
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 92, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "g", QtyG: 2000,
		}},
	}); err != nil {
		t.Fatalf("first partial issue: %v", err)
	}
	if _, err := svc.SubmitStockDocument(ctx, staleDraft.ID, "jj"); err == nil ||
		!strings.Contains(err.Error(), "当前剩余 WIP 缺口") ||
		!strings.Contains(err.Error(), "本次领用4000g") ||
		!strings.Contains(err.Error(), "当前可领3000g") ||
		!strings.Contains(err.Error(), "超出1000g") {
		t.Fatalf("stale draft over-issue error = %v, want current remaining WIP shortage detail", err)
	}
	currentDraft, err := svc.UpdateStockDocumentDraft(ctx, staleDraft.ID, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 92, Operator: "jj",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "g", QtyG: 3000,
		}},
	})
	if err != nil {
		t.Fatalf("refresh stale draft: %v", err)
	}
	if _, err := svc.SubmitStockDocument(ctx, currentDraft.ID, "jj"); err != nil {
		t.Fatalf("issue exact remaining shortage: %v", err)
	}
}

func TestWorkOrderStockDocumentRejectsPurposeItemTypeMismatch(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	tests := []struct {
		name    string
		purpose string
		item    stockapp.StockDocumentItemCommand
	}{
		{
			name: "production issue only accepts material", purpose: stockapp.PurposeMaterialTransferForManufacture,
			item: stockapp.StockDocumentItemCommand{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 1},
		},
		{
			name: "production consumption only accepts material", purpose: stockapp.PurposeMaterialConsumption,
			item: stockapp.StockDocumentItemCommand{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 1},
		},
		{
			name: "manufacture only accepts finished product", purpose: stockapp.PurposeManufacture,
			item: stockapp.StockDocumentItemCommand{MaterialID: 1, ItemType: "material", QtyG: 1000},
		},
		{
			name: "work order material issue only accepts material", purpose: stockapp.PurposeMaterialIssue,
			item: stockapp.StockDocumentItemCommand{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft, err := svc.CreateStockDocumentDraft(ctx, stockapp.StockDocumentCommand{
				Purpose: tt.purpose, WorkOrderID: 88, Operator: "jj",
				Items: []stockapp.StockDocumentItemCommand{tt.item},
			})
			if err != nil {
				t.Fatalf("create draft: %v", err)
			}
			if _, err := svc.SubmitStockDocument(ctx, draft.ID, "jj"); err == nil || !strings.Contains(err.Error(), "item type") {
				t.Fatalf("submit mismatch error = %v, want item type rejection", err)
			}
		})
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
