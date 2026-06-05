package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev347SkuBomJumpFocusFilterRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-347-SKU-BOM-JUMP-FOCUS-FILTER",
		"DEV-347-SKU-BOM-JUMP-FOCUS-FILTER",
		"UT-347-SKU-BOM-JUMP-FOCUS-FILTER",
		"API-347-SKU-BOM-JUMP-FOCUS-FILTER",
		"REV-347-SKU-BOM-JUMP-FOCUS-FILTER",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-345 requirement seed missing %q", want)
		}
	}
}

func TestDev347SkuBomJumpFocusFilterWiring(t *testing.T) {
	checks := []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"),
			markers: []string{
				"buildProductBomURL",
				"production_bom_id",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"),
			markers: []string{
				"kferp:navigate-view",
				"navigateProductBom",
				"production_bom_id",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "lib", "bom.js"),
			markers: []string{
				"filterBomRowsByProductFocus",
				"product_id",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"),
			markers: []string{
				"pendingProductionBomID",
				"production_bom_id",
				"productionBomRows",
				"生产 BOM列表",
			},
		},
	}
	for _, check := range checks {
		src := string(readOrderAppFileForTest(t, check.rel))
		for _, want := range check.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-342 wiring marker %q", check.rel, want)
			}
		}
	}
}

func TestDev347SkuBomJumpFocusFilterDocs(t *testing.T) {
	for _, check := range []struct {
		rel     string
		markers []string
	}{
		{rel: filepath.Join("docs", "REQUIREMENTS.md"), markers: []string{"PR-347-SKU-BOM-JUMP-FOCUS-FILTER", "维护 BOM", "显示全部 BOM"}},
		{rel: filepath.Join("docs", "ACCEPTANCE_TESTS.md"), markers: []string{"PR-347-SKU-BOM-JUMP-FOCUS-FILTER", "维护 BOM", "显示全部 BOM"}},
		{rel: filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"), markers: []string{"PR-417", "被哪些 BOM 使用", "产出商品"}},
		{rel: filepath.Join("docs", "acceptance", "2026-05-23-sku-bom-jump-focus-filter.md"), markers: []string{"PR-347-SKU-BOM-JUMP-FOCUS-FILTER", "维护 BOM", "显示全部 BOM"}},
	} {
		src := string(readOrderAppFileForTest(t, check.rel))
		for _, want := range check.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-345 documentation marker %q", check.rel, want)
			}
		}
	}
}
