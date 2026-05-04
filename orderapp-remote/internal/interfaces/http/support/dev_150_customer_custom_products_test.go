package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerCustomProductsRequirementSeeds(t *testing.T) {
	src := string(readDev150File(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-150",
		"DEV-150-01",
		"DEV-150-02",
		"UT-150-01",
		"API-150-01",
		"REV-150-01",
		"客户专属 SKU",
		"原料与公共产品共享",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer custom product requirement seed missing %q", want)
		}
	}
}

func TestCustomerCustomProductsFrontendWiring(t *testing.T) {
	productSettings := string(readDev150File(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"客户专属 SKU",
		"/api/product-settings/custom-products",
		"customForm.copy_bom",
		"customForm.copy_price_tiers",
		"ownerLabel(row)",
		"customTypeLabel(row.custom_type)",
	} {
		if !strings.Contains(productSettings, want) {
			t.Fatalf("ProductSettingsView.vue missing %q", want)
		}
	}

	orderEntry := string(readDev150File(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"filterProductsForCustomer",
		"filterProductsForCustomer(products.value, form.customer_id)",
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("OrderEntryView.vue missing %q", want)
		}
	}
}

func readDev150File(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
