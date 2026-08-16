package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev408ProductCreateConfigDrawerSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-408-PRODUCT-CREATE-CONFIG-DRAWER",
		"DEV-408-PRODUCT-CREATE-OPEN-CONFIG",
		"UT-408-PRODUCT-CREATE-CONFIG-DRAWER",
		"API-408-PRODUCT-CREATE-CONFIG-DRAWER",
		"REV-408-PRODUCT-CREATE-CONFIG-DRAWER",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing product create config drawer marker %q", want)
		}
	}
}

func TestDev408ProductCreateConfigDrawerSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"resolveCreatedProductForConfig",
			"createdProduct.id",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"const result = await apiSend('/api/product-settings/products'",
			"body: buildProductCreatePayload(skuForm.value)",
			"await loadAll()",
			"await openProductProductionConfig(createdProductForConfig)",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product create config drawer marker %q", rel, want)
			}
		}
	}
}
