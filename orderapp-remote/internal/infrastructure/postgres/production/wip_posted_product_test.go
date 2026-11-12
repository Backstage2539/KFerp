package production

import (
	app "orderapp/internal/application/production"
	"testing"
)

func TestPostedProductCoverageDoesNotDuplicateOrHideOtherBlockers(t *testing.T) {
	for _, reason := range []string{"WIP 不足 20000g", "quality_freeze"} {
		status := app.ProductionWIPStatus{DataComplete: true, Status: "blocked", BlockingReason: reason, Materials: []app.WIPReservationRow{
			{ItemType: "product", ItemID: 8, Unit: "kg", InventoryUnit: "kg", SourceWarehouse: "wip", RequiredG: 10000, ShortageG: 10000},
			{ItemType: "product", ItemID: 8, Unit: "kg", InventoryUnit: "kg", SourceWarehouse: "wip", RequiredG: 10000, ShortageG: 10000},
		}, StartedInputs: []app.TaskMaterialIssued{{ItemType: "product", ItemID: 8, Warehouse: "wip", Unit: "g", IssuedQty: 10000}}}
		applyPostedProductCoverage(&status)
		if status.Materials[0].ShortageG != 0 || status.Materials[1].ShortageG != 10000 || status.ShortageG != 10000 {
			t.Fatalf("duplicate posted input applied twice: %+v", status)
		}
		if reason == "quality_freeze" && status.BlockingReason != reason {
			t.Fatalf("other blocker lost: %+v", status)
		}
	}
}

func TestPostedProductCoverageSharesSamePhysicalStock(t *testing.T) {
	row := app.WIPReservationRow{ItemType: "product", ItemID: 8, Unit: "kg", InventoryUnit: "kg", SourceWarehouse: "wip", RequiredG: 10000, AvailableG: 5000, ShortageG: 5000}
	status := app.ProductionWIPStatus{DataComplete: true, Status: "blocked", BlockingReason: "WIP 不足 10000g", Materials: []app.WIPReservationRow{row, row}, StartedInputs: []app.TaskMaterialIssued{{ItemType: "product", ItemID: 8, Warehouse: "wip", Unit: "g", IssuedQty: 10000}}}
	applyPostedProductCoverage(&status)
	if status.Materials[0].ShortageG != 0 || status.Materials[1].AvailableG != 5000 || status.Materials[1].ShortageG != 5000 || status.ShortageG != 5000 || status.AvailableG != 5000 {
		t.Fatalf("stock counted more than once: %+v", status)
	}
}

func TestPostedProductCoveragePreservesSeparateReservations(t *testing.T) {
	row := app.WIPReservationRow{ItemType: "product", ItemID: 8, Unit: "kg", InventoryUnit: "kg", SourceWarehouse: "wip", Status: "reserved", RequiredG: 10000, AvailableG: 5000, ShortageG: 5000}
	status := app.ProductionWIPStatus{DataComplete: true, Status: "blocked", BlockingReason: "WIP 不足 10000g", Materials: []app.WIPReservationRow{row, row}}
	applyPostedProductCoverage(&status)
	if status.AvailableG != 10000 || status.ShortageG != 10000 {
		t.Fatalf("distinct reservation budget discarded: %+v", status)
	}
}
