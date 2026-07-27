package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev559ProductionConfigWIPIssueUXContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-559-PRODUCTION-CONFIG-WIP-ISSUE-UX",
			"DEV-559-PRODUCTION-CONFIG-BOM-TAB",
			"DEV-559-WIP-COVERAGE",
			"DEV-559-MULTI-MATERIAL-ISSUE",
			"DEV-559-DOCS-DELIVERY",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductionSettingsView.vue"): {
			"{ key: 'bom', label: '生产 BOM'",
			"{ key: 'workstations', label: '工位设备'",
			"<BomView",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"): {
			"productionConfig",
			"hiddenViewTitles",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"WorkOrderWIPCoverage",
			`json:"inventory_unit"`,
			`json:"required_qty"`,
			`json:"shortage_qty"`,
			`json:"work_order_no,omitempty"`,
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "wip_reservation.go"): {
			"GetWorkOrderWIPCoverage",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "ProductionExecutionHubDrawer.vue"): {
			"WIP库存不足",
			"生产领料",
			"required_qty",
			"available_qty",
			"shortage_qty",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "StockEntriesView.vue"): {
			"工单号：",
			"领用数量",
			"库存单位",
			"isBoundProductionDocument",
			"v-if=\"isReceipt\"",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-559-PRODUCTION-CONFIG-WIP-ISSUE-UX",
			"WorkOrderWIPCoverage",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K101. 生产配置、WIP 提示与工单领料",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-559-PRODUCTION-CONFIG-WIP-ISSUE-UX",
			"红色“WIP库存不足”",
		},
		filepath.Join("docs", "OP_MANUAL_STOCK.md"): {
			"PR-559-PRODUCTION-CONFIG-WIP-ISSUE-UX",
			"一个单据自动带出全部短缺物料",
		},
		filepath.Join("docs", "acceptance", "2026-07-27-production-config-wip-issue-ux.md"): {
			"PR-559 生产配置、WIP 提示与工单领料验收",
		},
	} {
		source := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(source, want) {
				t.Fatalf("%s missing PR-559 marker %q", rel, want)
			}
		}
	}
}
