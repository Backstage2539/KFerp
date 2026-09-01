package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev621DirectProductOrderStockUnitContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "sales", "order_stock_batches.go"): {
			"canonicalizeOrderStockItems",
			"orderStockIdentityModeProduct",
			"PRODUCT-FP-",
			"AvailableUnits",
		},
		filepath.Join("internal", "infrastructure", "postgres", "sales", "order_stock_deductions.go"): {
			"orderStockAllocationUsesUnits",
			"deductOrderSourceWarehouseItemsTx",
			"AllocatedUnits",
		},
		filepath.Join("internal", "interfaces", "http", "sales", "order_direct_product_stock_api_test.go"): {
			"TestOrderAPIDirectProductStockUsesInventoryUnitsThroughShipment",
			"PRODUCT-FP-7",
			"no-allocation direct-product",
		},
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-621-DIRECT-PRODUCT-ORDER-STOCK-UNITS",
			"DEV-621-DIRECT-PRODUCT-STOCK-PREVIEW",
			"DEV-621-DIRECT-PRODUCT-SHIPMENT-DEDUCTION",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-621-DIRECT-PRODUCT-ORDER-STOCK-UNITS",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-621-DIRECT-PRODUCT-ORDER-STOCK-UNITS",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"直接商品库存预占与发货（PR-621）",
			"不会把盒、袋误换算为克数",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-621 marker %q", rel, want)
			}
		}
	}
}
