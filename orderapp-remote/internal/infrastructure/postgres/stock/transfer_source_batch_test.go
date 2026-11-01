package stock

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	stockapp "orderapp/internal/application/stock"
)

func mustTestTransferBatch(t *testing.T, pool *pgxpool.Pool, schema string, productID, bomSpecID, specG int64, warehouse string) string {
	t.Helper()
	var code string
	err := pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT b.batch_code FROM %[1]s.stock_batches b
LEFT JOIN LATERAL (SELECT warehouse FROM %[1]s.stock_ledger_entries l WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g ORDER BY l.id DESC LIMIT 1) l ON true
WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2 AND b.spec_g=$3 AND (b.remaining_g>0 OR b.remaining_units>0) AND COALESCE(l.warehouse,'finished_goods')=$4 ORDER BY b.created_at,b.id LIMIT 1`, schema), productID, bomSpecID, specG, warehouse).Scan(&code)
	if err != nil {
		t.Fatalf("find transfer batch: %v", err)
	}
	return code
}

func TestFinishedTransferConsumesOnlySelectedSourceBatch(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
VALUES(9,1000,'finished_goods',4,0);
INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,qty_g,qty_units,remaining_g,remaining_units)
VALUES('OLD','finished_product',9,'测试熟豆',1000,2000,2,2000,2),('NEW','finished_product',9,'测试熟豆',1000,2000,2,2000,2);
`, schema, schema))
	svc := stockapp.NewService(NewRepository(pool, schema))
	detail, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Operator: "test", Items: []stockapp.StockDocumentItemCommand{{
		ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 1, BatchCode: "NEW", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var old, newer int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT remaining_units FROM %s.stock_batches WHERE batch_code='OLD'), (SELECT remaining_units FROM %s.stock_batches WHERE batch_code='NEW')`, schema, schema)).Scan(&old, &newer); err != nil {
		t.Fatal(err)
	}
	if old != 2 || newer != 1 {
		t.Fatalf("selected NEW: OLD=%d NEW=%d, want 2 and 1", old, newer)
	}
	if len(detail.Items[0].TransferBatchMoves) != 1 || detail.Items[0].TransferBatchMoves[0].SourceBatchCode != "NEW" || detail.Items[0].TransferBatchMoves[0].TargetBatchCode == "" {
		t.Fatalf("source and target batch identities missing: %+v", detail.Items[0])
	}
	var auditSource, auditTarget string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(meta#>>'{transfer_lines,0,source_batch_code}',''),COALESCE(meta#>>'{transfer_lines,0,target_batch_code}','') FROM %s.audit_logs WHERE entity_type='stock_entry' AND entity_id=$1 AND action='submit'`, schema), detail.ID).Scan(&auditSource, &auditTarget); err != nil {
		t.Fatal(err)
	}
	if auditSource != "NEW" || auditTarget != detail.Items[0].TransferBatchMoves[0].TargetBatchCode {
		t.Fatalf("audit batch identities source=%q target=%q", auditSource, auditTarget)
	}
}

