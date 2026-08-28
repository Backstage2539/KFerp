package production

import (
	"testing"

	productionapp "orderapp/internal/application/production"
)

func TestPR616MaterialOutputCountCapacityUsesFrozenInventoryQuantity(t *testing.T) {
	item := productionapp.ProductionPlanItem{
		OutputType:          "material",
		OutputMaterialID:    73,
		OutputName:          "初晓-挂耳包",
		OutputQty:           10,
		OutputUnit:          "袋",
		InventoryUnit:       "袋",
		PlannedInventoryQty: 10,
		ProcessSnapshotJSON: `{"operations":[{"seq":1,"operation":"挂耳包装"}]}`,
	}
	splits := []productionapp.ProductionPlanOperationSplit{{
		OperationSeq:   1,
		Operation:      "挂耳包装",
		BatchSizeUnit:  "袋",
		PlannedQty:     10,
		PlannedMinutes: 2,
	}}

	if err := validateProductionPlanOperationSplitCoverage(item, splits); err != nil {
		t.Fatalf("material count output should use frozen inventory quantity: %v", err)
	}

	splits[0].PlannedQty = 9
	if err := validateProductionPlanOperationSplitCoverage(item, splits); err == nil {
		t.Fatal("short material count capacity should still be rejected")
	}
}
