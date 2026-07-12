package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev533ProductionFlowPageContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-533-PRODUCTION-FLOW-PAGE", "DEV-533-PRODUCTION-FLOW-TABS", "DEV-533-NAVIGATION-CONSOLIDATION",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductionFlowView.vue"): {
			"生产流程", "生产计划", "生产工单", "工序卡", "生产质检", "生产验收",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"): {
			"key: 'productionFlow', label: '生产流程'", "productionAcceptance: '生产验收'", "key: 'productionManual', label: '生产手册'",
		},
		filepath.Join("docs", "REQUIREMENTS.md"):                                  {"PR-533-PRODUCTION-FLOW-PAGE"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                              {"PR-533-PRODUCTION-FLOW-PAGE"},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"):                          {"生产流程` 用五个 Tab", "生产手册固定在生产管理菜单最底部"},
		filepath.Join("docs", "acceptance", "2026-07-12-production-flow-page.md"): {"PR-533 生产流程页面归并验收"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-533 marker %q", rel, want)
			}
		}
	}
}
