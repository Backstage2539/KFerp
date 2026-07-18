package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev292ProductMarginOverrideRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"PR-292-PRODUCT-MARGIN-OVERRIDE",
		"PR-314-CUSTOMER-SKU-MARGIN-OVERRIDE",
		"DEV-292-PRODUCT-MARGIN-OVERRIDE-FIELD",
		"DEV-314-CUSTOMER-SKU-MARGIN-OVERRIDE",
		"DEV-292-PRODUCT-MARGIN-OVERRIDE-PRICING",
		"UT-292-PRODUCT-MARGIN-OVERRIDE",
		"UT-314-CUSTOMER-SKU-MARGIN-OVERRIDE",
		"API-292-PRODUCT-MARGIN-OVERRIDE",
		"API-314-CUSTOMER-SKU-MARGIN-OVERRIDE",
		"REV-292-PRODUCT-MARGIN-OVERRIDE",
		"REV-314-CUSTOMER-SKU-MARGIN-OVERRIDE",
		"产品级利润率覆盖",
		"客户自有/客户定制 SKU",
		"覆盖二级分类绑定模板利润率",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("product margin override requirement seed missing %q", want)
		}
	}
}

func TestDev292ProductSettingsVueRetiresMarginOverrideColumn(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"价格摘要",
		"productPriceSummaryLabel",
		"aliasPriceSummaryLabel",
		"暂无价格表价格",
		"buildProductBasicsPayload(row)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ProductSettingsView.vue missing product price-summary remodel marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"利润率覆盖",
		"saveProductMarginOverride(row)",
		"留空继承价格模板",
		"normalizeMarginRateOverride",
		"buildProductBasicsPayload(row, null)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("ProductSettingsView.vue must retire product margin override UI marker %q", forbidden)
		}
	}
}

func TestDev292ProductMarginOverrideManualsAndAcceptanceUpdated(t *testing.T) {
	files := map[string]string{
		filepath.Join("docs", "REQUIREMENTS.md"):                                          "产品级加价率覆盖",
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                                      "产品级加价率覆盖",
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"):                         "产品级利润率覆盖",
		filepath.Join("docs", "OP_MANUAL_COSTING.md"):                                     "产品级利润率覆盖",
		filepath.Join("docs", "acceptance", "2026-05-18-product-margin-override.md"):      "产品级利润率覆盖",
		filepath.Join("docs", "acceptance", "2026-05-22-customer-sku-margin-override.md"): "产品级利润率覆盖",
	}
	for file, currentOrHistoricalMarker := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(b)
		for _, want := range []string{currentOrHistoricalMarker, "产品子类型", "梯度模板"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing manual/acceptance marker %q", file, want)
			}
		}
	}
}
