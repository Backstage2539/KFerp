package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev531SettingsEntryConsolidationContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-531-SETTINGS-ENTRY-CONSOLIDATION", "DEV-531-COMPANY-SEAL-SETTINGS", "DEV-531-COSTING-SETTINGS-IN-PRICE-MANAGEMENT",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CompanyProfileView.vue"): {
			"CompanySealSettingsView", "公司资料与公章",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"价格计算模板 / Pricing Rule", "openPricingRuleTrial()",
		},
		filepath.Join("internal", "interfaces", "http", "support", "audit_page.go"): {
			"商品 / 商品价格管理 / 成本参数设置", "设置 / 公司设置 / 公章设置",
		},
		filepath.Join("docs", "REQUIREMENTS.md"):                                          {"PR-531-SETTINGS-ENTRY-CONSOLIDATION"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                                      {"PR-531-SETTINGS-ENTRY-CONSOLIDATION"},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"):                              {"设置 / 公司设置", "代加工模板设置` 不再作为主菜单入口"},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"):                                     {"过时的成本参数设置已移除"},
		filepath.Join("docs", "acceptance", "2026-07-12-settings-entry-consolidation.md"): {"PR-531 设置入口归并验收"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-531 marker %q", rel, want)
			}
		}
	}
}

func TestDev531StandaloneMenuEntriesAreRemovedButLegacyRoutesRemain(t *testing.T) {
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	primary := strings.Split(menu, "export const hiddenViewTitles")[0]
	for _, forbidden := range []string{"key: 'costingSettings'", "key: 'outsourceSettings'"} {
		if strings.Contains(primary, forbidden) {
			t.Fatalf("primary menu still contains consolidated setting %q", forbidden)
		}
	}
	if strings.Contains(menu, "costingSettings:") {
		t.Fatal("obsolete costingSettings route title still exists")
	}
	for _, legacy := range []string{"outsourceSettings:"} {
		if !strings.Contains(menu, legacy) {
			t.Fatalf("hidden legacy route title missing %q", legacy)
		}
	}
}
