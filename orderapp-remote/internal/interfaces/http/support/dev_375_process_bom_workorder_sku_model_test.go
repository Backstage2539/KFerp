package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev375ProcessBomWorkorderSkuModelRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
		"DEV-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
		"UT-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
		"API-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
		"REV-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("process BOM workorder SKU seed missing %q", want)
		}
	}
}

func TestDev375ProcessBomWorkorderSkuModelSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "bom", "bom_api.go"): {
			"ExpectedLossRate",
			"expected_loss_rate",
		},
		filepath.Join("internal", "interfaces", "http", "production", "work_order_api.go"): {
			"/api/produce/job-cards/:id/actuals",
			"ActualInputQty",
			"ActualOutputQty",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"预期损耗率",
			"预期产出率",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"): {
			"实际投入",
			"实际损耗",
			"保存实际",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"): {
			"损耗汇总",
			"预期损耗",
			"operation_summary_json",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing process model marker %q", rel, want)
			}
		}
	}
}

func TestDev375ProcessBomWorkorderSkuModelDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
			"BOM 维护预期损耗率",
			"工序卡记录实际投入",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
			"BOM 可维护预期损耗率",
			"工单展示冻结的预期损耗",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"预期损耗率",
			"预期产出率",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"工序卡",
			"实际损耗",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"理论成本",
			"预期产出率",
		},
		filepath.Join("docs", "acceptance", "2026-05-26-process-bom-workorder-sku-model.md"): {
			"PR-375",
			"浏览器验收",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing process model doc marker %q", rel, want)
			}
		}
	}
}
