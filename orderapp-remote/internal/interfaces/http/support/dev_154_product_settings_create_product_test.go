package support

import (
	"os"
	"strings"
	"testing"
)

func TestProductSettingsCanCreatePublicProducts(t *testing.T) {
	view := string(readDev154File(t, "frontend-vue-shell/src/views/ProductSettingsView.vue"))
	for _, want := range []string{
		"新增SKU",
		"新增公共 SKU",
		"product-editor-drawer",
		`@submit.prevent="createProduct"`,
		"productForm",
		"defaultProductForm",
		`/api/product-settings/products`,
		"公共产品已创建",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("ProductSettingsView.vue missing public product create wiring %q", want)
		}
	}

	routes := string(readDev154File(t, "internal/interfaces/http/catalog/product_routes.go"))
	for _, want := range []string{
		`e.POST("/api/product-settings/products", h.createProductAPI)`,
		"ProductKind",
		"DripBagGrams",
		"DripBoxBagCount",
		"AllowFulfillmentOrder",
		"AllowMallOrder",
	} {
		if !strings.Contains(routes, want) {
			t.Fatalf("product settings routes missing %q", want)
		}
	}

	productsJSON := string(readDev154File(t, "internal/interfaces/http/catalog/products_json.go"))
	for _, want := range []string{
		"product_kind",
		"drip_bag_grams",
		"drip_box_bag_count",
		"sales_units",
	} {
		if !strings.Contains(productsJSON, want) {
			t.Fatalf("products JSON source missing %q", want)
		}
	}

	reqs := string(readDev154File(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-154",
		"DEV-154-01",
		"UT-154-01",
		"API-154-01",
		"REV-154-01",
		"产品设置支持新增公共基础产品",
	} {
		if !strings.Contains(reqs, want) {
			t.Fatalf("requirement seed missing %q", want)
		}
	}
}

func readDev154File(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
