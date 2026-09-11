package production

import (
	app "orderapp/internal/application/production"
	"testing"
)

func TestPickingSharedStockBudgetHonoursManualIntentAcrossTasks(t *testing.T) {
	options := []app.ProductionPlanComponentSourceOption{{Warehouse: "raw_b", SortOrder: 20, AvailableG: 5000}, {Warehouse: "raw_a", SortOrder: 10, AvailableG: 10000}, {Warehouse: "wip", SortOrder: 90, AvailableG: 6000}}
	rows := []app.ProductionPlanComponentSource{
		{ComponentType: "material", ComponentID: 30, RequiredG: 11000, AllocationMode: "auto", Options: append([]app.ProductionPlanComponentSourceOption{}, options...)},
		{ComponentType: "material", ComponentID: 30, RequiredG: 10000, AllocationMode: "manual", ManualAllocations: []app.ProductionPlanSourceAllocation{{Warehouse: "raw_a", QtyG: 10000}}, Options: append([]app.ProductionPlanComponentSourceOption{}, options...)},
	}
	calculatePickingSources(rows)
	if rows[0].WIPCoveredG != 6000 || rows[0].TransferG != 5000 || rows[0].ShortageG != 0 || rows[1].TransferG != 10000 {
		t.Fatalf("shared budget or manual precedence failed %+v", rows)
	}
	if rows[0].Allocations[1].Warehouse != "raw_b" {
		t.Fatalf("stock promised twice %+v", rows)
	}
}
func TestPickingWarehousePriorityAndShortageUseEachInventoryUnit(t *testing.T) {
	rows := []app.ProductionPlanComponentSource{{ComponentType: "material", ComponentID: 20, RequiredUnits: 25, Unit: "袋", AllocationMode: "auto", Options: []app.ProductionPlanComponentSourceOption{{Warehouse: "b", SortOrder: 10, AvailableUnits: 10}, {Warehouse: "a", SortOrder: 10, AvailableUnits: 5}, {Warehouse: "wip", SortOrder: 99, AvailableUnits: 6}}}}
	calculatePickingSources(rows)
	s := rows[0]
	if s.WIPCoveredUnits != 6 || s.TransferUnits != 15 || s.ShortageUnits != 4 || s.WIPCoveredG != 0 || s.Allocations[0].Warehouse != "wip" || s.Allocations[1].Warehouse != "a" || s.PreparationStatus != "shortage" {
		t.Fatalf("unit/sort/shortage mismatch %+v", s)
	}
}
