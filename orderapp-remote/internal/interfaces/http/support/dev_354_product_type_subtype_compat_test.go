package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev354ProductTypeSubtypeCompatRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-354-PRODUCT-TYPE-SUBTYPE-COMPAT",
		"DEV-354-PRODUCT-TYPE-SUBTYPE-COMPAT",
		"UT-354-PRODUCT-TYPE-SUBTYPE-COMPAT",
		"API-354-PRODUCT-TYPE-SUBTYPE-COMPAT",
		"REV-354-PRODUCT-TYPE-SUBTYPE-COMPAT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product type/subtype compatibility seed missing %q", want)
		}
	}
}

func TestDev354ProductTypeSubtypeCompatDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-354-PRODUCT-TYPE-SUBTYPE-COMPAT",
			"产品类型",
			"产品子类型",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-354-PRODUCT-TYPE-SUBTYPE-COMPAT",
			"SKU设置",
			"产品类型",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"产品价格表",
			"产品类型",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"产品类型",
			"产品子类型",
		},
		filepath.Join("docs", "acceptance", "2026-05-24-product-type-subtype-compat.md"): {
			"PR-354-PRODUCT-TYPE-SUBTYPE-COMPAT",
			"product_kind",
			"产品价格表",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product type/subtype marker %q", rel, want)
			}
		}
	}
}

func TestDev354ProductTypeSubtypeCompatWiring(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "domain", "catalog", "product_category_config.go"): {
			"LegacyKindDefaultTypeName",
			"ProductCategoryRoleLabel",
			"产品子类型",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"产品类型",
			"产品子类型",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product type/subtype wiring marker %q", rel, want)
			}
		}
	}
}
