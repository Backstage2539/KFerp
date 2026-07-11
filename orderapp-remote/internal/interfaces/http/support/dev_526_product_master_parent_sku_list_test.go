package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev526ProductMasterParentSkuListContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"productArchiveRowsWithSkus",
			"sku_search_text",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"product-spec-skus",
			"v-for=\"sku in row.sku_rows\"",
			"个规格 SKU",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"列表只把父商品作为商品行",
			"X 个规格 SKU",
		},
	}
	for path, markers := range checks {
		body := readOrderAppFileForTest(t, path)
		for _, marker := range markers {
			if !strings.Contains(string(body), marker) {
				t.Fatalf("%s missing PR-526 marker %q", path, marker)
			}
		}
	}
}
