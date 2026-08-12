package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev588PricingTrialParentPickerBomAccordionContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-588-PRICING-TRIAL-PARENT-PICKER-BOM-ACCORDION",
			"DEV-588-TRIAL-PARENT-PRODUCTS",
			"DEV-588-TRIAL-PRODUCT-KIND-FILTER",
			"DEV-588-BOM-COMPACT-GROUP-ACTIONS",
			"DEV-588-BOM-SINGLE-EXPANDED-PAGINATION",
			"DEV-588-DOCS-DEVELOPMENT-DELIVERY",
			"REV-588-PRICING-TRIAL-PARENT-PICKER-BOM-ACCORDION",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"pricingRuleTrialMainProductOptions",
			"parent_product_id",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"): {
			"orderProductKindFilterOptions",
			"orderProductFamilyOptions",
		},
		filepath.Join("frontend-vue-shell", "src", "components", "SearchableSelect.vue"): {
			`<slot name="menu-header" />`,
			`<slot name="option"`,
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"pricingRuleTrialMainProductOptions",
			"pricingRuleTrialProductOptions",
			"pricingRuleTrialProductKindFilterOptions",
			"activePricingRuleTrialProductKindFilter",
			"setPricingRuleTrialProductKindFilter",
			`#menu-header`,
			`aria-label="商品分类"`,
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "business-grouping.js"): {
			"businessGroupInlineListState",
			"businessGroupVisibleRows",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"BusinessGroupInlineWorkspace",
			"productionBomDisplayGroups",
			"collapsedProductionBomGroups",
			"productionBomPaginationByGroup",
			"handleProductionBomGroupPaginationChange",
			`#group="{ group }"`,
			"<thead>",
			"data-bom-settings-drawer",
			"PaginationControls",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-588-PRICING-TRIAL-PARENT-PICKER-BOM-ACCORDION",
			"价格试算只显示启用主商品",
			"分页仅统计、切片当前展开项的 BOM 行",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"价格试算商品选择器只显示启用主商品",
			"多模板列表初始只展开第一顶层组",
			"不做浏览器或业务验证",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"价格试算的商品候选只列主商品",
			"全部 / 熟豆 / 挂耳 / 生豆 / 速溶咖啡",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-588 的单展开手风琴",
			"均为历史 UI，现行由 PR-596-INLINE-CATEGORY-LISTS 覆盖",
			"每个展开且含 BOM 的分类重复完整表头并有自己的分页",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-588",
			"分页只计算当前展开组的 BOM 行",
		},
		filepath.Join("docs", "acceptance", "2026-08-09-pricing-trial-parent-picker-bom-accordion.md"): {
			"PR-588",
			"主商品",
			"单展开手风琴",
			"Van 人工验收",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-588 marker %q", rel, want)
			}
		}
	}
}

func TestDev588BomListRemovesVerboseDescription(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, forbidden := range []string{
		"生产 BOM 是生产端主档案；选择分组模板后按大类、小类展示",
		"归类保存到 /api/business-group-assignments",
	} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("production BOM list must remove verbose description %q", forbidden)
		}
	}
}
