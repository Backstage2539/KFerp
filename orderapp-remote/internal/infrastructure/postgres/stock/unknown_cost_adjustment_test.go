package stock

import (
	"context"
	"fmt"
	stockapp "orderapp/internal/application/stock"
	"strings"
	"testing"
)

func TestMaterialUnknownCostCannotUseOrdinaryDeltaAdjustment(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	receipt, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{MaterialID: 1, QtyG: 1000, UnitCost: 10, Operator: "test"})
	if err != nil {
		t.Fatal(err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batches SET cost_unavailable=true WHERE id=%[2]d;UPDATE %[1]s.stock_batches SET cost_unavailable=true WHERE batch_code='%[3]s'`, schema, receipt.BatchID, receipt.BatchCode))
	_, err = repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{AdjustmentType: "material_cost", ItemType: "material", ItemID: 1, MaterialBatchID: receipt.BatchID, TargetUnitCost: 20, Reason: "调整", Operator: "test"})
	if err == nil || !strings.Contains(err.Error(), "原成本未知") {
		t.Fatalf("unknown cost accepted: %v", err)
	}
	var cost float64
	var unknown bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit_cost::float8,cost_unavailable FROM %s.material_batches WHERE id=$1`, schema), receipt.BatchID).Scan(&cost, &unknown); err != nil || cost != 10 || !unknown {
		t.Fatalf("cost=%v unknown=%v err=%v", cost, unknown, err)
	}
}
func TestManualManufacturePreservesConsumedUnknownCost(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	svc := stockapp.NewService(NewRepository(pool, schema))
	receipt, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialReceipt, Operator: "test", Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 6000, UnitCost: 40}}})
	if err != nil {
		t.Fatal(err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batches SET cost_unavailable=true WHERE batch_code='%[2]s';UPDATE %[1]s.stock_batches SET cost_unavailable=true WHERE batch_code='%[2]s'`, schema, receipt.Items[0].BatchCode))
	_, err = svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: 88, Operator: "test", Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 6000}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: 88, Operator: "test", Items: []stockapp.StockDocumentItemCommand{{MaterialID: 1, ItemType: "material", QtyG: 6000}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeManufacture, WorkOrderID: 88, Operator: "test", Items: []stockapp.StockDocumentItemCommand{{ProductID: 9, ItemType: "finished_product", SpecG: 1000, QtyUnits: 5}}})
	if err != nil {
		t.Fatal(err)
	}
	var unknown bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT cost_unavailable FROM %s.stock_batches WHERE batch_code=$1`, schema), result.Items[0].BatchCode).Scan(&unknown); err != nil || !unknown {
		t.Fatalf("manufactured unknown=%v err=%v", unknown, err)
	}
}
