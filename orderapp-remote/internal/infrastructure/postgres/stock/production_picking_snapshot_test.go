package stock

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	stockapp "orderapp/internal/application/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupPickingSnapshotTest(t *testing.T) (*pgxpool.Pool, string, *stockapp.Service, string) {
	t.Helper()
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
UPDATE %[1]s.work_orders SET target_warehouse='wip';
ALTER TABLE %[1]s.work_orders ADD COLUMN production_plan_id BIGINT NOT NULL DEFAULT 1;
CREATE TABLE %[1]s.production_plans(id BIGINT PRIMARY KEY,picking_version INT NOT NULL);
INSERT INTO %[1]s.production_plans VALUES(1,1);
CREATE TABLE %[1]s.work_order_material_reservation_batches(
 id BIGSERIAL PRIMARY KEY,reservation_id BIGINT NOT NULL,work_order_id BIGINT NOT NULL,
 material_id BIGINT NOT NULL,component_type TEXT NOT NULL DEFAULT 'material',component_id BIGINT NOT NULL,
 component_bom_spec_id BIGINT NOT NULL DEFAULT 0,component_bom_variant_id BIGINT NOT NULL DEFAULT 0,component_spec_g BIGINT NOT NULL DEFAULT 0,
 material_batch_id BIGINT NOT NULL DEFAULT 0,stock_batch_id BIGINT NOT NULL DEFAULT 0,
 owner_customer_id BIGINT NOT NULL DEFAULT 0,batch_code TEXT NOT NULL,warehouse TEXT NOT NULL,
 reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,
 consumed_g BIGINT NOT NULL DEFAULT 0,consumed_units BIGINT NOT NULL DEFAULT 0,
 returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
 status TEXT NOT NULL DEFAULT 'reserved',updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(reservation_id,component_type,component_id,component_bom_spec_id,component_spec_g,material_batch_id,stock_batch_id,warehouse)
);
UPDATE %[1]s.work_order_material_reservations SET reserved_g=15570 WHERE work_order_id=88;
`, schema))
	svc := stockapp.NewService(NewRepository(pool, schema))
	receipt, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "snapshot-test",
		Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 1, InventoryUnit: "g", QtyG: 120000, ToWarehouse: "raw_materials"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch := receipt.Items[0].BatchCode
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_id,material_batch_id,batch_code,warehouse,reserved_g)
SELECT r.id,88,1,1,b.id,b.batch_code,'raw_materials',15570 FROM %[1]s.work_order_material_reservations r CROSS JOIN %[1]s.material_batches b WHERE r.work_order_id=88 AND b.batch_code='%[2]s'`, schema, batch))
	return pool, schema, svc, batch
}

func snapshotIssue(t *testing.T, svc *stockapp.Service, batch string, qty int64) stockapp.StockDocumentDetail {
	t.Helper()
	doc, err := svc.CreateAndSubmitStockDocument(context.Background(), stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 88, Operator: "snapshot-test",
		Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 1, InventoryUnit: "g", QtyG: qty, FromWarehouse: "raw_materials", ToWarehouse: "wip", BatchCode: batch}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestPickingSnapshotRecordsMovesAndCancelsOutOfOrder(t *testing.T) {
	pool, schema, svc, batch := setupPickingSnapshotTest(t)
	ctx := context.Background()
	if err := ensureUnifiedStockDocumentTables(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	doc := snapshotIssue(t, svc, batch, 4000)
	var raw []byte
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT picking_reservation_snapshot FROM %s.stock_entry_items WHERE id=$1`, schema), doc.Items[0].ID).Scan(&raw); err != nil {
		t.Fatalf("submitted picking must persist its reservation movement: %v", err)
	}
	var snap struct {
		Version int `json:"version"`
		Moves   []struct {
			From int64 `json:"from_binding_id"`
			To   int64 `json:"to_binding_id"`
			G    int64 `json:"qty_g"`
		} `json:"moves"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil || snap.Version != 1 || len(snap.Moves) != 1 || snap.Moves[0].G != 4000 || snap.Moves[0].From == snap.Moves[0].To {
		t.Fatalf("unexpected reservation movement %s, err=%v", raw, err)
	}
	bulk := snapshotIssue(t, svc, batch, 60000)
	if _, err := svc.CancelStockDocument(ctx, doc.ID, "snapshot-test"); err != nil {
		t.Fatal(err)
	}
	assertPickingSnapshotBalance(t, pool, schema, 60000, 60000, 4000, 11570)
	if _, err := svc.CancelStockDocument(ctx, bulk.ID, "snapshot-test"); err != nil {
		t.Fatal(err)
	}
	assertPickingSnapshotBalance(t, pool, schema, 120000, 0, 15570, 0)
}

// A compatibility image must be able to cancel documents posted by the next
// image, where physical movement exceeds the reservation movement.
func TestPickingSnapshotCancelsFutureBulkDocumentPrecisely(t *testing.T) {
	for _, freeOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("free_only_%t", freeOnly), func(t *testing.T) {
			pool, schema, svc, batch := setupPickingSnapshotTest(t)
			doc := snapshotIssue(t, svc, batch, 15570)
			ctx := context.Background()
			if freeOnly {
				// The later document transferred free stock after another document
				// had already moved the reservation. Cancelling it must not move
				// that earlier document's WIP reservation back out.
				mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.stock_entry_items SET picking_reservation_snapshot='{"version":1,"moves":[]}' WHERE id=%d`, schema, doc.Items[0].ID))
			}
			inflateSnapshotPhysicalMovement(t, pool, schema, doc, 60000)
			if freeOnly {
				// Preserve 15.57kg in WIP for the other document while returning
				// this document's 60kg free portion.
				mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=qty_g+CASE warehouse WHEN 'wip' THEN 15570 ELSE -15570 END`, schema))
			}
			if _, err := svc.CancelStockDocument(ctx, doc.ID, "snapshot-test"); err != nil {
				t.Fatalf("future bulk cancellation: %v", err)
			}
			if freeOnly {
				assertPickingSnapshotBalance(t, pool, schema, 104430, 15570, 0, 15570)
			} else {
				assertPickingSnapshotBalance(t, pool, schema, 120000, 0, 15570, 0)
			}
		})
	}
}

