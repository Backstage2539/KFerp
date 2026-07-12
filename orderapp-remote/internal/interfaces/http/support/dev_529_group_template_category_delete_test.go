package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev529GroupTemplateCategoryDeleteContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-529-GROUP-TEMPLATE-CATEGORY-DELETE",
			"DEV-529-CATEGORY-DELETE-UI",
			"DEV-529-CATEGORY-DELETE-REPOSITORY",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue"): {
			"确认删除大类",
			"确认删除小类",
			"自动归入未分类",
			"分类已删除",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"DELETE FROM %s.business_group_items WHERE id=ANY($1)",
			"SET group_item_id=0",
			"delete_business_group_item",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-529-GROUP-TEMPLATE-CATEGORY-DELETE",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-529-GROUP-TEMPLATE-CATEGORY-DELETE",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-529-GROUP-TEMPLATE-CATEGORY-DELETE",
			"自动归入未分类",
		},
		filepath.Join("docs", "acceptance", "2026-07-12-group-template-category-delete.md"): {
			"PR-529 分组模板分类删除",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-529 marker %q", rel, want)
			}
		}
	}
}

func TestDev529GroupTemplateCategoryUIHasNoActivation(t *testing.T) {
	page := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue")))
	for _, forbidden := range []string{
		"groupTemplateCategoryForm.active",
		"确认停用分类",
		"分类已停用",
		">启用</span>",
	} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("group template categories must use delete instead of activation state, found %q", forbidden)
		}
	}
}
