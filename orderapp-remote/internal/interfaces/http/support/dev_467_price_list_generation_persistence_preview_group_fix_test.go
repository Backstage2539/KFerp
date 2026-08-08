package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev467PriceListGenerationPersistencePreviewGroupFixContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-467-PRICE-LIST-GENERATION-PERSISTENCE-PREVIEW-GROUP-FIX",
			"DEV-467-PRICE-LIST-DRAFT-PERSISTENCE",
			"DEV-467-PRICE-LIST-TIER-TRIAL-APPLY",
			"DEV-467-PRICE-LIST-PRODUCT-GROUP-TEMPLATE",
			"REV-467-PRICE-LIST-GENERATION-PERSISTENCE-PREVIEW-GROUP-FIX",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-price-list-draft.js"): {
			"priceListGenerationDraftKey",
			"savePriceListGenerationDraft",
			"readPriceListGenerationDraft",
			"localStorage",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"restorePriceListGenerationDraftForActiveType",
			"savePriceListGenerationDraftForActiveType",
			"mode === 'pricing_rule' || mode === 'tier_template'",
			"selectedProductCatalogGroupTemplates",
			"businessGroupRowsForFeatureSelection",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"businessGroupRowsForFeatureSelection(businessGroups.value, productGroupFeatureSelectionIDs.value)",
			"apiGet('/api/business-group-feature-selections/product_catalog')",
			"skuGroupPagination: skuGroupPagination.value",
			"if (restoringProductSettingsDraft) return",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-price-list-draft.test.js"): {
			"price list generation draft persists pricing selections by scope and product type key",
			"pricing_rule",
			"tier_template",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "costing-bean-list-version-ui.test.js"): {
			"price list generation persists pricing drafts and applies tier-template trial results",
			"assert.match(flatRowSource, /mode === 'pricing_rule' \\|\\| mode === 'tier_template'/)",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "business-grouping.test.js"): {
			"feature-owned business group selection resolves templates in the selected order without template usages",
			"feature-owned business group selection normalizes GET and PUT contracts",
			"group_template_ids",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-467-PRICE-LIST-GENERATION-PERSISTENCE-PREVIEW-GROUP-FIX",
			"刷新后继续保留",
			"熟豆-红岩拼配",
			"PR-534 的“无用途绑定即通用候选”口径由本需求替代",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-467-PRICE-LIST-GENERATION-PERSISTENCE-PREVIEW-GROUP-FIX",
			"计价方式刷新后不回退",
			"意式拼配豆",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-467-PRICE-LIST-GENERATION-PERSISTENCE-PREVIEW-GROUP-FIX",
			"价格表生成草稿",
			"商品分组模板",
		},
		filepath.Join("docs", "acceptance", "2026-06-10-price-list-generation-persistence-preview-group-fix.md"): {
			"PR-467-PRICE-LIST-GENERATION-PERSISTENCE-PREVIEW-GROUP-FIX",
			"熟豆-红岩拼配",
			"浏览器验收",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-467 marker %q", rel, want)
			}
		}
		if rel == filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue") && strings.Contains(src, "businessGroupRowsForUsage(businessGroups.value, 'product_catalog')") {
			t.Fatalf("%s should resolve product templates from the saved feature selection", rel)
		}
	}
}
