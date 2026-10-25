package production

import (
	"math"
	"testing"

	app "orderapp/internal/application/production"
)

func TestDefaultMaterialUseScalesWithActualOverproduction(t *testing.T) {
	uses, err := resolveOperationMaterialUses([]app.OperationMaterialUse{{TotalQty: 4.54, Unit: "kg"}}, nil, 12, 10, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := uses[0].Qty, 5.448; math.Abs(got-want) > 1e-8 {
		t.Fatalf("overproduction must consume frozen BOM quantity for all 12 bags: got %v, want %v", got, want)
	}
}

func TestCompletionGroupIdentityKeepsProductionIdentityAcrossCustomerAndBOMVariants(t *testing.T) {
	base := completionGroupSource{
		cardID: 1, workID: 1, no: "WO-1", seq: 1, operation: "包装", station: "包装台",
		p: app.OperationCompletionPreview{
			ProductName: "小菠萝", OutputSpecG: 454, OutputUnit: "袋", TotalPlannedQty: 10,
			Warehouse: "finished_goods", Execution: app.OperationExecution{ReceiptEnabled: true, ReceiptWarehouse: "finished_goods"},
			Materials: []app.OperationMaterialUse{{Key: "material:127:0:0", Unit: "g", TotalQty: 4540, Warehouse: "wip"}, {Key: "material:100:0:0", Unit: "pic", TotalQty: 10, Warehouse: "wip"}},
		},
		wo: app.WorkOrderRow{ID: 1, OutputProductID: 57, CustomerID: 11, BomVariantID: 48},
	}
	other := base
	other.cardID, other.workID, other.no = 2, 2, "WO-2"
	other.wo.ID, other.wo.CustomerID, other.wo.BomVariantID = 2, 22, 491
	if completionGroupIdentity(base) != completionGroupIdentity(other) {
		t.Fatal("sales customer and BOM variant must not split identical frozen production work")
	}

	other.p.Materials = append([]app.OperationMaterialUse(nil), base.p.Materials...)
	other.p.Materials[0].OwnerCustomerID = 7
	if completionGroupIdentity(base) == completionGroupIdentity(other) {
		t.Fatal("different material owners must remain separate groups")
	}
	other = base
	other.p.Materials = append([]app.OperationMaterialUse(nil), base.p.Materials...)
	other.p.Materials[0].Warehouse = "packaging"
	if completionGroupIdentity(base) == completionGroupIdentity(other) {
		t.Fatal("different frozen source warehouses must remain separate groups")
	}
	other = base
	other.station = "包装台B"
	if completionGroupIdentity(base) == completionGroupIdentity(other) {
		t.Fatal("different workstations must remain separate groups")
	}
}