func TestPickingSnapshotCancellationProtectsOtherReservationsAndConsumption(t *testing.T) {
	for _, consumed := range []bool{false, true} {
		t.Run(fmt.Sprintf("consumed_%t", consumed), func(t *testing.T) {
			pool, schema, svc, batch := setupPickingSnapshotTest(t)
			doc := snapshotIssue(t, svc, batch, 15570)
			inflateSnapshotPhysicalMovement(t, pool, schema, doc, 60000)
			ctx := context.Background()
			if consumed {
				mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET consumed_g=1 WHERE warehouse='wip'`, schema))
			} else {
				mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.work_order_material_reservations(work_order_id,material_id,reserved_g) VALUES(89,1,20000);
INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_id,material_batch_id,batch_code,warehouse,reserved_g)
SELECT r.id,89,1,1,b.id,b.batch_code,'wip',20000 FROM %[1]s.work_order_material_reservations r CROSS JOIN %[1]s.material_batches b WHERE r.work_order_id=89`, schema))
			}
			if _, err := svc.CancelStockDocument(ctx, doc.ID, "snapshot-test"); err == nil {
				t.Fatal("cancellation must preserve consumed quantities and other reservations")
			}
			var status string
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.stock_entries WHERE id=$1`, schema), doc.ID).Scan(&status); err != nil || status != "submitted" {
				t.Fatalf("failed cancellation changed status %q: %v", status, err)
			}
			var wip int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE warehouse='wip'`, schema)).Scan(&wip); err != nil || wip != 60000 {
				t.Fatalf("failed cancellation changed physical stock %d: %v", wip, err)
			}
		})
	}
}

func TestPickingSnapshotLegacyNullCancellation(t *testing.T) {
	pool, schema, svc, batch := setupPickingSnapshotTest(t)
	doc := snapshotIssue(t, svc, batch, 4000)
	mustExecStockSQL(t, context.Background(), pool, fmt.Sprintf(`UPDATE %s.stock_entry_items SET picking_reservation_snapshot=NULL WHERE id=%d`, schema, doc.Items[0].ID))
	if _, err := svc.CancelStockDocument(context.Background(), doc.ID, "snapshot-test"); err != nil {
		t.Fatal(err)
	}
	assertPickingSnapshotBalance(t, pool, schema, 120000, 0, 15570, 0)
}

func inflateSnapshotPhysicalMovement(t *testing.T, pool *pgxpool.Pool, schema string, doc stockapp.StockDocumentDetail, qty int64) {
	t.Helper()
	mustExecStockSQL(t, context.Background(), pool, fmt.Sprintf(`
UPDATE %[1]s.stock_entry_items SET qty_g=%[2]d WHERE id=%[3]d;
UPDATE %[1]s.stock_entry_batch_allocations SET qty_g=%[2]d WHERE stock_entry_item_id=%[3]d;
UPDATE %[1]s.material_batch_locations SET qty_g=qty_g+CASE warehouse WHEN 'wip' THEN %[4]d ELSE -%[4]d END;
`, schema, qty, doc.Items[0].ID, qty-doc.Items[0].QtyG))
}

func assertPickingSnapshotBalance(t *testing.T, pool *pgxpool.Pool, schema string, raw, wip, rawReserved, wipReserved int64) {
	t.Helper()
	var got [4]int64
	err := pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT
COALESCE((SELECT qty_g FROM %[1]s.material_batch_locations WHERE warehouse='raw_materials'),0),
COALESCE((SELECT qty_g FROM %[1]s.material_batch_locations WHERE warehouse='wip'),0),
COALESCE((SELECT SUM(reserved_g) FROM %[1]s.work_order_material_reservation_batches WHERE warehouse='raw_materials' AND work_order_id=88),0),
COALESCE((SELECT SUM(reserved_g) FROM %[1]s.work_order_material_reservation_batches WHERE warehouse='wip' AND work_order_id=88),0)`, schema)).Scan(&got[0], &got[1], &got[2], &got[3])
	if err != nil || got != [4]int64{raw, wip, rawReserved, wipReserved} {
		t.Fatalf("balances=%v want=%v err=%v", got, [4]int64{raw, wip, rawReserved, wipReserved}, err)
	}
}
