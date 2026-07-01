package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev513MaterialClassificationTemplateContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "MaterialsView.vue"): {
			"data-pr513-material-business-groups",
			"BusinessGroupControls",
			"material_catalog",
			"MATERIAL_OBJECT_KEY = 'material'",
			"/api/business-group-assignments",
			"groupRowsByBusinessGroupTemplate",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"migrateMaterialClassificationsToBusinessGroups",
			"material_catalog_migrated",
			"material_catalog",
			"物料档案归组",
			"legacy_material_classification_group_",
			"legacy_material_classification_category_",
		},
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-513-MATERIAL-CLASSIFICATION-TEMPLATE",
			"DEV-513-MATERIAL-GROUP-TEMPLATE-UI",
			"DEV-513-MATERIAL-CLASSIFICATION-MIGRATION",
			"REV-513-MATERIAL-CLASSIFICATION-TEMPLATE",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-513-MATERIAL-CLASSIFICATION-TEMPLATE",
			"material_catalog",
			"物料档案",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-513-MATERIAL-CLASSIFICATION-TEMPLATE",
			"物料档案",
			"移动到分类",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-513-MATERIAL-CLASSIFICATION-TEMPLATE",
			"设置 → 系统设置 → 分组模板",
			"material_catalog",
		},
		filepath.Join("docs", "acceptance", "2026-07-01-material-classification-template.md"): {
			"PR-513-MATERIAL-CLASSIFICATION-TEMPLATE",
			"物料档案",
			"BusinessGroupControls",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-513 marker %q", rel, want)
			}
		}
	}
}
