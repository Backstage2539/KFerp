package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev377ProductConfigKVSKUReferenceLayoutRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-377-PRODUCT-CONFIG-KV-SKU-REFERENCE-LAYOUT",
		"DEV-377-PRODUCT-CONFIG-KV-SKU-REFERENCE-LAYOUT",
		"UT-377-PRODUCT-CONFIG-KV-SKU-REFERENCE-LAYOUT",
		"API-377-PRODUCT-CONFIG-KV-SKU-REFERENCE-LAYOUT",
		"REV-377-PRODUCT-CONFIG-KV-SKU-REFERENCE-LAYOUT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product config KV SKU reference/layout seed missing %q", want)
		}
	}
}

func TestDev377ProductConfigKVSKUReferenceLayoutSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"产品信息字段（特殊属性KV）",
			"展示到价格表/PDF",
			"SKU列表特殊属性列填写具体值",
			"openSpecialAttrConfigForProduct",
			"ensurePublicProductReferenceForCustomer",
			"use_public_sku: true",
			"use_public_categories: true",
			"sku-table-wrap",
			"sku-category-cell",
			"white-space: nowrap",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"enableCustomerPublicProductReference",
			"UsePublicSKU:               true",
			"UsePublicCategories:        true",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product config KV SKU reference/layout marker %q", rel, want)
			}
		}
	}
}

func TestDev377ProductConfigKVSKUReferenceLayoutDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-377-PRODUCT-CONFIG-KV-SKU-REFERENCE-LAYOUT",
			"产品信息字段（特殊属性KV）",
			"客户看到的是引用公共产品",
			"产品类型一个字一行",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-377-PRODUCT-CONFIG-KV-SKU-REFERENCE-LAYOUT",
			"展示到价格表/PDF",
			"引用的公共产品",
			"横向滚动",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-376/377",
			"配置字段",
			"公共产品引用",
			"左右滑动",
		},
		filepath.Join("docs", "acceptance", "2026-05-26-product-config-kv-sku-reference-layout.md"): {
			"PR-377",
			"SKU列表特殊属性列填写具体值",
			"引用公共 SKU",
			"产品类型一个字一行",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product config KV SKU reference/layout doc marker %q", rel, want)
			}
		}
	}
}
