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
			"enableCustomerPublicProductReference(ctx, cmd.Actor, cmd.CustomerID)",
		},
		filepath.Join("internal", "application", "catalog", "service_test.go"): {
			"TestDeriveProductCategoryEnablesPublicSKUReference",
			"UsePublicSKU",
			"UsePublicCategories",
			"UsePublicGradientTemplates",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_settings_api_test.go"): {
			"derive category should enable public SKU/category reference",
			"publicUsageSaved",
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
			"复制为客户分类",
			"公共 SKU",
			"不复制公共 SKU 主档",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-380-CUSTOMER-CATEGORY-PUBLIC-SKU-REFERENCE",
			"是否使用公共SKU",
			"只读引用",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-380-CUSTOMER-CATEGORY-PUBLIC-SKU-REFERENCE",
			"公共商品分类引用",
			"公共梯度模板引用开关不随这次操作改变",
		},
		filepath.Join("docs", "acceptance", "2026-05-26-customer-category-public-sku-reference.md"): {
			"PR-380",
			"RED",
			"GREEN",
			"浏览器验收",
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
