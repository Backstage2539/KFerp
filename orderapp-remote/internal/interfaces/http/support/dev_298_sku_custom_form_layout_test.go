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

	for _, rel := range []string{
		"docs/REQUIREMENTS.md",
		"docs/ACCEPTANCE_TESTS.md",
		"docs/OP_MANUAL_COSTING.md",
		"docs/acceptance/2026-05-21-sku-custom-form-layout.md",
	} {
		doc := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"客户专属 SKU",
			"横向",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing SKU custom form layout marker %q", rel, want)
			}
		}
	}
}
