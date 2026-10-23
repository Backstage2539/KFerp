package stock

import (
	"context"
	"fmt"
	"testing"

	stockapp "orderapp/internal/application/stock"
)

func TestPickingSnapshotCancelsCustomerFutureBulkWithOwnedLedger(t *testing.T) {
	pool, schema, svc, batch := setupPickingSnapshotTest(t)
	ctx := context.Background()
	const ownerID int64 = 72201
	// Give this isolated receipt and its frozen source binding to a customer.
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
UPDATE %[1]s.materials SET owner_customer_id=%[2]d WHERE id=1;
UPDATE %[1]s.material_batches SET owner_customer_id=%[2]d WHERE material_id=1;
UPDATE %[1]s.work_order_material_reservation_batches SET owner_customer_id=%[2]d WHERE work_order_id=88;
`, schema, ownerID))
	doc, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 88, Operator: "snapshot-owner-test",
		Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 1, InventoryUnit: "g", QtyG: 15570,
			FromWarehouse: "raw_materials", ToWarehouse: "wip", BatchCode: batch, OwnerCustomerID: ownerID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Emulate a future release's 60kg posting with only 15.57kg reserved. The
	// compatibility release still caps new postings, but must undo this safely.
	inflateSnapshotPhysicalMovement(t, pool, schema, doc, 60000)
	if _, err := svc.CancelStockDocument(ctx, doc.ID, "snapshot-owner-test"); err != nil {
		t.Fatalf("cancel customer's future bulk document: %v", err)
	}
	assertPickingSnapshotBalance(t, pool, schema, 120000, 0, 15570, 0)
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT warehouse,owner_customer_id,qty_change_g
FROM %s.stock_ledger_entries WHERE source_doc_type='stock_entry_cancel' AND source_doc_id=$1 ORDER BY warehouse`, schema), doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := map[string]int64{"raw_materials": 60000, "wip": -60000}
	for rows.Next() {
		var warehouse string
		var owner, change int64
		if err := rows.Scan(&warehouse, &owner, &change); err != nil {
			t.Fatal(err)
		}
		quantity, found := want[warehouse]
		if !found || owner != ownerID || change != quantity {
			t.Fatalf("customer cancellation ledger warehouse=%q owner=%d change=%d; want owner=%d change=%d", warehouse, owner, change, ownerID, quantity)
		}
		delete(want, warehouse)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(want) != 0 {
		t.Fatalf("missing customer cancellation ledger rows: %v", want)
	}
}
