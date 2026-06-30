package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev508E2ERawMaterialOrderProductionFlowContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"DEV-508-E2E-FLOW-MAP",
			"DEV-508-BROWSER-AUDIT",
			"DEV-508-BLOCKER-FIXES",
			"DEV-508-DOCS-ACCEPTANCE",
			"API-508-END-TO-END-FLOW",
			"REV-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料",
			"下单",
			"生产",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料入库",
			"生产工单",
			"操作日志",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料",
			"商品 / SKU",
			"生产",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料入库",
			"生产计划",
			"完工入库",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料入库",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"下单",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"生产计划",
		},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"完工入库",
		},
		filepath.Join("docs", "acceptance", "2026-06-30-e2e-raw-material-order-production-flow.md"): {
			"PR-508-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"Browser/API",
			"原料 -> 商品 / SKU -> 下单 -> 生产",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-508 marker %q", rel, want)
			}
		}
	}
}
