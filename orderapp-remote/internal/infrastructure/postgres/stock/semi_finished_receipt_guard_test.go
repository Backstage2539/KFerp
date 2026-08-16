package stock

import (
	"context"
	"fmt"
	"strings"
	"testing"

	stockapp "orderapp/internal/application/stock"
)

func TestOrdinaryMaterialReceiptRejectsSemiFinishedAtDraftAndSubmitPostgres(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	svc := stockapp.NewService(repo)
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		ALTER TABLE %[1]s.materials ADD COLUMN is_semi_finished BOOLEAN NOT NULL DEFAULT false;
		UPDATE %[1]s.materials SET is_semi_finished=true WHERE id=1;
	`, schema))

	command := stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeMaterialReceipt, Operator: "warehouse",
		Items: []stockapp.StockDocumentItemCommand{{
			MaterialID: 1, ItemType: "material", InventoryUnit: "g",
			ToWarehouse: "raw_materials", QtyG: 1000, UnitCost: 288,
		}},
	}
	if _, err := svc.CreateStockDocumentDraft(ctx, command); err == nil || !strings.Contains(err.Error(), "半成品只能通过生产入库") {
		t.Fatalf("CreateStockDocumentDraft error=%v, want manufacture-only guard", err)
	}

	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=false WHERE id=1`, schema))
	draft, err := svc.CreateStockDocumentDraft(ctx, command)
	if err != nil {
		t.Fatalf("create ordinary receipt draft: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true WHERE id=1`, schema))
	if _, err := svc.SubmitStockDocument(ctx, draft.ID, "warehouse"); err == nil || !strings.Contains(err.Error(), "半成品只能通过生产入库") {
		t.Fatalf("SubmitStockDocument error=%v, want rechecked manufacture-only guard", err)
	}
	if _, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 1, UnitCode: "g", QtyG: 1000, UnitCost: 288, Operator: "legacy-receipt",
	}); err == nil || !strings.Contains(err.Error(), "半成品只能通过生产入库") {
		t.Fatalf("legacy ReceiveMaterial error=%v, want manufacture-only guard", err)
	}

	var onhandG, batchCount, legacyReceiptCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT m.onhand_g,
		       (SELECT count(*) FROM %[1]s.material_batches WHERE material_id=m.id),
		       (SELECT count(*) FROM %[1]s.material_receipts WHERE material_id=m.id)
		FROM %[1]s.materials m WHERE m.id=1
	`, schema)).Scan(&onhandG, &batchCount, &legacyReceiptCount); err != nil {
		t.Fatal(err)
	}
	if onhandG != 0 || batchCount != 0 || legacyReceiptCount != 0 {
		t.Fatalf("rejected receipts mutated stock onhand/batches/legacy=%d/%d/%d", onhandG, batchCount, legacyReceiptCount)
	}
}
