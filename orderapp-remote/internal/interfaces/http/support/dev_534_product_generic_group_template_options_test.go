package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev534ProductGenericGroupTemplateOptionsContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "business-grouping.js"): {
			"activeUsages.length === 0 ||",
			"assignmentUsage(usage) === normalizedUsage",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"businessGroupRowsForUsage(businessGroups.value, 'product_catalog')",
			"BusinessGroupControls",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "BusinessGroupControls.vue"): {
			"选择分组模板",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue"): {
			"usages: []",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-534-PRODUCT-GENERIC-GROUP-TEMPLATE-OPTIONS",
			"没有旧用途绑定表示通用模板",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-534-PRODUCT-GENERIC-GROUP-TEMPLATE-OPTIONS",
			"选择分组模板",
		},
		filepath.Join("docs", "acceptance", "2026-07-12-product-generic-group-template-options.md"): {
			"PR-534 商品档案通用分组模板候选兼容验收",
			"商品-咖啡豆",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-534 marker %q", rel, want)
			}
		}
	}
}
