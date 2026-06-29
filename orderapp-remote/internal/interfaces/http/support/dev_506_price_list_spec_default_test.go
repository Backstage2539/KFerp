package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev506PriceListSpecDefaultContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-506-PRICE-LIST-SPEC-DEFAULT-ROW-ERRORS",
			"DEV-506-FLAT-PRICE-SPEC-ROW-ERRORS",
			"DEV-506-SALES-SPEC-DEFAULT",
			"REV-506-PRICE-LIST-SPEC-DEFAULT-ROW-ERRORS",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"priceListFlatRowDisplayTitle(row)",
			"priceListFlatRowPriceUnitLabel(row)",
			"flat-price-row-error-list",
			"hasPriceListFlatRowError(row)",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"默认规格",
			"setSalesSpecDefault(productUnitTemplateForm, rowIndex)",
			"default-spec-toggle",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"defaultSpecKey",
			"defaultSpec?.sales_unit",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-506-PRICE-LIST-SPEC-DEFAULT-ROW-ERRORS",
			"平铺价格行",
			"默认规格",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-506-PRICE-LIST-SPEC-DEFAULT-ROW-ERRORS",
			"行级错误",
			"默认规格",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-506-PRICE-LIST-SPEC-DEFAULT-ROW-ERRORS",
			"平铺价格行",
			"默认规格",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-506-PRICE-LIST-SPEC-DEFAULT-ROW-ERRORS",
			"默认规格",
			"销售规格模板",
		},
		filepath.Join("docs", "acceptance", "2026-06-29-price-list-spec-default.md"): {
			"PR-506-PRICE-LIST-SPEC-DEFAULT-ROW-ERRORS",
			"熟豆-白巧坚果拼配",
			"227g袋装",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-506 marker %q", rel, want)
			}
		}
	}
}
