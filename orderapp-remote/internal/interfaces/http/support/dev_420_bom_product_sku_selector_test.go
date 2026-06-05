package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev420BomProductSkuSelectorRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-420-BOM-PRODUCT-SKU-SELECTOR",
		"DEV-420-BOM-PRODUCT-SKU-LABEL",
		"UT-420-BOM-PRODUCT-SKU-SELECTOR",
		"API-420-BOM-PRODUCT-SKU-SELECTOR",
		"REV-420-BOM-PRODUCT-SKU-SELECTOR",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-420 requirement seed missing %q", want)
		}
	}
}

func TestDev420BomProductSkuSelectorDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-420-BOM-PRODUCT-SKU-SELECTOR",
			"BOM 编辑抽屉选择产出商品和商品组件时必须显示商品 SKU 编号",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-420-BOM-PRODUCT-SKU-SELECTOR",
			"SKU-000518 初晓",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-420",
			"SKU-000518 初晓",
		},
		filepath.Join("docs", "acceptance", "2026-06-05-bom-product-sku-selector.md"): {
			"PR-420",
			"node --test src/lib/bom.test.js",
		},
	}
	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-420 documentation marker %q", rel, want)
			}
		}
	}
}