func TestOldTransferDraftWithoutBatchCannotBeSubmitted(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	draft, err := svc.CreateStockDocumentDraft(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{
		MaterialID: 1, ItemType: "material", QtyG: 1, BatchCode: "OLD", FromWarehouse: "raw_materials", ToWarehouse: "wip",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.stock_entry_items SET batch_code='' WHERE stock_entry_id=%d`, schema, draft.ID))
	_, err = svc.SubmitStockDocument(ctx, draft.ID, "test")
	if err == nil || !strings.Contains(err.Error(), "批次") {
		t.Fatalf("expected source batch error, got %v", err)
	}
}

func TestOrdinaryMaterialTransferCannotUseReservedBatchQuantity(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	receipt, err := svc.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{MaterialID: 1, QtyG: 10000, UnitCost: 10})
	if err != nil {
		t.Fatal(err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET reserved_g=5000 WHERE work_order_id=88 AND material_id=1`, schema))
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.work_order_material_reservation_batches (
 reservation_id bigint, work_order_id bigint, material_batch_id bigint, stock_batch_id bigint DEFAULT 0,
 warehouse text, owner_customer_id bigint DEFAULT 0, reserved_g bigint, reserved_units bigint DEFAULT 0,
 consumed_g bigint DEFAULT 0, consumed_units bigint DEFAULT 0, returned_g bigint DEFAULT 0,
 returned_units bigint DEFAULT 0, status text DEFAULT 'reserved');
INSERT INTO %s.work_order_material_reservation_batches(reservation_id,work_order_id,material_batch_id,warehouse,reserved_g)
SELECT id,88,%d,'raw_materials',5000 FROM %s.work_order_material_reservations WHERE work_order_id=88;
`, schema, schema, receipt.BatchID, schema))
	options, err := NewRepository(pool, schema).ListTransferOptions(ctx, stockapp.TransferOptionsQuery{Warehouse: "raw_materials", ItemType: "material", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Rows) != 1 || options.Rows[0].AvailableG != 5000 {
		t.Fatalf("transfer options = %+v, want 5000g available", options.Rows)
	}
	_, wrongUnitErr := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{
		MaterialID: 1, ItemType: "material", QtyUnits: 1, BatchCode: receipt.BatchCode, FromWarehouse: "raw_materials", ToWarehouse: "wip",
	}}})
	if wrongUnitErr == nil || !strings.Contains(wrongUnitErr.Error(), "重量") {
		t.Fatalf("weight material must reject count quantities, got %v", wrongUnitErr)
	}
	_, err = svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{
		MaterialID: 1, ItemType: "material", QtyG: 6000, BatchCode: receipt.BatchCode, FromWarehouse: "raw_materials", ToWarehouse: "wip",
	}}})
	if err == nil {
		t.Fatal("transferring 6000g must fail when 5000g of 10000g is reserved")
	}
	var source int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse='raw_materials'`, schema), receipt.BatchID).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != 10000 {
		t.Fatalf("failed transfer changed source balance: %d", source)
	}
}

func TestTransferRejectsUnboundLegacyReservation(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	receipt, err := svc.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{MaterialID: 1, QtyG: 10000, UnitCost: 10})
	if err != nil {
		t.Fatal(err)
	}
	options, err := svc.ListTransferOptions(ctx, stockapp.TransferOptionsQuery{Warehouse: "raw_materials", ItemType: "material"})
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Rows) != 1 || options.Rows[0].AvailableG != 0 || !strings.Contains(options.Rows[0].UnavailableReason, "预留") {
		t.Fatalf("unbound reservation must not appear free: %+v", options.Rows)
	}
	_, err = svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", BatchCode: receipt.BatchCode, FromWarehouse: "raw_materials", ToWarehouse: "wip", QtyG: 1000}}})
	if err == nil || !strings.Contains(err.Error(), "预留") {
		t.Fatalf("unbound reservation must block transfer: %v", err)
	}
}

func TestOrdinaryFinishedTransferCannotUseReservedBatchQuantity(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(9,1000,'finished_goods',4,0);
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,spec_g,qty_g,qty_units,remaining_g,remaining_units)
VALUES('RESERVED','finished_product',9,1000,4000,4,4000,4);
CREATE TABLE %[1]s.work_order_material_reservation_batches (
 reservation_id bigint, work_order_id bigint, material_batch_id bigint DEFAULT 0, stock_batch_id bigint,
 warehouse text, owner_customer_id bigint DEFAULT 0, reserved_g bigint DEFAULT 0, reserved_units bigint DEFAULT 0,
 consumed_g bigint DEFAULT 0, consumed_units bigint DEFAULT 0, returned_g bigint DEFAULT 0,
 returned_units bigint DEFAULT 0, status text DEFAULT 'reserved');
INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,stock_batch_id,warehouse,reserved_units)
SELECT r.id,88,b.id,'finished_goods',3 FROM %[1]s.work_order_material_reservations r, %[1]s.stock_batches b WHERE r.work_order_id=88 AND b.batch_code='RESERVED';
`, schema))
	svc := stockapp.NewService(NewRepository(pool, schema))
	_, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{
		ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 2, BatchCode: "RESERVED", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
	}}})
	if err == nil {
		t.Fatal("transferring two units must fail when three of four units are reserved")
	}
	var source int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='RESERVED'`, schema)).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != 4 {
		t.Fatalf("failed transfer changed source balance: %d", source)
	}
}

