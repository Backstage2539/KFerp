package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev407ProductionBomGroupCategoriesVersionEditSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-407-PRODUCTION-BOM-GROUP-CATEGORIES-VERSION-EDIT",
		"DEV-407-BOM-GROUP-CATEGORIES-DATA-API",
		"DEV-407-BOM-VERSION-DRAFT-RECIPE",
		"DEV-407-BOM-VUE-GROUP-CATEGORY-UX",
		"DEV-407-BOM-VUE-VERSION-RECIPE-UX",
		"UT-407-PRODUCTION-BOM-GROUP-CATEGORIES-VERSION-EDIT",
		"API-407-PRODUCTION-BOM-GROUP-CATEGORIES-VERSION-EDIT",
		"REV-407-PRODUCTION-BOM-GROUP-CATEGORIES-VERSION-EDIT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing production BOM group category/version marker %q", want)
		}
	}
}

func TestDev407ProductionBomGroupCategoriesVersionEditSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"groupRowsByBusinessGroupTemplates",
			"businessGroupMoveAssignmentPayload",
			"BusinessGroupInlineWorkspace",
			"collapsedProductionBomGroups",
			"productionBomCategoryMoveActive",
			`@target="handleProductionBomCategoryMoveTarget"`,
			"handleProductionBomGroupPaginationChange",
			"data-bom-settings-drawer",
			"已发布版本只读，复制为新版草稿后编辑",
			"version-recipe-panel",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "schema.go"): {
			"production_bom_group_categories",
			"group_category_id BIGINT NOT NULL DEFAULT 0",
			"repairEmptyInitialPublishedProductionBomVersions",
		},
		filepath.Join("internal", "interfaces", "http", "bom", "bom_api.go"): {
			"/api/production-bom-groups/:id/categories",
			"/api/production-bom-group-categories/:id",
			"version_id",
		},
		filepath.Join("internal", "application", "bom", "service.go"): {
			"ProductionBomGroupCategory",
			"CreateProductionBomGroupCategory",
			"DeleteProductionBomGroupCategory",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing production BOM group category/version marker %q", rel, want)
			}
		}
	}
}
