package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev410CustomerAliasRenamePriceDisplaySeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-410-CUSTOMER-ALIAS-RENAME-PRICE-DISPLAY",
		"DEV-410-ALIAS-RENAME-UI",
		"DEV-410-PRICE-LIST-RENAME-SOURCE",
		"DEV-410-PRODUCT-CODE-DISPLAY",
		"UT-410-CUSTOMER-ALIAS-RENAME-PRICE-DISPLAY",
		"API-410-CUSTOMER-ALIAS-RENAME-PRICE-DISPLAY",
		"REV-410-CUSTOMER-ALIAS-RENAME-PRICE-DISPLAY",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing customer alias rename/product code marker %q", want)
		}
	}
}

func TestDev410CustomerAliasRenamePriceDisplaySourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"customerAliasEffectiveDisplayName(alias)",
			"productCodeLabel(row)",
			"<span>客户商品名</span>",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"customerAliasEffectiveDisplayName",
			"productCodeLabel",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"COALESCE(NULLIF(pcr.customer_display_name,''), NULLIF(cpa.brand_name,''), NULLIF(cpa.display_name,''), p.name) AS customer_product_display_name",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing customer alias rename/product code marker %q", rel, want)
			}
		}
	}
}
