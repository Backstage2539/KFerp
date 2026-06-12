package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev478ProductionPlanDetailDrawerContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-478-PRODUCTION-PLAN-DOCUMENT-DETAIL",
			"DEV-478-PRODUCTION-PLAN-DETAIL-API",
			"DEV-478-PRODUCTION-PLAN-DETAIL-DRAWER",
			"DEV-478-DOCS-ACCEPTANCE",
			"REV-478-PRODUCTION-PLAN-DOCUMENT-DETAIL",
		},
		filepath.Join("internal", "application", "production", "service.go"): {
			"type ProductionPlanRelatedWorkOrder",
			"MaterialSummary",
			"[]MaterialNeed",
			"RelatedWorkOrders",
			"[]ProductionPlanRelatedWorkOrder",
			"JobCardCount",
			"int64",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "production_plan.go"): {
			"aggregateProductionPlanMaterialSummary",
			"loadProductionPlanRelatedWorkOrdersTx",
			"detail.MaterialSummary",
			"detail.RelatedWorkOrders",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"productionPlanDetailEndpoint",
			"/api/production-plans/${id}",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"production-plan-detail-drawer",
			"openProductionPlanDetail",
			"单据头",
			"计划行",
			"物料需求汇总",
			"工艺路线摘要",
			"工艺参数 / 商品生产配置快照",
			"生成结果",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-478-PRODUCTION-PLAN-DOCUMENT-DETAIL",
			"生产计划单据详情抽屉",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-478-PRODUCTION-PLAN-DOCUMENT-DETAIL",
			"点击计划号或详情打开生产计划单据详情抽屉",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-478-PRODUCTION-PLAN-DOCUMENT-DETAIL",
			"点击计划号或详情打开生产计划单据详情抽屉",
		},
		filepath.Join("docs", "acceptance", "2026-06-12-production-plan-document-detail.md"): {
			"PR-478-PRODUCTION-PLAN-DOCUMENT-DETAIL",
			"生产计划单据详情抽屉",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-478 marker %q", rel, want)
			}
		}
	}
}
