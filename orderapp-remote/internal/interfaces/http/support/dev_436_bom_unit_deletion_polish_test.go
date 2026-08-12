package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev436BomUnitDeletionPolishRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-436-BOM-UNIT-DELETION-POLISH",
		"DEV-436-BOM-GROUP-DELETE-AND-MOVE-LAYOUT",
		"DEV-436-BOM-ACTIVE-OUTPUT-PRODUCTS",
		"DEV-436-UNIT-TEMPLATE-DELETE",
		"DEV-436-GLOBAL-UNIT-DICTIONARY-DELETE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing PR-436 marker %q", want)
		}
	}
}

func TestDev436BomUnitDeletionPolishSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"BusinessGroupInlineWorkspace",
			"collapsedProductionBomGroups",
			"productionBomCategoryMoveActive",
			`@target="handleProductionBomCategoryMoveTarget"`,
			"handleProductionBomGroupPaginationChange",
			"data-bom-settings-drawer",
			"groupRowsByBusinessGroupTemplates",
			"businessGroupMoveAssignmentPayload",
			"outputProductOptions = computed(() => products.value.filter(isProductionBomOutputProductCandidate)",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"deleteProductUnitTemplate",
			"deleteGlobalUnitDefinitionFromDrawer",
			"/api/product-settings/unit-templates/${templateID}",
			"/api/product-settings/units/${encodeURIComponent(editingCode)}",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "GlobalUnitDefinitionsView.vue"): {
			"deleteGlobalUnitDefinition",
			"/api/product-settings/units/${encodeURIComponent(editingCode)}",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			"DELETE",
			"/api/product-settings/units/:code",
			"/api/product-settings/unit-templates/:id",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"delete_product_unit_definition",
			"delete_product_unit_template",
			"active=false",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-436 source marker %q", rel, want)
			}
		}
	}
}

func TestDev436BomUnitDeletionPolishDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-436-BOM-UNIT-DELETION-POLISH",
			"生产 BOM",
			"单位模板支持删除",
			"全局单位字典支持删除",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-436-BOM-UNIT-DELETION-POLISH",
			"移动到分类",
			"失效商品不出现在产出商品选择器",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"单位模板",
			"删除",
			"全局单位字典",
		},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"): {
			"全局单位字典",
			"删除",
		},
		filepath.Join("docs", "acceptance", "2026-06-06-bom-unit-deletion-polish.md"): {
			"PR-436-BOM-UNIT-DELETION-POLISH",
			"RED",
			"GREEN",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-436 docs marker %q", rel, want)
			}
		}
	}
}
