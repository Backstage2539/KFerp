package stock

import "testing"

func TestGenericStockDocumentRejectsPlanTransferOrigin(t *testing.T) {
	_, err := normalizeStockDocumentCommand(StockDocumentCommand{Purpose: PurposeMaterialTransfer, SourceType: "production_plan_material_transfer", SourceID: 42, Items: []StockDocumentItemCommand{{ItemType: "material", MaterialID: 1, FromWarehouse: "raw_materials", ToWarehouse: "wip", QtyG: 1, BatchCode: "B1"}}})
	if err == nil {
		t.Fatal("generic stock API accepted forged production-plan transfer origin")
	}
}
