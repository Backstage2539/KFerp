package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev532ProductionSystemMenuConsolidationContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-532-PRODUCTION-SYSTEM-MENU-CONSOLIDATION", "DEV-532-SYSTEM-SETTINGS-MENU", "DEV-532-PRODUCTION-CONFIG-TABS",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductionSettingsView.vue"): {
			"生产配置", "ProcessTemplatesView", "ManufacturingOperationsView", "ManufacturingWorkstationsView",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"): {
			"key: 'productionConfig', label: '生产配置'", "producePlan: '生产计划'", "productionCosts: '生产成本'",
		},
		filepath.Join("docs", "REQUIREMENTS.md"):                                                  {"PR-532-PRODUCTION-SYSTEM-MENU-CONSOLIDATION"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                                              {"PR-532-PRODUCTION-SYSTEM-MENU-CONSOLIDATION"},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"):                                          {"生产管理 / 生产配置", "生产成本不再作为常规菜单"},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"):                                      {"系统 / 系统设置"},
		filepath.Join("docs", "acceptance", "2026-07-12-production-system-menu-consolidation.md"): {"PR-532 生产与系统菜单归并验收"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-532 marker %q", rel, want)
			}
		}
	}
}
