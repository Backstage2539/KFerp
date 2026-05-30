package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev380CustomerCategoryPublicSKUReferenceRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-380-CUSTOMER-CATEGORY-PUBLIC-SKU-REFERENCE",
		"DEV-380-CUSTOMER-CATEGORY-PUBLIC-SKU-REFERENCE",
		"UT-380-CUSTOMER-CATEGORY-PUBLIC-SKU-REFERENCE",
		"API-380-CUSTOMER-CATEGORY-PUBLIC-SKU-REFERENCE",
		"REV-380-CUSTOMER-CATEGORY-PUBLIC-SKU-REFERENCE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer category public SKU reference seed missing %q", want)
		}
	}
}

func TestDev380CustomerCategoryPublicSKUReferenceSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"func (s *Service) DeriveProductCategory",
			"func (s *Service) CopySKUs",
		},
		filepath.Join("internal", "application", "catalog", "service_test.go"): {
			"TestCopySKUsDedupesSourceIDsAndDelegatesOverwriteResult",
			"TestCopySKUsAllowsSameSourceAndTargetOwner",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_settings_api_test.go"): {
			"TestProductSettingsAPISKUCopyOptionsAndCopy",
			"/api/product-settings/skus/copy",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing customer category public SKU reference marker %q", rel, want)
			}
		}
	}
}

func TestDev380CustomerCategoryPublicSKUReferenceDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-380-CUSTOMER-CATEGORY-PUBLIC-SKU-REFERENCE",
			"历史方案",
			"PR-382",
			"SKU复制",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-380-CUSTOMER-CATEGORY-PUBLIC-SKU-REFERENCE",
			"历史验收项已由 PR-382 取代",
			"SKU复制",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-382-SKU-UNIFIED-CREATE-COPY",
			"SKU复制",
			"新增 X 款、覆盖 Y 款、跳过 Z 款",
		},
		filepath.Join("docs", "acceptance", "2026-05-26-customer-category-public-sku-reference.md"): {
			"PR-380",
			"历史方案",
			"PR-382",
			"SKU复制",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing customer category public SKU reference doc marker %q", rel, want)
			}
		}
	}
}
