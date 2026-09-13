package production

import (
	"testing"

	productionapp "orderapp/internal/application/production"
)

func TestConnectedProductionReplanRootsCarriesSharedDemandIntoNewDraft(t *testing.T) {
	items := []productionapp.ProductionPlanItem{
		{ID: 1, OutputType: "product", ProductID: 101, ProductName: "原商品", PlannedOutputG: 4000, OrderNos: "SO-1"},
		{ID: 2, OutputType: "product", ProductID: 102, ProductName: "共用上游关联商品", PlannedOutputG: 36000, OrderNos: "SO-2"},
		{ID: 3, OutputType: "product", ProductID: 103, ProductName: "中间品", PlannedOutputG: 50000},
		{ID: 4, OutputType: "material", OutputMaterialID: 88, OutputName: "物料"},
	}
	ids, needs, demands, totalG := connectedProductionReplanRoots([]int64{1}, []int64{1, 2, 3, 4}, items)
	if len(ids) != 1 || ids[0] != 2 || len(needs) != 1 || len(demands) != 1 || totalG != 36000 {
		t.Fatalf("connected roots ids=%v needs=%+v demands=%+v total=%d", ids, needs, demands, totalG)
	}
}

func TestAlignProductionReplanNeedDestinationsMergesNewDemandIntoOriginalDestination(t *testing.T) {
	needs := []productionapp.StartNeed{
		{ProductID: 1, ParentProductID: 1, ProductName: "商品", SpecG: 227, SalesUnit: "袋", InventoryUnit: "kg", InventoryQtyPerSalesUnit: 0.227, TargetWarehouse: "finished_goods", GapG: 908},
		{ProductID: 1, ParentProductID: 1, ProductName: "商品", SpecG: 227, SalesUnit: "袋", InventoryUnit: "kg", InventoryQtyPerSalesUnit: 0.227, GapG: 8172},
	}
	aligned := alignProductionReplanNeedDestinations(needs)
	groups := groupStartNeedsForRuns(aligned, nil)
	if len(groups) != 1 || groups[0].TargetWarehouse != "finished_goods" || groups[0].NeedG != 9080 {
		t.Fatalf("aligned=%+v groups=%+v", aligned, groups)
	}
}

func TestProductionPlanItemStartNeedKeepsFrozenSalesUnit(t *testing.T) {
	need := productionPlanItemStartNeed(productionapp.ProductionPlanItem{
		ProductID: 1, ParentProductID: 1, ProductName: "商品", SpecG: 227,
		InventoryUnit: "kg", SalesSpecCount: 4, PlannedInventoryQty: 0.908, PlannedOutputG: 908,
		SalesSpecSnapshotJSON: `{"sku_id":1,"parent_product_id":1,"spec_label":"227g","sales_unit":"袋","inventory_unit":"kg","inventory_qty_per_sales_unit":0.227,"conversion_source":"catalog_sku_net_content"}`,
	})
	if need.SalesUnit != "袋" || need.SpecLabel != "227g" {
		t.Fatalf("need=%+v, want frozen sales unit and label", need)
	}
}
