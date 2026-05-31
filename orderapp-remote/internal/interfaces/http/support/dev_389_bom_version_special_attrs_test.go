package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev389BomVersionSpecialAttrsRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-389-BOM-GROUP-SPECIAL-ATTRS",
		"DEV-389-BOM-GROUP-CRUD",
		"DEV-389-BOM-VERSION-SPECIAL-ATTRS",
		"DEV-389-MIGRATION-BACKFILL",
		"DEV-389-COST-PRODUCTION-INTEGRATION",
		"DEV-389-VUE-UI",
		"API-389-BOM-GROUP-SPECIAL-ATTRS",
		"UT-389-BOM-GROUP-SPECIAL-ATTRS",
		"REV-389-BOM-GROUP-SPECIAL-ATTRS",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("BOM version special attrs seed missing %q", want)
		}
	}
}

func TestDev389BomVersionSpecialAttrsSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"管理分组",
			"BOM版本与特殊属性",
			"special_attrs_schema_json",
			"special_attrs_json",
			"include_inactive=1",
			"saveProductionBomVersionSpecialAttrs",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"生产 BOM",
			"productionBomLabel(row)",
			"action-cell",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "schema.go"): {
			"special_attrs_schema_json JSONB",
			"special_attrs_json JSONB",
			"backfillProductionBomVersionSpecialAttrs",
			"copyProductionBomForSpecialAttrsConflict",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"bound_bv.special_attrs_json",
			"bound_bv.special_attrs_schema_json",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "work_order.go"): {
			"bound_bv.special_attrs_json->>'roast_level'",
		},
		filepath.Join("internal", "interfaces", "http", "bom", "bom_api.go"): {
			"include_inactive",
			"UpdateProductionBomGroup",
			"SpecialAttrsSchemaJSON",
			"SpecialAttrsJSON",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing BOM version special attrs marker %q", rel, want)
			}
		}
	}
}

func TestDev389BomVersionSpecialAttrsDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-389-BOM-GROUP-SPECIAL-ATTRS",
			"special_attrs_schema_json",
			"BOM 版本",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-389-BOM-GROUP-SPECIAL-ATTRS",
			"BOM 分组",
			"特殊属性",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-389",
			"管理分组",
			"BOM 版本特殊属性",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-389",
			"BOM 版本",
			"fallback",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"PR-389",
			"roast_level",
			"工艺参数",
		},
		filepath.Join("docs", "acceptance", "2026-05-31-bom-version-special-attrs.md"): {
			"PR-389-BOM-GROUP-SPECIAL-ATTRS",
			"RED",
			"GREEN",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing BOM version special attrs doc marker %q", rel, want)
			}
		}
	}
}
