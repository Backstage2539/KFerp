package support

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev589PricingTrialProductSpecsContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table    string
		code     string
		status   string
		assignee string
	}{
		{table: "req_product", code: "PR-589-PRICING-TRIAL-PRODUCT-SPECS", status: "done", assignee: "VA"},
		{table: "req_dev", code: "DEV-589-TRIAL-SPEC-CANDIDATES", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-589-CONCRETE-SKU-TRIAL", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-589-DOCS-DEVELOPMENT-DELIVERY", status: "done", assignee: "Codex"},
		{table: "req_review", code: "REV-589-PRICING-TRIAL-PRODUCT-SPECS", status: "done", assignee: "VA"},
	} {
		requireDev589SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}

	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"pricingRuleTrialProductSpecOptions",
			"pricingRuleTrialDefaultProductSpecID",
			"pricingRuleTrialProductSpecUnit",
			"derivedStatus === '' || derivedStatus === 'active'",
			"seen.has(skuID)",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			`v-model="pricingRuleTrialForm.parent_product_id"`,
			`<span>销售规格</span>`,
			`v-model.number="pricingRuleTrialForm.product_id"`,
			"pricingRuleTrialSalesSpecOptions",
			"pricingRuleTrialProductSpecUnit(product)",
			"pricingRuleTrialRunID++",
			"pricingRuleTrialLoading.value = false",
		},
		filepath.Join("internal", "interfaces", "http", "costing", "costing_api_test.go"): {
			"TestPricingRuleTrialAPIBindsConcreteSalesSpecSKUAndUnit",
			`"product_id":560`,
			`"quote_unit":"454g"`,
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-589-PRICING-TRIAL-PRODUCT-SPECS",
			"同一销售单位的不同 SKU 仍分别显示",
			"沿用现有 `product_id` 和 `quote_unit`",
			"SKU → 父商品 BOM",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-589-PRICING-TRIAL-PRODUCT-SPECS",
			"默认选中 `default_sku_id`",
			"没有有效子规格时与价格表一致回退",
			"Van 于 2026-08-10 确认验收完成",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-589-PRICING-TRIAL-PRODUCT-SPECS",
			"选择主商品及具体销售规格",
			"同单位的不同 SKU 不会合并",
			"商品价格表同一具体规格",
		},
		filepath.Join("docs", "acceptance", "2026-08-10-pricing-trial-product-specs.md"): {
			"PR-589 价格试算具体销售规格验收记录",
			"DEV-589-TRIAL-SPEC-CANDIDATES",
			"DEV-589-CONCRETE-SKU-TRIAL",
			"DEV-589-DOCS-DEVELOPMENT-DELIVERY",
			"Van 人工验收",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-589 marker %q", rel, want)
			}
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	if strings.Contains(view, `v-model="pricingRuleTrialForm.quote_unit"`) {
		t.Fatal("pricing trial must derive quote_unit from the selected concrete SKU instead of exposing a free unit selector")
	}

	orderAppRoot := findAncestorForTest(t, "go.mod")
	repoRoot := filepath.Dir(orderAppRoot)
	for _, rel := range []string{"REQUIREMENTS.md", "ACCEPTANCE_TESTS.md"} {
		src, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), "PR-589-PRICING-TRIAL-PRODUCT-SPECS") {
			t.Fatalf("root %s missing PR-589 contract", rel)
		}
	}
}

func requireDev589SeedRow(t *testing.T, src, table, code, status, assignee string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(assignee) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s and assignee %s", table, code, status, assignee)
	}
}
