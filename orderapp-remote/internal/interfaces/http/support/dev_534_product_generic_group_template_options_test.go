package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev534ProductGenericGroupTemplateOptionsContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "business-grouping.js"): {
			"businessGroupRowsForUsage",
			"return usages.some((usage) => usage.active !== false && assignmentUsage(usage) === normalizedUsage)",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"businessGroupRowsForUsage(businessGroups.value, 'product_catalog')",
			"BusinessGroupControls",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "BusinessGroupControls.vue"): {
			"选择分组模板",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue"): {
			"功能引用",
			"groupTemplateForm.usage_keys",
			".filter((usage) => groupTemplateForm.usage_keys.includes(usage.key))",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-534-PRODUCT-GENERIC-GROUP-TEMPLATE-OPTIONS",
			"没有旧用途绑定表示通用模板",
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"PR-534 的“无用途绑定即通用候选”口径由本需求替代",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-534-PRODUCT-GENERIC-GROUP-TEMPLATE-OPTIONS",
			"选择分组模板",
			"没有用途的通用模板和仅被其他功能引用的模板不能进入商品档案",
		},
		filepath.Join("docs", "acceptance", "2026-07-12-product-generic-group-template-options.md"): {
			"PR-534 商品档案通用分组模板候选兼容验收",
			"商品-咖啡豆",
		},
		filepath.Join("docs", "acceptance", "2026-08-07-product-multi-group-templates.md"): {
			"只有明确启用引用的模板才进入相应业务页面",
			"未引用通用模板和仅引用其他功能的模板不得出现在商品档案",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-534 marker %q", rel, want)
			}
		}
	}

	for rel, forbidden := range map[string]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "business-grouping.js"):     "activeUsages.length === 0 ||",
		filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue"): "usages: []",
	} {
		src := string(readOrderAppFileForTest(t, rel))
		if strings.Contains(src, forbidden) {
			t.Fatalf("%s retains superseded PR-534 marker %q", rel, forbidden)
		}
	}
}
