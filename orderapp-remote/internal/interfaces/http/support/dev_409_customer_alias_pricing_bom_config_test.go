package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev409CustomerAliasPricingBomConfigSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-409-CUSTOMER-ALIAS-PRICING-BOM-CONFIG",
		"DEV-409-CUSTOMER-ALIAS-PRICING-UNIT",
		"DEV-409-PRICE-LIST-ALIAS-OVERRIDE",
		"DEV-409-PRODUCT-BOM-SELECTOR",
		"DEV-409-INDUSTRY-FIELD-LEGACY-SAVE",
		"UT-409-CUSTOMER-ALIAS-PRICING-BOM-CONFIG",
		"API-409-CUSTOMER-ALIAS-PRICING-BOM-CONFIG",
		"REV-409-CUSTOMER-ALIAS-PRICING-BOM-CONFIG",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing customer alias pricing/BOM config marker %q", want)
		}
	}
}

func TestDev409CustomerAliasPricingBomConfigSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"customerProductAliasForm.product_config_template_id",
			"aliasProductConfigTemplateOptions",
			"BOM 使用",
			"bomUsageRelationLabel",
			"ensureProductBomUsage",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"product_config_template_id=$8",
			"gradient_template_id=$9",
			"unit_template_id=$10",
			"return fields, nil",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"NULLIF(alias_config.gradient_template_id,0)",
			"NULLIF(p.customer_product_alias_gradient_template_id,0)",
			"alias_legacy_unit.inventory_unit",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing customer alias pricing/BOM config marker %q", rel, want)
			}
		}
	}
}
