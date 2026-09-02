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
			"product-bom-specs",
			"v-for=\"spec in row.bom_specs\"",
			"未配置 BOM 规格",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"列表只把父商品作为商品行",
			"BOM 规格",
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
