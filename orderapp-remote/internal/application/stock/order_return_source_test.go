package stock

import (
	"strings"
	"testing"
)

func TestStockDocumentRejectsPrivateOrderReturnSource(t *testing.T) {
	cmd := StockDocumentCommand{Purpose: PurposeMaterialReceipt, SourceType: "order_resolution_return", Items: []StockDocumentItemCommand{{ItemType: "finished_product", ProductID: 1, BomSpecID: 2, BomVariantID: 3, QtyUnits: 1, ToWarehouse: "finished_goods"}}}
	_, err := normalizeStockDocumentCommand(cmd)
	if err == nil || !strings.Contains(err.Error(), "订单处理") {
		t.Fatalf("private return provenance accepted: %v", err)
	}
}
