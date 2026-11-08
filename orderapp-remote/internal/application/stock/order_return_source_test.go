package stock

import (
	"strings"
	"testing"
)

func TestStockDocumentRejectsPrivateOrderReturnSource(t *testing.T) {
	for _, source := range []string{"order_resolution_return", "order_resolution_ownership_return_out", "order_resolution_company_scrap"} {
		cmd := StockDocumentCommand{Purpose: PurposeMaterialReceipt, SourceType: source, Items: []StockDocumentItemCommand{{ItemType: "finished_product", ProductID: 1, BomSpecID: 2, BomVariantID: 3, QtyUnits: 1, ToWarehouse: "finished_goods"}}}
		_, err := normalizeStockDocumentCommand(cmd)
		if err == nil || !strings.Contains(err.Error(), "订单处理") {
			t.Fatalf("private return provenance accepted: %v", err)
		}
	}

}
