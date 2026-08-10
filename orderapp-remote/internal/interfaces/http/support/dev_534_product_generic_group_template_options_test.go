package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev534ProductGenericGroupTemplateOptionsContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "business-grouping.js"): {
			"businessGroupFeatureSelectionIDs",
			"businessGroupRowsForFeatureSelection",
			"group_template_ids",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"/api/business-group-feature-selections/product_catalog",
			"businessGroupRowsForFeatureSelection(businessGroups.value, productGroupFeatureSelectionIDs.value)",
			"BusinessGroupWorkspace",
			"selectedProductBusinessGroupCategoryKey",
			"productCategoryMoveActive",
			`@target="handleProductCategoryMoveTarget"`,
			`@configure="openProductGroupTemplateDrawer"`,
			"selectableProductGroupTemplates",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "BusinessGroupWorkspace.vue"): {
			"data-business-group-workspace",
			"emit('target'",
			"configureLabel",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue"): {
			"模板只定义分组结构，不在这里维护业务对象",
			"商品、BOM、仓库在各自页面选择模板后完成归类",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-534-PRODUCT-GENERIC-GROUP-TEMPLATE-OPTIONS",
			"没有旧用途绑定表示通用模板",
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"PR-534 的“无用途绑定即通用候选”口径由本需求替代",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-584-PRODUCT-MULTI-GROUP-TEMPLATES",
			"商品档案可同时多选“商品-咖啡豆”和“商品-挂耳”",
			"商品价格表不提供自己的分组模板选择",
		},
		filepath.Join("docs", "acceptance", "2026-07-12-product-generic-group-template-options.md"): {
			"PR-534 商品档案通用分组模板候选兼容验收",
			"商品-咖啡豆",
		},
		filepath.Join("docs", "acceptance", "2026-08-07-product-multi-group-templates.md"): {
			"只有被商品档案功能选择的 active 模板才进入商品档案",
			"商品价格表不维护自己的模板引用",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-534 marker %q", rel, want)
			}
		}
	}

	for rel, forbiddens := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "business-grouping.js"):     {"activeUsages.length === 0 ||"},
		filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue"): {"功能引用", "groupTemplateForm.usage_keys", "replace_usages"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, forbidden := range forbiddens {
			if strings.Contains(src, forbidden) {
				t.Fatalf("%s retains superseded PR-534 marker %q", rel, forbidden)
			}
		}
	}
}