func TestTransferDraftAcceptsHistoricalBatchSpecification(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.production_bom_specs(id bigint primary key, name text, inventory_unit text);
CREATE TABLE %[1]s.production_bom_version_variants(id bigint primary key, bom_spec_id bigint, spec_name_snapshot text, inventory_unit text);
INSERT INTO %[1]s.production_bom_specs VALUES(91,'旧规格','袋');
INSERT INTO %[1]s.production_bom_version_variants VALUES(191,91,'旧规格','袋');
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,bom_spec_id,bom_variant_id,qty_units,remaining_units)
VALUES('HIST','finished_product',9,91,191,2,2);
INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(9,91,191,0,'finished_goods',2,0);
`, schema))
	svc := stockapp.NewService(NewRepository(pool, schema))
	draft, err := svc.CreateStockDocumentDraft(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{
		ProductID: 9, ItemType: "finished_product", BomSpecID: 91, BomVariantID: 191, InventoryUnit: "袋", QtyUnits: 1, BatchCode: "HIST", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
	}}})
	if err != nil {
		t.Fatalf("historical batch identity should be usable without current BOM binding: %v", err)
	}
	if draft.Items[0].BomVariantID != 191 {
		t.Fatalf("batch variant changed: %+v", draft.Items[0])
	}
	submitted, err := svc.SubmitStockDocument(ctx, draft.ID, "test")
	if err != nil {
		t.Fatalf("post historical batch: %v", err)
	}
	if submitted.Items[0].BomVariantID != 191 || len(submitted.Items[0].TransferBatchMoves) != 1 || submitted.Items[0].TransferBatchMoves[0].SourceBatchCode != "HIST" {
		t.Fatalf("historical source identity changed: %+v", submitted.Items[0])
	}
}

func TestFinishedTransferRejectsSummaryBatchMismatch(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(9,1000,'finished_goods',5,0);
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,qty_g,qty_units,remaining_g,remaining_units)
VALUES('MISMATCH','finished_product',9,'测试熟豆',1000,4000,4,4000,4);`, schema))
	svc := stockapp.NewService(NewRepository(pool, schema))
	options, err := svc.ListTransferOptions(ctx, stockapp.TransferOptionsQuery{Warehouse: "finished_goods", ItemType: "finished_product"})
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Rows) != 1 || options.Rows[0].UnavailableReason == "" || options.Rows[0].AvailableUnits != 0 {
		t.Fatalf("mismatched stock should be unavailable: %+v", options.Rows)
	}
	_, err = svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 1, BatchCode: "MISMATCH", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop"}}})
	if err == nil {
		t.Fatal("must reject mismatched batch and summary")
	}
}

