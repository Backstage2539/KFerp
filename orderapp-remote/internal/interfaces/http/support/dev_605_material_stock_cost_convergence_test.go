package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev605MaterialStockCostConvergenceContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, marker := range []string{
		`code: "PR-605-MATERIAL-STOCK-COST-CONVERGENCE"`,
		`code: "DEV-605-MATERIAL-ARCHIVE-READONLY-COST"`,
		`code: "DEV-605-WAREHOUSE-BALANCE-ADJUSTMENT"`,
		`code: "DEV-605-PURCHASE-RECEIPT-POSTING"`,
		`code: "DEV-605-DOCS-DEVELOPMENT-DELIVERY"`,
		`code: "REV-605-MATERIAL-STOCK-COST-CONVERGENCE"`,
	} {
		if !strings.Contains(reqStore, marker) {
			t.Fatalf("req_store.go missing PR-605 marker %q", marker)
		}
	}

	checks := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"): {
			"material-row-checkbox", "<th>行业字段</th>", "industryFieldSummary", "最近采购入库价", "采购入库或盘点调整", "purchase_price is server-owned",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "StockEntriesView.vue"): {
			"/api/stock/material-balances", "来源仓账面库存", "冻结库存", "历史原料入库草稿已停用", "前往采购入库",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "StockAdjustmentsView.vue"): {
			"当前仓库账面库存", "initializeMaterialTargetFromBalance", "selectedMaterialUsesCount", "重量及袋、件、盒等离散物料",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "PurchaseView.vue"): {
			"预计数量", "预计单价", "收货确认", "实际数量", "最终单价", "target_warehouse", "qty_units",
		},
		filepath.Join("internal", "infrastructure", "postgres", "stock", "repository.go"): {
			"ListMaterialBalances", "reduceMaterialWarehouseFIFOForAdjustmentTx", "stock_adjustment_batch_allocations", "recomputeMaterialOnhandTx", "remaining_units",
		},
		filepath.Join("internal", "infrastructure", "postgres", "purchase", "repository.go"): {
			"CreatePurchaseReceiptAtomic", "CreateAndSubmitStockDocumentTx", "purchase_receipt_price", "target_warehouse",
		},
		filepath.Join("internal", "interfaces", "http", "production", "stock_entry_api.go"): {
			"普通原料入库已停用", "历史原料入库草稿不能继续提交", "历史原料入库单据只读",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-605-MATERIAL-STOCK-COST-CONVERGENCE", "DEV-605-WAREHOUSE-BALANCE-ADJUSTMENT", "DEV-605-PURCHASE-RECEIPT-POSTING",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-605-MATERIAL-STOCK-COST-CONVERGENCE", "单仓增减不覆盖其他仓", "任一步故障",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"采购单本身不会改变库存或正式成本", "普通“原料入库”新建入口已停用", "该仓当前账面数量",
		},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"): {
			"PR-605-MATERIAL-STOCK-COST-CONVERGENCE", "来源仓账面/可用/冻结余额", "同一事务完成库存单据",
		},
		filepath.Join("docs", "acceptance", "2026-08-24-material-stock-cost-convergence.md"): {
			"RED（实现前）", "GREEN（实现后）", "development", "main", "production",
		},
	}
	for rel, markers := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, marker := range markers {
			if !strings.Contains(src, marker) {
				t.Fatalf("%s missing PR-605 marker %q", rel, marker)
			}
		}
	}

	entryOptions := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "manufacturing-execution.js")))
	if strings.Contains(entryOptions, "value: 'material_receipt'") {
		t.Fatal("new stock entry options must not expose ordinary material receipt")
	}
}
