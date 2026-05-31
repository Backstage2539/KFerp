package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev378ProductConfigKVOptionsEditorRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-378-PRODUCT-CONFIG-KV-OPTIONS-EDITOR",
		"DEV-378-PRODUCT-CONFIG-KV-OPTIONS-EDITOR",
		"UT-378-PRODUCT-CONFIG-KV-OPTIONS-EDITOR",
		"API-378-PRODUCT-CONFIG-KV-OPTIONS-EDITOR",
		"REV-378-PRODUCT-CONFIG-KV-OPTIONS-EDITOR",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product config KV options editor seed missing %q", want)
		}
	}
}

func TestDev378ProductConfigKVOptionsEditorSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"normalizeSpecialAttrOptions",
			"options_text",
			"valueType === 'select'",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"BOM版本与特殊属性",
			"special_attrs_schema_json",
			"special_attrs_json",
			"保存特殊属性",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product config KV options editor marker %q", rel, want)
			}
		}
	}
}

func TestDev378ProductConfigKVOptionsEditorDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-378-PRODUCT-CONFIG-KV-OPTIONS-EDITOR",
			"PR-389-BOM-GROUP-SPECIAL-ATTRS",
			"BOM 版本",
			"特殊属性",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-378-PRODUCT-CONFIG-KV-OPTIONS-EDITOR",
			"PR-389-BOM-GROUP-SPECIAL-ATTRS",
			"BOM 版本",
			"特殊属性",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-389",
			"BOM 版本",
			"特殊属性",
			"fallback",
		},
		filepath.Join("docs", "acceptance", "2026-05-26-product-config-kv-options-editor.md"): {
			"PR-378",
			"RED",
			"GREEN",
			"浏览器验收",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product config KV options editor doc marker %q", rel, want)
			}
		}
	}
}
