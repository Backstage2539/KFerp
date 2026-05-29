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

func TestDev292ProductSettingsVueExposesMarginOverrideColumn(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"利润率覆盖",
		"margin_rate_override",
		"saveProductMarginOverride(row)",
		"留空继承分类模板",
		"normalizeMarginRateOverride",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ProductSettingsView.vue missing product margin override marker %q", want)
		}
	}
	for _, forbidden := range []string{
		`<th v-if="!selectedCustomerSkuCustomerID">利润率覆盖</th>`,
		`<td v-if="!selectedCustomerSkuCustomerID">`,
		`:colspan="selectedCustomerSkuCustomerID ? 14 : 15"`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("ProductSettingsView.vue must expose margin override for customer-owned SKU rows, found old public-only gate %q", forbidden)
		}
	}
	for _, want := range []string{
		`:disabled="!canEditSkuRow(row)"`,
		`:colspan="14"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ProductSettingsView.vue missing customer SKU margin override behavior %q", want)
		}
	}
}

func TestDev292ProductMarginOverrideManualsAndAcceptanceUpdated(t *testing.T) {
	files := []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"),
		filepath.Join("docs", "OP_MANUAL_COSTING.md"),
		filepath.Join("docs", "acceptance", "2026-05-18-product-margin-override.md"),
		filepath.Join("docs", "acceptance", "2026-05-22-customer-sku-margin-override.md"),
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(b)
		for _, want := range []string{"产品级利润率覆盖", "产品子类型", "梯度模板"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing manual/acceptance marker %q", file, want)
			}
		}
	}
}
