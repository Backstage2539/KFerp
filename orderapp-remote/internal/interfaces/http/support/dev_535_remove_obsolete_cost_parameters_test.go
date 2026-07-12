package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev535RemoveObsoleteCostParametersContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-535-REMOVE-OBSOLETE-COST-PARAMETERS", "DEV-535-REMOVE-COST-PARAMETER-UI", "DEV-535-COMPAT-DOCS-DEPLOY",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"价格计算模板 / Pricing Rule", "openPricingRuleTrial()", "resetPricingRuleForm",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "obsolete-cost-parameters.test.js"): {
			"product price management only exposes pricing rules", "obsolete cost parameter Vue components and helpers are deleted",
		},
		filepath.Join("docs", "REQUIREMENTS.md"):                                             {"PR-535-REMOVE-OBSOLETE-COST-PARAMETERS"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                                         {"PR-535-REMOVE-OBSOLETE-COST-PARAMETERS"},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"):                                        {"旧成本参数设置已移除"},
		filepath.Join("docs", "acceptance", "2026-07-12-remove-obsolete-cost-parameters.md"): {"PR-535 删除过时成本参数设置验收"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-535 marker %q", rel, want)
			}
		}
	}

	for rel, forbidden := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {"CostingSettingsPanel", "成本参数设置", "activeProductPriceManagementTab"},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"):         {"快速成本参数设置", "CostingSettingsPanel", "settingsOpen"},
		filepath.Join("frontend-vue-shell", "src", "App.vue"):                          {"CostingSettingsView", "costingSettings:"},
		filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"):                {"costingSettings:"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, value := range forbidden {
			if strings.Contains(src, value) {
				t.Fatalf("%s still contains obsolete cost parameter marker %q", rel, value)
			}
		}
	}

	for _, rel := range []string{
		filepath.Join("frontend-vue-shell", "src", "components", "CostingSettingsPanel.vue"),
		filepath.Join("frontend-vue-shell", "src", "views", "CostingSettingsView.vue"),
		filepath.Join("frontend-vue-shell", "src", "lib", "costing-settings.js"),
	} {
		if _, err := os.Stat(filepath.Join(findAncestorForTest(t, "go.mod"), rel)); !os.IsNotExist(err) {
			t.Fatalf("obsolete cost parameter file still exists: %s", rel)
		}
	}
}
