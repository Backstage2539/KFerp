package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev509E2ERawMaterialOrderProductionFlowContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"DEV-509-E2E-FLOW-MAP",
			"DEV-509-BROWSER-AUDIT",
			"DEV-509-BLOCKER-FIXES",
			"DEV-509-DOCS-ACCEPTANCE",
			"API-509-END-TO-END-FLOW",
			"REV-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料",
			"下单",
			"生产",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料入库",
			"生产工单",
			"操作日志",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料",
			"商品 / SKU",
			"生产",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料入库",
			"生产计划",
			"完工入库",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"原料入库",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"下单",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"生产计划",
		},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"完工入库",
		},
		filepath.Join("docs", "acceptance", "2026-06-30-e2e-raw-material-order-production-flow.md"): {
			"PR-509-E2E-RAW-MATERIAL-ORDER-PRODUCTION-FLOW",
			"Browser/API",
			"原料 -> 商品 / SKU -> 下单 -> 生产",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-509 marker %q", rel, want)
			}
		}
	}
}