func TestTransferOptionsShowUnbatchedFinishedBalanceForReconciliation(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(9,1000,'finished_goods',2,0)`, schema))
	options, err := stockapp.NewService(NewRepository(pool, schema)).ListTransferOptions(ctx, stockapp.TransferOptionsQuery{Warehouse: "finished_goods", ItemType: "finished_product"})
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Rows) != 1 || options.Rows[0].BatchID != 0 || options.Rows[0].QtyUnits != 2 || options.Rows[0].UnavailableReason == "" {
		t.Fatalf("unbatched stock must be visible and unavailable: %+v", options.Rows)
	}
}

func TestTransferDuplicateBatchLinesRollbackOnCombinedShortage(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(9,1000,'finished_goods',4,0);
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,spec_g,qty_g,qty_units,remaining_g,remaining_units)
VALUES('ONE','finished_product',9,1000,4000,4,4000,4);`, schema))
	item := stockapp.StockDocumentItemCommand{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 3, BatchCode: "ONE", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop"}
	_, err := stockapp.NewService(NewRepository(pool, schema)).CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{item, item}})
	if err == nil {
		t.Fatal("two lines exceeding a batch must fail")
	}
	var units, moves int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT remaining_units FROM %[1]s.stock_batches WHERE batch_code='ONE'),(SELECT count(*) FROM %[1]s.stock_entry_finished_batch_moves)`, schema)).Scan(&units, &moves); err != nil {
		t.Fatal(err)
	}
	if units != 4 || moves != 0 {
		t.Fatalf("failed transfer left batch units=%d moves=%d", units, moves)
	}
}

func TestSelectedBatchTransferIsIdempotent(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(9,1000,'finished_goods',4,0);
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,spec_g,qty_g,qty_units,remaining_g,remaining_units)
VALUES('ONCE','finished_product',9,1000,4000,4,4000,4);`, schema))
	cmd := stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, IdempotencyKey: "selected-batch-retry", Items: []stockapp.StockDocumentItemCommand{{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 1, BatchCode: "ONCE", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop"}}}
	svc := stockapp.NewService(NewRepository(pool, schema))
	first, err := svc.CreateAndSubmitStockDocument(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateAndSubmitStockDocument(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	var units, moves int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT remaining_units FROM %[1]s.stock_batches WHERE batch_code='ONCE'),(SELECT count(*) FROM %[1]s.stock_entry_finished_batch_moves)`, schema)).Scan(&units, &moves); err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || units != 3 || moves != 1 {
		t.Fatalf("replayed transfer IDs=%d/%d, remaining=%d moves=%d", first.ID, second.ID, units, moves)
	}
}

func TestConcurrentMaterialTransfersSerializeWithoutOverdraw(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	svc := stockapp.NewService(NewRepository(pool, schema))
	receipt, err := svc.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{MaterialID: 1, QtyG: 10000, UnitCost: 10})
	if err != nil {
		t.Fatal(err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET status='consumed' WHERE material_id=1`, schema))
	cmd := stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 7000, BatchCode: receipt.BatchCode, FromWarehouse: "raw_materials", ToWarehouse: "wip"}}}
	start := make(chan struct{})
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; _, e := svc.CreateAndSubmitStockDocument(ctx, cmd); done <- e }()
	}
	close(start)
	success, failures := 0, 0
	for i := 0; i < 2; i++ {
		if e := <-done; e == nil {
			success++
		} else {
			failures++
			if strings.Contains(e.Error(), "deadlock") || strings.Contains(e.Error(), "deadline") {
				t.Fatalf("transfer lock failure: %v", e)
			}
		}
	}
	var source int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse='raw_materials'`, schema), receipt.BatchID).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if success != 1 || failures != 1 || source != 3000 {
		t.Fatalf("success=%d failure=%d source=%d", success, failures, source)
	}
}

func TestMaterialTransferAuditContainsSelectedSourceBatch(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	receipt, err := svc.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{MaterialID: 1, QtyG: 2000, UnitCost: 10})
	if err != nil {
		t.Fatal(err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET status='consumed' WHERE material_id=1`, schema))
	detail, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 1000, BatchCode: receipt.BatchCode, FromWarehouse: "raw_materials", ToWarehouse: "wip"}}})
	if err != nil {
		t.Fatal(err)
	}
	var source, target string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(meta#>>'{transfer_lines,0,source_batch_code}',''),COALESCE(meta#>>'{transfer_lines,0,target_batch_code}','') FROM %s.audit_logs WHERE entity_type='stock_entry' AND entity_id=$1 AND action='submit'`, schema), detail.ID).Scan(&source, &target); err != nil {
		t.Fatal(err)
	}
	if source != receipt.BatchCode || target != receipt.BatchCode {
		t.Fatalf("audit source=%q target=%q want %q", source, target, receipt.BatchCode)
	}
}

func TestFinishedTransferRechecksUnboundReservationAfterBatchLock(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
ALTER TABLE %[1]s.work_order_material_reservations ADD COLUMN component_type text NOT NULL DEFAULT 'material';
ALTER TABLE %[1]s.work_order_material_reservations ADD COLUMN component_id bigint NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.work_order_material_reservations ADD COLUMN source_warehouse text NOT NULL DEFAULT '';
ALTER TABLE %[1]s.work_order_material_reservations ADD COLUMN source_owner_customer_id bigint NOT NULL DEFAULT 0;
INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(9,1000,'finished_goods',4,0);
INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,spec_g,qty_g,qty_units,remaining_g,remaining_units) VALUES('RACE','finished_product',9,1000,4000,4,4000,4);`, schema))
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.stock_batches WHERE batch_code='RACE' FOR UPDATE`, schema)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, e := stockapp.NewService(NewRepository(pool, schema)).CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransfer, Items: []stockapp.StockDocumentItemCommand{{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 2, BatchCode: "RACE", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop"}}})
		done <- e
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%'||$1||'.stock_batches b%' AND query LIKE '%FOR UPDATE OF b%')`, schema).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("transfer did not wait for batch lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := holder.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservations(work_order_id,material_id,component_type,component_id,source_warehouse,reserved_units,status) VALUES(88,0,'product',9,'finished_goods',3,'reserved')`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil || !strings.Contains(err.Error(), "预留") {
		t.Fatalf("transfer must observe committed reservation: %v", err)
	}
}
