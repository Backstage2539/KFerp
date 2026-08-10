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
			"BusinessGroupWorkspace",
			"selectedProductionBomCategoryKey",
			"productionBomCategoryMoveActive",
			`@target="handleProductionBomCategoryMoveTarget"`,
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProcessTemplatesView.vue"): {
			"工艺路线",
			"路线工序",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "IndustryFieldTemplatesView.vue"): {
			"行业字段模板",
			"字段定义",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue"): {
			"工序要求",
			"实际分钟",
			"实际损耗",
			"损耗原因",
			"进入工位",
			"执行枢纽",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"): {
			"v-model=\"status\"",
			"workOrderStatusOptions",
			"损耗汇总",
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

	workOrders := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue")))
	for _, unwanted := range []string{
		"按 BOM 预览生产需求",
		"多层展开策略",
		"bom-workbench",
		"apiGet('/api/production-boms?status=all')",
		"预期损耗",
		"整体产出率",
	} {
		if strings.Contains(workOrders, unwanted) {
			t.Fatalf("WorkOrdersView.vue should not keep removed BOM demand preview marker %q", unwanted)
		}
	}

	jobCards := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "JobCardsView.vue")))
	jobCardTemplate := jobCards
	if idx := strings.Index(jobCardTemplate, "<script setup>"); idx >= 0 {
		jobCardTemplate = jobCardTemplate[:idx]
	}
	for _, unwanted := range []string{"<input", "保存实际"} {
		if strings.Contains(jobCardTemplate, unwanted) {
			t.Fatalf("JobCardsView.vue must be a read-only record and omit %q", unwanted)
		}
	}
	for _, unwanted := range []string{"runJobCardAction", "saveActuals"} {
		if strings.Contains(jobCards, unwanted) {
			t.Fatalf("JobCardsView.vue must delegate execution to workstation and omit %q", unwanted)
		}
	}
}

func TestDev375ProcessBomWorkorderSkuModelDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
			"BOM 是生产端主档案",
			"工序卡以只读方式记录实际工时",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL",
			"产出商品",
			"工单冻结 BOM 版本",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"BOM 版本设置中只维护一个 `原料损耗比`",
			"净配方数量 ÷ (1 - 原料损耗率)",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"工序卡",
			"工位视图",
			"实际损耗",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"理论成本",
			"唯一的原料损耗",
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
