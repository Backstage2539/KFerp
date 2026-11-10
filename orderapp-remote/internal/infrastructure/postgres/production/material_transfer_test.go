package production

import (
	"fmt"
	app "orderapp/internal/application/production"
	stock "orderapp/internal/application/stock"
	"testing"
)

func TestPlanTransferOptionsPreserveIdentityAndOwner(t *testing.T) {
	source := app.ProductionPlanComponentSource{ComponentType: "product", ComponentID: 7, ComponentBOMSpecID: 9, ComponentBOMVariantID: 11, ComponentSpecG: 454, Options: []app.ProductionPlanComponentSourceOption{{Warehouse: "finished_goods", OwnerCustomerID: 3}}}
	row := stock.TransferOptionRow{Warehouse: "finished_goods", ItemType: "finished_product", ItemID: 7, BomSpecID: 9, BomVariantID: 11, SpecG: 454, OwnerCustomerID: 3}
	if !planTransferOptionMatchesSource(source, row) {
		t.Fatal("valid exact candidate excluded")
	}
	for name, mutate := range map[string]func(*stock.TransferOptionRow){"owner": func(r *stock.TransferOptionRow) { r.OwnerCustomerID = 4 }, "variant": func(r *stock.TransferOptionRow) { r.BomVariantID = 12 }, "spec": func(r *stock.TransferOptionRow) { r.BomSpecID = 8 }, "warehouse": func(r *stock.TransferOptionRow) { r.Warehouse = "wip" }} {
		t.Run(name, func(t *testing.T) {
			v := row
			mutate(&v)
			if planTransferOptionMatchesSource(source, v) {
				t.Fatal("incorrect inventory identity included")
			}
		})
	}
}

func TestPlanTransferSuggestionsExcludePendingAndUpstream(t *testing.T) {
	source := app.ProductionPlanComponentSource{ID: 1, RequiredG: 10000, DemandG: 15000, WIPCoveredG: 2000, UpstreamG: 5000, Allocations: []app.ProductionPlanSourceAllocation{{Warehouse: "raw_materials", QtyG: 8000, Batches: []app.ProductionPlanSourceBatch{{BatchCode: "B1", QtyG: 8000}}}}}
	out := planTransferSuggestions([]app.ProductionPlanComponentSource{source}, map[int64][2]int64{1: {3000, 0}})
	if len(out) != 1 || len(out[0].Batches) != 1 || out[0].Batches[0].QtyG != 5000 {
		t.Fatalf("pending/upstream not excluded: %+v", out)
	}
}

func TestPlanTransferPreviewKeepsValidItemsWhenAnotherIsUnavailable(t *testing.T) {
	items := []app.PlanMaterialTransferItem{{SourceID: 1}, {SourceID: 2}}
	accepted, rejected := filterPlanTransferPreviewItems(items, func(in []app.PlanMaterialTransferItem) error {
		for _, x := range in {
			if x.SourceID == 2 {
				return fmt.Errorf("batch frozen")
			}
		}
		return nil
	})
	if len(accepted) != 1 || accepted[0].SourceID != 1 || len(rejected) != 1 || rejected[0].SourceID != 2 {
		t.Fatalf("accepted %+v rejected %+v", accepted, rejected)
	}
}

func TestPlanTransferBatchUsesFrozenCountWithoutConvertingLooseStock(t *testing.T) {
	source := app.ProductionPlanComponentSource{ComponentType: "product", ComponentSpecG: 454, RequiredG: 908, RequiredUnits: 2}
	for _, b := range []app.PlanMaterialTransferBatch{{BatchCode: "B", QtyG: 908, QtyUnits: 2}, {BatchCode: "B", QtyG: 908}, {BatchCode: "B", QtyUnits: 2}} {
		got, equivalent, err := normalizePlanTransferBatch(source, b)
		if err != nil || got.QtyG != 0 || got.QtyUnits != 2 || equivalent != 908 {
			t.Fatalf("input %+v got %+v/%d %v", b, got, equivalent, err)
		}
	}
	for _, b := range []app.PlanMaterialTransferBatch{{BatchCode: "B", QtyG: 500}, {BatchCode: "B", QtyG: 900, QtyUnits: 2}} {
		if _, _, err := normalizePlanTransferBatch(source, b); err == nil {
			t.Fatalf("accepted invalid fractional or inconsistent count: %+v", b)
		}
	}
}
