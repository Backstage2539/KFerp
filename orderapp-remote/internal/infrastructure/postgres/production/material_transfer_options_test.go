package production

import (
	"context"
	app "orderapp/internal/application/production"
	stock "orderapp/internal/application/stock"
	"testing"
)

func TestPlanMaterialTransferOptionsRejectInvalidIDs(t *testing.T) {
	r := Repository{}
	for _, args := range [][2]int64{{0, 1}, {1, 0}, {-1, 1}} {
		if _, err := r.PlanMaterialTransferOptions(context.Background(), args[0], args[1], "raw_materials"); err == nil {
			t.Fatalf("expected invalid identifier for %v", args)
		}
	}
}

func TestFilterPlanTransferRowsEnforcesIdentityAndOwner(t *testing.T) {
	source := app.ProductionPlanComponentSource{ComponentType: "product", ComponentID: 7, ComponentBOMSpecID: 4, ComponentBOMVariantID: 5, ComponentSpecG: 454, Options: []app.ProductionPlanComponentSourceOption{{Warehouse: "wip", OwnerCustomerID: 0}, {Warehouse: "finished_goods", OwnerCustomerID: 11}}}
	base := stock.TransferOptionRow{Warehouse: "finished_goods", ItemType: "finished_product", ItemID: 7, BomSpecID: 4, BomVariantID: 5, SpecG: 454, OwnerCustomerID: 11, BatchID: 1, BatchCode: "right"}
	wrongOwner := base
	wrongOwner.OwnerCustomerID = 12
	wrongVariant := base
	wrongVariant.BomVariantID = 6
	wrongWarehouse := base
	wrongWarehouse.Warehouse = "other"
	rows := filterPlanTransferRows(source, []stock.TransferOptionRow{base, wrongOwner, wrongVariant, wrongWarehouse})
	if len(rows) != 1 || rows[0].BatchCode != "right" {
		t.Fatalf("unexpected candidates: %+v", rows)
	}
}
