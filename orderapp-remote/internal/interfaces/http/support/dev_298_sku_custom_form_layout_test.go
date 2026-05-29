package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev298SkuCustomFormLayoutSeedsAndDocs(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-298-SKU-CUSTOM-FORM-LAYOUT",
		"DEV-298-SKU-CUSTOM-FORM-LAYOUT",
		"REV-298-SKU-CUSTOM-FORM-LAYOUT",
		"客户专属 SKU 创建区必须横向全宽展示",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 298 SKU custom form layout seed missing %q", want)
		}
	}

	docMarkers := map[string][]string{
		"docs/REQUIREMENTS.md":                                  {"客户专属 SKU", "横向"},
		"docs/ACCEPTANCE_TESTS.md":                             {"客户专属 SKU", "横向"},
		"docs/OP_MANUAL_COSTING.md":                            {"客户专属 SKU", "新增SKU"},
		"docs/acceptance/2026-05-21-sku-custom-form-layout.md": {"客户专属 SKU", "横向"},
	}
	for rel, wants := range docMarkers {
		doc := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing SKU custom form layout marker %q", rel, want)
			}
		}
	}
}
