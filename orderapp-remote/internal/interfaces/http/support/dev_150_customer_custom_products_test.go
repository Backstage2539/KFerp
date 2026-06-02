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
		"PR-151",
		"DEV-151-01",
		"UT-151-01",
		"API-151-01",
		"REV-151-01",
		"客户专属 SKU",
		"原料与公共产品共享",
		"模糊搜索",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer custom product requirement seed missing %q", want)
		}
	}
}

func TestCustomerCustomProductsFrontendWiring(t *testing.T) {
	productSettings := string(readDev150File(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"商品档案",
		"/api/product-settings/skus",
		"buildSkuCreatePayload",
		"批量添加商品档案",
		"saveCustomerAliasBatch",
		"/api/customer-product-aliases/batch",
		"ownerLabel(row)",
	} {
		if !strings.Contains(productSettings, want) {
			t.Fatalf("ProductSettingsView.vue missing %q", want)
		}
	}

	orderEntry := string(readDev150File(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"filterProductsForCustomer",
		"customerOwnedBeanListPublicationIDsByType",
		"customerPublicUsages.value",
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("OrderEntryView.vue missing %q", want)
		}
	}
}

func TestCustomerCustomSkuFormUsesSearchableDropdowns(t *testing.T) {
	productSettings := string(readDev150File(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"selectedCustomerSkuCustomerID",
		"product-editor-drawer",
		`class="sku-create-form product-create-form product-drawer-form"`,
		`@submit.prevent="createSku"`,
		"skuForm.name",
		"skuForm.remark",
		"buildSkuCreatePayload(skuContextCustomerID.value, skuForm.value)",
	} {
		if !strings.Contains(productSettings, want) {
			t.Fatalf("ProductSettingsView.vue missing unified SKU create wiring %q", want)
		}
	}
	for _, forbidden := range []string{
		`v-else class="custom-product-form product-drawer-form"`,
		"customForm.copy_bom",
		"customForm.copy_price_tiers",
		"skuForm.product_type_category_id",
		"skuForm.product_subtype_category_id",
	} {
		if strings.Contains(productSettings, forbidden) {
			t.Fatalf("ProductSettingsView.vue should not expose legacy custom SKU create wiring %q", forbidden)
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
