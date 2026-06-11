package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev473GenericManufacturingAbstractionContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-473-GENERIC-MANUFACTURING-ABSTRACTION",
			"DEV-473-PRODUCE-PLAN-GENERIC-UI",
			"DEV-473-PRODUCTION-PLAN-CREATE-PAYLOAD",
			"DEV-473-WORK-ORDER-GENERIC-MAIN-COLUMNS",
			"DEV-473-DOCS-ACCEPTANCE",
			"REV-473-GENERIC-MANUFACTURING-ABSTRACTION",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"): {
			"工艺路线摘要",
			"buildProductionPlanCreatePayload(filters, keys)",
			"apiSend('/api/production-plans'",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WorkOrdersView.vue"): {
			"BOM/工艺路线",
			"工序摘要",
			"工艺参数",
			"商品生产配置快照",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "produce-plan.js"): {
			"source_type: 'erp_order'",
			"selected: selectedKeys",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-473-GENERIC-MANUFACTURING-ABSTRACTION",
			"生产建议、建议设备、单批投入、锅数不属于一期通用生产计划主流程",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-473-GENERIC-MANUFACTURING-ABSTRACTION",
			"包装盒",
			"童装",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-473-GENERIC-MANUFACTURING-ABSTRACTION",
			"商品 / BOM / 工艺路线 / 工序 / 工位 / 生产计划 / 工单 / 工序卡",
		},
		filepath.Join("docs", "acceptance", "2026-06-12-generic-manufacturing-abstraction.md"): {
			"PR-473-GENERIC-MANUFACTURING-ABSTRACTION",
			"包装盒",
			"童装",
			"不请求 /api/produce/machines",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-473 marker %q", rel, want)
			}
		}
	}

	produceView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue")))
	for _, avoid := range []string{
		"生产建议",
		"推荐机器",
		"每锅数量",
		"锅数",
		"最终投料数",
		"预计成品",
		"/api/produce/machines",
		"roastPlans",
		"machineRows",
	} {
		if strings.Contains(produceView, avoid) {
			t.Fatalf("ProducePlanView.vue should not expose roast scheduling marker %q", avoid)
		}
	}
}
