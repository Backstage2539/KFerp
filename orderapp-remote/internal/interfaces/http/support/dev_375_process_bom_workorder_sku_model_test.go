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
		"DEV-375-PROCESS-TEMPLATE-ROUTE",
		"DEV-375-INDUSTRY-FIELD-TEMPLATE",
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
			"/api/produce/job-cards/:id/metrics",
			"ActualInputQty",
			"ActualOutputQty",
		},
		filepath.Join("internal", "interfaces", "http", "manufacturing", "api.go"): {
			"/api/process-templates",
			"/api/industry-field-templates",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"被哪些 BOM 使用",
			"productProductionConfigUsedByBomRows",
			"bomUsageRowKey",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"生产 BOM（制造主档）",
			"前往分组管理",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProcessTemplatesView.vue"): {
			"工艺模板",
			"工艺路线",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "IndustryFieldTemplatesView.vue"): {
			"行业字段模板",
			"字段定义",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"): {
			"实际投入",
			"实际损耗",
			"保存实际",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"): {
			"按 BOM 预览生产需求",
			"多层展开策略",
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
			"BOM 是生产端主档案",
			"工序卡记录实际投入",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
			"产出商品",
			"工单冻结 BOM 版本",
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
